package projects

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// ClientVisibility says which project panels the client's users see (brief
// 6.3, D-167). Progress, datasets, deliverables and data retention are always
// shared; these two can be held back by Ops. Stored in
// projects.client_visibility: a missing key means shown, so projects made
// before Stage 25 keep showing everything.
type ClientVisibility struct {
	// Quality: gold accuracy, agreement and review rate on the project page.
	Quality bool `json:"quality"`
	// Workforce: contributor counts, skills and per-client codes.
	Workforce bool `json:"workforce"`
}

// Panels a setting can hold back.
const (
	PanelQuality   = "quality"
	PanelWorkforce = "workforce"
)

// ErrNotShared means Ops has not shared this panel with the client.
var ErrNotShared = errors.New("projects: not shared with the client")

func parseVisibility(raw []byte) ClientVisibility {
	v := ClientVisibility{Quality: true, Workforce: true}
	var m map[string]bool
	if json.Unmarshal(raw, &m) == nil {
		if b, ok := m[PanelQuality]; ok {
			v.Quality = b
		}
		if b, ok := m[PanelWorkforce]; ok {
			v.Workforce = b
		}
	}
	return v
}

func (v ClientVisibility) shows(panel string) bool {
	switch panel {
	case PanelQuality:
		return v.Quality
	case PanelWorkforce:
		return v.Workforce
	}
	return false
}

// SetClientVisibility stores what the client sees. Allowed in every project
// state, closed included: it changes the display, not the work.
func (s *Service) SetClientVisibility(ctx context.Context, p *auth.Principal, id uuid.UUID, v ClientVisibility, m auth.Meta) (Project, error) {
	if p.Kind != auth.KindStaff {
		return Project{}, ErrForbidden
	}
	var out Project
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		pr, err := q.LockProject(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		before := parseVisibility(pr.ClientVisibility)
		if before != v {
			if _, err := q.SetClientVisibility(ctx, store.SetClientVisibilityParams{ID: id, ClientVisibility: mustJSON(v)}); err != nil {
				return err
			}
			if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "project.client_visibility", EntityType: "project",
				EntityID: &id, OrgID: &pr.OrgID, Before: before, After: v}); err != nil {
				return err
			}
		}
		out, err = s.load(ctx, q, p, id)
		return err
	})
	return out, err
}

// ClientSees answers ErrNotShared when p is a client user and Ops holds the
// panel back on project id. Staff always see it. A project the caller cannot
// see answers nil, so the route's own handler gives its usual not-found.
func (s *Service) ClientSees(ctx context.Context, p *auth.Principal, id uuid.UUID, panel string) error {
	if p == nil || p.Kind != auth.KindClient {
		return nil
	}
	var raw []byte
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		raw, err = store.New(tx).GetClientVisibility(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !parseVisibility(raw).shows(panel) {
		return ErrNotShared
	}
	return nil
}
