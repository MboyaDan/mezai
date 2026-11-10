package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// The quality engine (Stage 10a, brief 8). It runs inside the submit
// transaction: a gold answer is scored at once (never shown to the
// contributor), an item that reaches k answers gets its consensus or goes to
// review, and every affected contributor's trust score is recalculated.

// trustSettings are the settings-table values (brief 8.4: settings, not code).
type trustSettings struct {
	weights       quality.TrustWeights
	floor         float64
	windowDays    int32
	minGold       int32
	probationMult float64
	probationMax  float64
}

func loadTrustSettings(ctx context.Context, q *store.Queries) (trustSettings, error) {
	ts := trustSettings{weights: quality.TrustWeights{GoldAccuracy: 0.5, Agreement: 0.3, ReviewAcceptance: 0.2},
		floor: 0.70, windowDays: 30, minGold: 20, probationMult: 2, probationMax: 0.5}
	num := func(key string, dst *float64) error {
		v, err := q.GetSetting(ctx, key)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // keep the brief's default
		}
		if err != nil {
			return err
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
		if err != nil {
			return fmt.Errorf("setting %s: %w", key, err)
		}
		*dst = f
		return nil
	}
	var window, minGold float64 = float64(ts.windowDays), float64(ts.minGold)
	for _, s := range []struct {
		key string
		dst *float64
	}{{"trust.floor", &ts.floor}, {"trust.window_days", &window}, {"trust.min_gold_results", &minGold},
		{"trust.probation_gold_multiplier", &ts.probationMult}, {"trust.probation_gold_max", &ts.probationMax}} {
		if err := num(s.key, s.dst); err != nil {
			return ts, err
		}
	}
	ts.windowDays, ts.minGold = int32(window), int32(minGold)
	if v, err := q.GetSetting(ctx, "trust.weights"); err == nil {
		if err := json.Unmarshal(v, &ts.weights); err != nil {
			return ts, fmt.Errorf("setting trust.weights: %w", err)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ts, err
	}
	return ts, nil
}

// goldRateFor is the project's gold rate, raised during probation (brief
// 8.4: fewer than trust.min_gold_results gold results).
func goldRateFor(ctx context.Context, q *store.Queries, contributor, project uuid.UUID, rate float64) (float64, error) {
	if rate <= 0 {
		return 0, nil // no gold on this project: neither probation nor warm-up adds any
	}
	warm, err := warmupRate(ctx, q, contributor, project)
	if err != nil {
		return rate, err
	}
	rate = math.Max(rate, warm)
	ts, err := loadTrustSettings(ctx, q)
	if err != nil {
		return rate, err
	}
	gs, err := q.GoldStats(ctx, store.GoldStatsParams{ContributorID: contributor, Days: ts.windowDays})
	if err != nil {
		return rate, err
	}
	if gs.AllTime < ts.minGold {
		return math.Min(ts.probationMax, math.Max(rate, rate*ts.probationMult)), nil
	}
	return rate, nil
}

// scoreGold records whether a gold answer matched (brief 8.1 rules).
func scoreGold(ctx context.Context, q *store.Queries, taskType string, itemID, submissionID, contributor uuid.UUID, ans quality.Answer) error {
	g, err := q.GetGoldForItem(ctx, itemID)
	if err != nil {
		return err
	}
	ok, err := quality.Matches(taskType, g.Expected, g.Tolerance, ans)
	if err != nil {
		return err
	}
	score := 0.0
	if ok {
		score = 1
	}
	return q.AddGoldResult(ctx, store.AddGoldResultParams{OrgID: g.OrgID, GoldItemID: g.ID, SubmissionID: submissionID,
		ContributorID: contributor, Correct: ok, Score: score, Detail: []byte(`{}`)})
}

// consensusOutcome is the result of combining an item's answers.
type consensusOutcome struct {
	method    string // majority or mean
	result    any
	agrees    map[uuid.UUID]bool // per submission; empty when k = 1
	agreement *float64
	spread    *float64
	reason    string // no_majority, spread, flagged, audit_sample, or ""
}

func normText(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// combine applies brief section 7's consensus rule for the task type.
func combine(taskType string, cfg quality.Config, threshold float64, rows []store.ListFinalAnswersForItemRow) (consensusOutcome, error) {
	answers := make([]quality.Answer, len(rows))
	for i, r := range rows {
		if err := json.Unmarshal(r.Answer, &answers[i]); err != nil {
			return consensusOutcome{}, err
		}
	}
	out := consensusOutcome{agrees: map[uuid.UUID]bool{}}
	share := func(n int) {
		if len(rows) > 1 {
			v := float64(n) / float64(len(rows))
			out.agreement = &v
		}
	}
	switch taskType {
	case quality.Classification:
		out.method = "majority"
		var final []string
		var ok bool
		if cfg.MultiLabel {
			sets := make([][]string, len(answers))
			for i, a := range answers {
				sets[i] = a.Labels
			}
			final, ok = quality.MajorityLabels(sets)
		} else {
			votes := make([]string, len(answers))
			for i, a := range answers {
				if len(a.Labels) > 0 {
					votes[i] = a.Labels[0]
				}
			}
			var w string
			w, ok = quality.Majority(votes)
			final = []string{w}
		}
		if !ok {
			out.reason, out.result = "no_majority", map[string]any{"labels": []string{}}
			return out, nil
		}
		out.result = map[string]any{"labels": final}
		n := 0
		for i, a := range answers {
			agree := quality.Jaccard(a.Labels, final) == 1
			out.agrees[rows[i].ID] = agree
			if agree {
				n++
			}
		}
		share(n)
	case quality.Pairwise:
		out.method = "majority"
		prefs := make([]string, len(answers))
		for i, a := range answers {
			prefs[i] = a.Preference
		}
		pref, ok := quality.Majority(prefs)
		means := map[string]map[string]float64{"scores_a": {}, "scores_b": {}}
		worst := 0.0
		for _, side := range []string{"scores_a", "scores_b"} {
			for _, c := range cfg.Criteria {
				vals := make([]float64, 0, len(answers))
				for _, a := range answers {
					sc := a.ScoresA
					if side == "scores_b" {
						sc = a.ScoresB
					}
					vals = append(vals, float64(sc[c]))
				}
				m, sp := quality.MeanSpread(vals)
				means[side][c] = math.Round(m*100) / 100
				worst = math.Max(worst, sp)
			}
		}
		out.spread = &worst
		out.result = map[string]any{"preference": pref, "scores_a": means["scores_a"], "scores_b": means["scores_b"]}
		if !ok {
			out.reason = "no_majority"
			return out, nil
		}
		n := 0
		for i, a := range answers {
			out.agrees[rows[i].ID] = a.Preference == pref
			if a.Preference == pref {
				n++
			}
		}
		share(n)
		if worst > threshold {
			out.reason = "spread"
		}
	case quality.TranslationReview:
		out.method = "mean"
		ad, fl := make([]float64, len(answers)), make([]float64, len(answers))
		corrections := map[string]bool{}
		for i, a := range answers {
			ad[i], fl[i] = float64(a.Adequacy), float64(a.Fluency)
			corrections[normText(a.Corrected)] = true
		}
		mAd, sAd := quality.MeanSpread(ad)
		mFl, sFl := quality.MeanSpread(fl)
		worst := math.Max(sAd, sFl)
		out.spread = &worst
		corrected := ""
		if len(corrections) == 1 {
			corrected = answers[0].Corrected
		}
		out.result = map[string]any{"adequacy": math.Round(mAd*100) / 100, "fluency": math.Round(mFl*100) / 100, "corrected": corrected}
		n := 0
		for i, a := range answers {
			agree := math.Abs(float64(a.Adequacy)-mAd) <= 1 && math.Abs(float64(a.Fluency)-mFl) <= 1
			out.agrees[rows[i].ID] = agree
			if agree {
				n++
			}
		}
		share(n)
		switch {
		case worst > threshold:
			out.reason = "spread"
		case len(corrections) > 1:
			out.reason = "no_majority" // corrected text chosen at review (brief 7)
		}
	default:
		return out, fmt.Errorf("tasks: no consensus rule for %q", taskType)
	}
	if len(rows) == 1 {
		out.agrees = map[uuid.UUID]bool{} // one rater: nothing to agree with
	}
	return out, nil
}

// runConsensus combines an item's answers once it has k of them, routes it
// to review when brief 8.3 says so, and records each rater's agreement.
// It returns the contributors whose trust inputs changed.
func (s *Service) runConsensus(ctx context.Context, q *store.Queries, pr store.GetProjectForWorkRow, itemID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.ListFinalAnswersForItem(ctx, itemID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	cfg, err := quality.ValidateConfig(pr.TaskType, pr.Config)
	if err != nil {
		return nil, err
	}
	out, err := combine(pr.TaskType, cfg.Answers(), pr.ReviewSpreadThreshold, rows)
	if err != nil {
		return nil, err
	}
	if out.reason == "" {
		for _, r := range rows {
			var w []json.RawMessage
			if json.Unmarshal(r.QaWarnings, &w) == nil && len(w) > 0 {
				out.reason = "flagged" // a QA warning: a person looks (D-077)
				break
			}
		}
	}
	if out.reason == "" && pr.AuditSampleRate > 0 && s.rnd() < pr.AuditSampleRate {
		out.reason = "audit_sample"
	}
	result, _ := json.Marshal(out.result)
	var reason *string
	status := "consensus"
	if out.reason != "" {
		reason, status = &out.reason, "in_review"
	}
	if err := q.AddConsensus(ctx, store.AddConsensusParams{OrgID: pr.OrgID, ProjectID: pr.ID, ItemID: itemID, Method: out.method,
		Result: result, Agreement: out.agreement, Spread: out.spread, RoutedToReview: reason != nil, RouteReason: reason}); err != nil {
		return nil, err
	}
	if err := q.SetItemQualityStatus(ctx, store.SetItemQualityStatusParams{ID: itemID, Status: status}); err != nil {
		return nil, err
	}
	// The final answer is known only without review; Stage 11 records
	// agreement for reviewed items when the reviewer decides.
	var touched []uuid.UUID
	if reason == nil {
		for _, r := range rows {
			if agree, ok := out.agrees[r.ID]; ok {
				if err := q.SetSubmissionAgreement(ctx, store.SetSubmissionAgreementParams{ID: r.ID, Agrees: &agree}); err != nil {
					return nil, err
				}
				touched = append(touched, r.ContributorID)
			}
		}
	}
	return touched, nil
}

// TrustSnapshot is a contributor's latest trust score and its inputs.
type TrustSnapshot struct {
	Score            *float64 `json:"score"`
	GoldAccuracy     *float64 `json:"gold_accuracy"`
	Agreement        *float64 `json:"agreement"`
	ReviewAcceptance *float64 `json:"review_acceptance"`
	GoldCount        int32    `json:"gold_count"`
	Building         bool     `json:"building"` // fewer gold results than the minimum (brief 8.4)
	Paused           bool     `json:"-"`
}

// recomputeTrust recalculates one contributor's trust score over the rolling
// window, stores it, and pauses them (notifying Ops) when it is below the
// floor and no longer building.
func (s *Service) recomputeTrust(ctx context.Context, q *store.Queries, m auth.Meta, contributor uuid.UUID) (TrustSnapshot, error) {
	ts, err := loadTrustSettings(ctx, q)
	if err != nil {
		return TrustSnapshot{}, err
	}
	gs, err := q.GoldStats(ctx, store.GoldStatsParams{ContributorID: contributor, Days: ts.windowDays})
	if err != nil {
		return TrustSnapshot{}, err
	}
	as, err := q.AgreementStats(ctx, store.AgreementStatsParams{ContributorID: contributor, Days: ts.windowDays})
	if err != nil {
		return TrustSnapshot{}, err
	}
	rs, err := q.ReviewStats(ctx, store.ReviewStatsParams{ContributorID: contributor, Days: ts.windowDays})
	if err != nil {
		return TrustSnapshot{}, err
	}
	rate := func(part, whole int32) *float64 {
		if whole == 0 {
			return nil
		}
		v := math.Round(float64(part)/float64(whole)*10000) / 10000
		return &v
	}
	snap := TrustSnapshot{GoldAccuracy: rate(gs.WindowCorrect, gs.WindowTotal), Agreement: rate(as.Agreed, as.Total),
		ReviewAcceptance: rate(rs.Accepted, rs.Decided), GoldCount: gs.WindowTotal, Building: gs.AllTime < ts.minGold}
	if v, ok := quality.TrustScore(ts.weights, quality.TrustInputs{GoldAccuracy: snap.GoldAccuracy, Agreement: snap.Agreement,
		ReviewAcceptance: snap.ReviewAcceptance}); ok {
		v = math.Round(v*10000) / 10000
		snap.Score = &v
	}
	weights, _ := json.Marshal(ts.weights)
	if _, err := q.AddTrustScore(ctx, store.AddTrustScoreParams{ContributorID: contributor, Score: snap.Score, GoldAccuracy: snap.GoldAccuracy,
		AgreementRate: snap.Agreement, ReviewAcceptance: snap.ReviewAcceptance, GoldCount: gs.WindowTotal, WindowDays: ts.windowDays,
		Weights: weights}); err != nil {
		return snap, err
	}
	if snap.Building || snap.Score == nil || *snap.Score >= ts.floor {
		return snap, nil
	}
	reason := fmt.Sprintf("Trust score %.2f below the floor %.2f", *snap.Score, ts.floor)
	paused, err := contributors.PauseForTrust(ctx, q, m, contributor, reason)
	if err != nil || !paused {
		return snap, err
	}
	snap.Paused = true
	return snap, s.notifyPause(ctx, q, contributor, snap, ts.floor)
}

// notifyPause emails the Ops inbox (brief 8.4: Ops is notified).
func (s *Service) notifyPause(ctx context.Context, q *store.Queries, contributor uuid.UUID, snap TrustSnapshot, floor float64) error {
	if s.opts.NotifyTo == "" {
		return nil
	}
	cp, err := q.GetContributorProfile(ctx, contributor)
	if err != nil {
		return err
	}
	pct := func(v *float64) string {
		if v == nil {
			return "no data yet"
		}
		return fmt.Sprintf("%.0f%%", *v*100)
	}
	return mail.Enqueue(ctx, q, s.opts.NotifyTo, mail.TemplateTrustPause, map[string]string{
		"Code": cp.PublicCode, "Score": fmt.Sprintf("%.2f", *snap.Score), "Floor": fmt.Sprintf("%.2f", floor),
		"Gold": pct(snap.GoldAccuracy), "Agreement": pct(snap.Agreement), "Review": pct(snap.ReviewAcceptance),
		"Link": s.opts.AppOrigin + "/w/contributors/" + contributor.String(),
	})
}
