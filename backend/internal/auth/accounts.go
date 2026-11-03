package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/password"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Errors for account management. Handlers map them to HTTP codes.
var (
	ErrForbidden      = errors.New("auth: not allowed")
	ErrNotFound       = errors.New("auth: not found")
	ErrInvalidToken   = errors.New("auth: link is invalid or has expired")
	ErrEmailInUse     = errors.New("auth: an account with this email already exists")
	ErrLastSuperAdmin = errors.New("auth: the last Super Admin cannot be removed, demoted or disabled")
	ErrNotGrantable   = errors.New("auth: this role cannot be granted through the API")
	ErrMixedKinds     = errors.New("auth: a user cannot hold staff, client and contributor roles together")
	ErrInvalidInput   = errors.New("auth: invalid input")
	ErrSelf           = errors.New("auth: you cannot change your own access this way")
)

const (
	verifyTTL = 24 * time.Hour
	resetTTL  = time.Hour
	inviteTTL = 7 * 24 * time.Hour
)

func (s *Service) link(path, token string) string {
	return s.opt.AppOrigin + path + "?token=" + url.QueryEscape(token)
}

// RequestPasswordReset emails a reset link when the address belongs to an
// active account. The caller always answers the same way.
func (s *Service) RequestPasswordReset(ctx context.Context, email string, m Meta) error {
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserForLogin(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (u.Status != "active" || u.PasswordHash == nil)) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := q.InvalidatePasswordResets(ctx, u.ID); err != nil {
			return err
		}
		token, hash := newToken()
		if err := q.CreatePasswordReset(ctx, store.CreatePasswordResetParams{UserID: u.ID, TokenHash: hash, ExpiresAt: time.Now().Add(resetTTL)}); err != nil {
			return err
		}
		if err := mail.Enqueue(ctx, q, u.Email, mail.TemplatePasswordReset, map[string]string{
			"Name": u.DisplayName, "Link": s.link("/reset-password", token),
		}); err != nil {
			return err
		}
		return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "auth.password_reset_requested", EntityType: "user", EntityID: &u.ID})
	})
}

// ResetPassword sets a new password from a reset link, signs the user out
// everywhere and clears any lockout. The token stays usable if the new
// password is rejected.
func (s *Service) ResetPassword(ctx context.Context, token, plain string, m Meta) error {
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		userID, err := q.UsePasswordReset(ctx, hashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		u, err := q.GetUserByID(ctx, userID)
		if err != nil {
			return err
		}
		if err := password.Validate(plain, u.Email); err != nil {
			return err // rolls back, so the token is not spent
		}
		hash, err := password.Hash(plain)
		if err != nil {
			return err
		}
		for _, step := range []func() error{
			func() error { return q.SetPassword(ctx, store.SetPasswordParams{ID: userID, PasswordHash: &hash}) },
			func() error { return q.RevokeUserSessions(ctx, userID) },
			func() error { return q.InvalidatePasswordResets(ctx, userID) },
			// The link reached this inbox, which proves the address.
			func() error { return q.MarkEmailVerified(ctx, userID) },
		} {
			if err := step(); err != nil {
				return err
			}
		}
		return Audit(ctx, q, m, Entry{Actor: &userID, Action: "auth.password_reset", EntityType: "user", EntityID: &userID})
	})
}

// SendVerification emails a verification link to an unverified active user.
// Contributor sign-up (Stage 7) calls it; the resend endpoint does too.
func (s *Service) SendVerification(ctx context.Context, email string, m Meta) error {
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserForLogin(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (u.EmailVerifiedAt != nil || u.Status != "active")) {
			return nil
		}
		if err != nil {
			return err
		}
		token, hash := newToken()
		if err := q.CreateEmailVerification(ctx, store.CreateEmailVerificationParams{UserID: u.ID, TokenHash: hash, ExpiresAt: time.Now().Add(verifyTTL)}); err != nil {
			return err
		}
		return mail.Enqueue(ctx, q, u.Email, mail.TemplateVerifyEmail, map[string]string{
			"Name": u.DisplayName, "Link": s.link("/verify-email", token),
		})
	})
}

// VerifyEmail spends a verification token.
func (s *Service) VerifyEmail(ctx context.Context, token string, m Meta) error {
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		userID, err := q.UseEmailVerification(ctx, hashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		if err := q.MarkEmailVerified(ctx, userID); err != nil {
			return err
		}
		return Audit(ctx, q, m, Entry{Actor: &userID, Action: "auth.email_verified", EntityType: "user", EntityID: &userID})
	})
}

