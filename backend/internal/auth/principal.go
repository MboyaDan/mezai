// Package auth signs people in, keeps their sessions, and says what they may do.
package auth

import (
	"sort"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/db"
)

// Kind is the surface a user belongs to; it decides their database scope.
type Kind string

const (
	KindStaff       Kind = "staff"       // ELS staff: Super Admin, Ops Manager, Payout Approver, Reviewer
	KindClient      Kind = "client"      // client users of one organisation
	KindContributor Kind = "contributor" // contributors
)

// PermSystemAdmin implies every permission.
const PermSystemAdmin = "system.admin"

// Principal is the signed-in user for one request.
type Principal struct {
	UserID        uuid.UUID
	Email         string
	DisplayName   string
	Kind          Kind
	OrgID         uuid.UUID // client users only
	ContributorID uuid.UUID // contributors only
	SessionID     uuid.UUID
	CSRFSecret    []byte
	perms         map[string]bool
}

// Has reports whether the principal holds the permission key. Authorisation
// is always by key, never by role name (CLAUDE.md section 2).
func (p *Principal) Has(key string) bool {
	return p != nil && (p.perms[key] || p.perms[PermSystemAdmin])
}

// Permissions returns the held keys, sorted.
func (p *Principal) Permissions() []string {
	out := make([]string, 0, len(p.perms))
	for k := range p.perms {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Scope is the row-level security scope for this principal's transactions.
func (p *Principal) Scope() db.Scope {
	switch p.Kind {
	case KindStaff:
		return db.Staff()
	case KindClient:
		return db.Org(p.OrgID)
	case KindContributor:
		return db.Contributor(p.ContributorID)
	}
	return db.Scope{} // rejected by db.InScope
}

// grant is one row of a user's role grants (see queries/auth.sql ListUserGrants).
type grant struct {
	roleKey, roleScope, perm string
	orgID                    *uuid.UUID
}

// resolveKind works out a user's kind and organisation from their grants.
// Mixing kinds (a staff member who is also a client user) is refused: the
// pilot keeps each person on one surface (A-017).
func resolveKind(grants []grant) (Kind, uuid.UUID, map[string]bool, bool) {
	perms := map[string]bool{}
	kinds := map[Kind]bool{}
	var org uuid.UUID
	for _, g := range grants {
		perms[g.perm] = true
		switch {
		case g.roleScope == "org" && g.orgID != nil:
			kinds[KindClient] = true
			if org != uuid.Nil && org != *g.orgID {
				return "", uuid.Nil, nil, false // client users belong to one organisation
			}
			org = *g.orgID
		case g.roleKey == "contributor":
			kinds[KindContributor] = true
		default:
			kinds[KindStaff] = true
		}
	}
	if len(kinds) != 1 {
		return "", uuid.Nil, nil, false
	}
	for k := range kinds {
		return k, org, perms, true
	}
	return "", uuid.Nil, nil, false
}
