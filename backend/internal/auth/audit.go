package auth

import (
	"context"
	"encoding/json"
	"net/netip"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Meta describes the request behind a change, for the audit log.
type Meta struct {
	IP        *netip.Addr
	UserAgent string
	RequestID string
}

// Entry is one audit record. Write it in the same transaction as the change.
// Before and After must never hold secrets, tokens, full phone numbers or
// task content (CLAUDE.md section 2).
type Entry struct {
	Actor      *uuid.UUID // nil for system actions
	Action     string     // for example "auth.login"
	EntityType string
	EntityID   *uuid.UUID
	OrgID      *uuid.UUID
	Reason     string
	Before     any
	After      any
}

// Audit inserts e into audit_log.
func Audit(ctx context.Context, q *store.Queries, m Meta, e Entry) error {
	kind := "system"
	if e.Actor != nil {
		kind = "user"
	}
	var reason *string
	if e.Reason != "" {
		reason = &e.Reason
	}
	var reqID *string
	if m.RequestID != "" {
		reqID = &m.RequestID
	}
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	return q.InsertAuditLog(ctx, store.InsertAuditLogParams{
		ActorUserID: e.Actor, ActorKind: kind, Action: e.Action, EntityType: e.EntityType,
		EntityID: e.EntityID, OrgID: e.OrgID, Reason: reason, Before: before, After: after,
		RequestID: reqID, Ip: m.IP,
	})
}

func jsonOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
