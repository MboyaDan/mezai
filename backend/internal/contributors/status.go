package contributors

import (
	"context"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Contributor statuses (brief 9): applied → qualifying → active ⇄ paused → removed.
const (
	StatusApplied    = "applied"
	StatusQualifying = "qualifying"
	StatusActive     = "active"
	StatusPaused     = "paused"
	StatusRemoved    = "removed"
)

// Via is who or what asks for a status change.
type Via int

const (
	// ViaProfile: the contributor completed their profile (applied → qualifying).
	ViaProfile Via = iota
	// ViaApproval: Ops approved a passed qualification attempt (qualifying → active).
	ViaApproval
	// ViaOps: Ops pauses, reinstates or removes by hand.
	ViaOps
	// ViaTrust: the trust score fell below the floor (Stage 10).
	ViaTrust
)

// CanTransition is the contributor state machine. Only Ops makes a
// contributor active or removed; removed is final and keeps ledger history.
func CanTransition(from, to string, via Via) bool {
	switch via {
	case ViaProfile:
		return from == StatusApplied && to == StatusQualifying
	case ViaApproval:
		return from == StatusQualifying && to == StatusActive
	case ViaOps:
		switch to {
		case StatusPaused:
			return from == StatusActive
		case StatusActive:
			return from == StatusPaused
		case StatusRemoved:
			return from != StatusRemoved && validStatus(from)
		}
	case ViaTrust:
		return from == StatusActive && to == StatusPaused
	}
	return false
}

func validStatus(s string) bool {
	switch s {
	case StatusApplied, StatusQualifying, StatusActive, StatusPaused, StatusRemoved:
		return true
	}
	return false
}

// PauseForTrust pauses an active contributor whose trust score fell below
// the floor (brief 8.4), through the state machine (history and audit).
// It reports whether the contributor was paused; anyone not active is left
// as they are.
func PauseForTrust(ctx context.Context, q *store.Queries, m auth.Meta, id uuid.UUID, reason string) (bool, error) {
	cp, err := q.LockContributorProfile(ctx, id)
	if err != nil {
		return false, err
	}
	if cp.Status != StatusActive {
		return false, nil
	}
	if err := transition(ctx, q, m, id, StatusActive, StatusPaused, ViaTrust, nil, reason); err != nil {
		return false, err
	}
	return true, nil
}