// Invitation is a pending invitation as shown to staff or a client admin.
type Invitation struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	OrgID     *uuid.UUID `json:"org_id,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// Invite invites a staff or client user. Staff with user.manage may invite
// to any staff role or any organisation; a Client Admin (org.user.invite)
// only to client roles in their own organisation. Contributors sign up
// themselves (Stage 7). Roles holding system.admin are never invitable.
func (s *Service) Invite(ctx context.Context, p *Principal, email, roleKey string, orgID *uuid.UUID, m Meta) (Invitation, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 254 {
		return Invitation{}, ErrInvalidInput
	}
	var out Invitation
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		role, err := q.GetRoleWithGrantability(ctx, roleKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidInput
		}
		if err != nil {
			return err
		}
		if !role.Grantable || role.Key == "contributor" {
			return ErrNotGrantable
		}
		if err := mayManageRole(p, role.Scope, orgID); err != nil {
			return err
		}
		if _, err := q.GetUserByEmail(ctx, email); err == nil {
			return ErrEmailInUse
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := q.RevokePendingInvitations(ctx, email); err != nil {
			return err
		}
		token, hash := newToken()
		inv, err := q.CreateInvitation(ctx, store.CreateInvitationParams{
			Email: email, RoleID: role.ID, OrgID: orgID, TokenHash: hash, InvitedBy: p.UserID, ExpiresAt: time.Now().Add(inviteTTL),
		})
		if err != nil {
			return foreignKeyAsInvalid(err)
		}
		orgName := ""
		if orgID != nil {
			orgs, err := q.ListOrganisations(ctx)
			if err != nil {
				return err
			}
			for _, o := range orgs {
				if o.ID == *orgID {
					orgName = o.Name
				}
			}
		}
		if err := mail.Enqueue(ctx, q, email, mail.TemplateInvitation, map[string]string{
			"InvitedBy": p.DisplayName, "Role": role.Name, "Org": orgName, "Link": s.link("/accept-invitation", token),
		}); err != nil {
			return err
		}
		out = Invitation{ID: inv.ID, Email: inv.Email, Role: role.Key, OrgID: inv.OrgID, ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "invitation.created", EntityType: "invitation", EntityID: &inv.ID, OrgID: orgID,
			After: map[string]any{"role": role.Key}})
	})
	return out, err
}

// mayManageRole applies the who-may-grant rules shared by invitations and
// role grants.
func mayManageRole(p *Principal, roleScope string, orgID *uuid.UUID) error {
	switch roleScope {
	case "platform":
		if orgID != nil {
			return ErrInvalidInput
		}
		if !p.Has("user.manage") {
			return ErrForbidden
		}
	case "org":
		if orgID == nil {
			return ErrInvalidInput
		}
		if p.Has("user.manage") {
			return nil
		}
		if !p.Has("org.user.invite") || p.Kind != KindClient || *orgID != p.OrgID {
			return ErrForbidden
		}
	default:
		return ErrInvalidInput
	}
	return nil
}

// ListInvitations returns pending invitations: all for staff, own
// organisation for a Client Admin.
func (s *Service) ListInvitations(ctx context.Context, p *Principal) ([]Invitation, error) {
	var filter *uuid.UUID
	if !p.Has("user.manage") {
		org := p.OrgID
		filter = &org
	}
	var out []Invitation
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListPendingInvitations(ctx, filter)
		for _, r := range rows {
			out = append(out, Invitation{ID: r.ID, Email: r.Email, Role: r.RoleKey, OrgID: r.OrgID, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt})
		}
		return err
	})
	if out == nil {
		out = []Invitation{}
	}
	return out, err
}

// RevokeInvitation cancels a pending invitation the caller could have made.
func (s *Service) RevokeInvitation(ctx context.Context, p *Principal, id uuid.UUID, m Meta) error {
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		inv, err := q.GetInvitation(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		scope := "platform"
		if inv.OrgID != nil {
			scope = "org"
		}
		if err := mayManageRole(p, scope, inv.OrgID); err != nil {
			return ErrNotFound // do not reveal invitations outside the caller's reach
		}
		if inv.AcceptedAt != nil || inv.RevokedAt != nil {
			return ErrNotFound
		}
		if err := q.RevokeInvitation(ctx, id); err != nil {
			return err
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "invitation.revoked", EntityType: "invitation", EntityID: &id, OrgID: inv.OrgID})
	})
}

// AcceptInvitation creates the invited account with a verified email.
func (s *Service) AcceptInvitation(ctx context.Context, token, displayName, plain string, m Meta) error {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len(displayName) > 120 {
		return ErrInvalidInput
	}
	return s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		inv, err := q.GetPendingInvitationByToken(ctx, hashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidToken
		}
		if err != nil {
			return err
		}
		if err := password.Validate(plain, inv.Email); err != nil {
			return err
		}
		hash, err := password.Hash(plain)
		if err != nil {
			return err
		}
		now := time.Now()
		u, err := q.CreateUser(ctx, store.CreateUserParams{Email: inv.Email, DisplayName: displayName, PasswordHash: &hash, EmailVerifiedAt: &now})
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return ErrEmailInUse
		}
		if err != nil {
			return err
		}
		if err := q.AddUserRole(ctx, store.AddUserRoleParams{UserID: u.ID, RoleID: inv.RoleID, OrgID: inv.OrgID, GrantedBy: &inv.InvitedBy}); err != nil {
			return err
		}
		if inv.OrgID != nil {
			if err := q.AddOrgMember(ctx, store.AddOrgMemberParams{OrgID: *inv.OrgID, UserID: u.ID}); err != nil {
				return err
			}
		}
		if err := q.MarkInvitationAccepted(ctx, inv.ID); err != nil {
			return err
		}
		return Audit(ctx, q, m, Entry{Actor: &u.ID, Action: "invitation.accepted", EntityType: "user", EntityID: &u.ID, OrgID: inv.OrgID,
			After: map[string]any{"role": inv.RoleKey, "invitation_id": inv.ID}})
	})
}

// UserSummary is a staff or client user in the administration list.
type UserSummary struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name"`
	Status        string     `json:"status"`
	EmailVerified bool       `json:"email_verified"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	Roles         []string   `json:"roles"` // "role_key" or "role_key:org_id"
}

func (s *Service) ListUsers(ctx context.Context, p *Principal) ([]UserSummary, error) {
	out := []UserSummary{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListUsersWithRoles(ctx)
		for _, r := range rows {
			out = append(out, UserSummary{ID: r.ID, Email: r.Email, DisplayName: r.DisplayName, Status: r.Status,
				EmailVerified: r.EmailVerifiedAt != nil, LastLoginAt: r.LastLoginAt, Roles: r.Roles})
		}
		return err
	})
	return out, err
}

// GrantRole gives an existing user a role. Never a role holding
// system.admin, never the contributor role, never mixing user kinds.
func (s *Service) GrantRole(ctx context.Context, p *Principal, userID uuid.UUID, roleKey string, orgID *uuid.UUID, m Meta) error {
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		role, err := q.GetRoleWithGrantability(ctx, roleKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidInput
		}
		if err != nil {
			return err
		}
		if !role.Grantable || role.Key == "contributor" {
			return ErrNotGrantable
		}
		if err := mayManageRole(p, role.Scope, orgID); err != nil {
			return err
		}
		if _, err := q.GetUserByID(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		existing, err := q.ListUserGrants(ctx, userID)
		if err != nil {
			return err
		}
		grants := []grant{{roleKey: role.Key, roleScope: role.Scope, orgID: orgID, perm: "-"}}
		for _, g := range existing {
			grants = append(grants, grant{roleKey: g.RoleKey, roleScope: g.RoleScope, orgID: g.OrgID, perm: g.PermissionKey})
		}
		if _, _, _, ok := resolveKind(grants); !ok {
			return ErrMixedKinds
		}
		if err := q.AddUserRole(ctx, store.AddUserRoleParams{UserID: userID, RoleID: role.ID, OrgID: orgID, GrantedBy: &p.UserID}); err != nil {
			return foreignKeyAsInvalid(err)
		}
		if orgID != nil {
			if err := q.AddOrgMember(ctx, store.AddOrgMemberParams{OrgID: *orgID, UserID: userID}); err != nil {
				return err
			}
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "user.role_granted", EntityType: "user", EntityID: &userID, OrgID: orgID,
			After: map[string]any{"role": role.Key}})
	})
}

// RemoveRole takes a role away. Super Admins can only be changed by holders
// of system.admin, and the last active Super Admin can never be demoted.
func (s *Service) RemoveRole(ctx context.Context, p *Principal, userID uuid.UUID, roleKey string, orgID *uuid.UUID, m Meta) error {
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		role, err := q.GetRoleWithGrantability(ctx, roleKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalidInput
		}
		if err != nil {
			return err
		}
		if role.Key == "super_admin" {
			if !p.Has(PermSystemAdmin) {
				return ErrForbidden
			}
			n, err := q.LockSuperAdminCount(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastSuperAdmin
			}
		} else if err := mayManageRole(p, role.Scope, orgID); err != nil {
			return err
		}
		removed, err := q.RemoveUserRole(ctx, store.RemoveUserRoleParams{UserID: userID, RoleID: role.ID, OrgID: orgID})
		if err != nil {
			return err
		}
		if removed == 0 {
			return ErrNotFound
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "user.role_removed", EntityType: "user", EntityID: &userID, OrgID: orgID,
			Before: map[string]any{"role": role.Key}})
	})
}

// SetUserStatus enables or disables a staff or client user. Disabling signs
// them out everywhere. Nobody can disable themselves or the last Super Admin.
func (s *Service) SetUserStatus(ctx context.Context, p *Principal, userID uuid.UUID, status string, m Meta) error {
	if status != "active" && status != "disabled" {
		return ErrInvalidInput
	}
	if userID == p.UserID {
		return ErrSelf
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		u, err := q.GetUserByID(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		isSuper, err := q.IsUserSuperAdmin(ctx, userID)
		if err != nil {
			return err
		}
		if isSuper {
			if !p.Has(PermSystemAdmin) {
				return ErrForbidden
			}
			if status == "disabled" {
				n, err := q.LockSuperAdminCount(ctx)
				if err != nil {
					return err
				}
				if n <= 1 {
					return ErrLastSuperAdmin
				}
			}
		}
		if err := q.SetUserStatus(ctx, store.SetUserStatusParams{ID: userID, Status: status}); err != nil {
			return err
		}
		if status == "disabled" {
			if err := q.RevokeUserSessions(ctx, userID); err != nil {
				return err
			}
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "user.status_changed", EntityType: "user", EntityID: &userID,
			Before: map[string]any{"status": u.Status}, After: map[string]any{"status": status}})
	})
}

// Role is a role and its permission keys.
type Role struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

func (s *Service) ListRoles(ctx context.Context, p *Principal) ([]Role, error) {
	out := []Role{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListRolesWithPermissions(ctx)
		for _, r := range rows {
			out = append(out, Role{Key: r.Key, Name: r.Name, Description: r.Description, Scope: r.Scope, Permissions: r.Permissions})
		}
		return err
	})
	return out, err
}

// Permission is a permission key as listed for role configuration.
type Permission struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Grantable   bool   `json:"grantable"`
}

func (s *Service) ListPermissions(ctx context.Context, p *Principal) ([]Permission, error) {
	out := []Permission{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		rows, err := store.New(tx).ListPermissions(ctx)
		for _, r := range rows {
			out = append(out, Permission{Key: r.Key, Description: r.Description, Grantable: r.Grantable})
		}
		return err
	})
	return out, err
}

// SetRolePermissions replaces a role's permission keys. The Super Admin role
// is fixed, and non-grantable keys (system.admin) can never be added.
func (s *Service) SetRolePermissions(ctx context.Context, p *Principal, roleKey string, keys []string, m Meta) error {
	if roleKey == "super_admin" {
		return ErrNotGrantable
	}
	return s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		role, err := q.GetRoleByKey(ctx, roleKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, 0, len(keys))
		seen := map[string]bool{}
		for _, k := range keys {
			if seen[k] {
				continue
			}
			seen[k] = true
			perm, err := q.GetPermissionByKey(ctx, k)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("%w: unknown permission %q", ErrInvalidInput, k)
			}
			if err != nil {
				return err
			}
			if !perm.Grantable {
				return ErrNotGrantable
			}
			ids = append(ids, perm.ID)
		}
		before, err := q.ListRolesWithPermissions(ctx)
		if err != nil {
			return err
		}
		var prev []string
		for _, r := range before {
			if r.Key == roleKey {
				prev = r.Permissions
			}
		}
		if err := q.ClearRolePermissions(ctx, role.ID); err != nil {
			return err
		}
		for _, id := range ids {
			if err := q.AddRolePermission(ctx, store.AddRolePermissionParams{RoleID: role.ID, PermissionID: id}); err != nil {
				return err
			}
		}
		return Audit(ctx, q, m, Entry{Actor: &p.UserID, Action: "role.permissions_changed", EntityType: "role", EntityID: &role.ID,
			Before: map[string]any{"permissions": prev}, After: map[string]any{"permissions": keys}})
	})
}

// foreignKeyAsInvalid turns "no such organisation" into ErrInvalidInput.
func foreignKeyAsInvalid(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" {
		return ErrInvalidInput
	}
	return err
}
