package quality

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

// Single-response rating (Stage 22b, D-164): every piece of the task type.
var rt = Config{Criteria: []string{"helpful", "accurate"}}

func TestRatingParse(t *testing.T) {
	good := `{"scores":{"helpful":4,"accurate":2},"rationale":"Clear but one date is wrong."}`
	a, err := Parse(Rating, rt, json.RawMessage(good))
	if err != nil || a.Scores["helpful"] != 4 || a.Scores["accurate"] != 2 {
		t.Fatalf("good answer: %+v %v", a, err)
	}
	for _, raw := range []string{
		`{"scores":{"helpful":4},"rationale":"x"}`,                             // criterion missing
		`{"scores":{"helpful":4,"accurate":0},"rationale":"x"}`,                // below 1
		`{"scores":{"helpful":6,"accurate":3},"rationale":"x"}`,                // above 5
		`{"scores":{"helpful":4,"accurate":3,"style":2},"rationale":"x"}`,      // unknown criterion
		`{"scores":{"helpful":4,"accurate":3},"rationale":"  "}`,               // no rationale
		`{"scores":{"helpful":4,"accurate":3},"rationale":"x","preference":1}`, // wrong shape
	} {
		if _, err := Parse(Rating, rt, json.RawMessage(raw)); !errors.Is(err, ErrInvalidAnswer) {
			t.Errorf("%s: want ErrInvalidAnswer, got %v", raw, err)
		}
	}
}

func TestRatingGold(t *testing.T) {
	ans := func(h, a int) Answer {
		return Answer{Scores: map[string]int{"helpful": h, "accurate": a}, Rationale: "r"}
	}
	match := func(exp string, a Answer) bool {
		ok, err := Matches(Rating, json.RawMessage(exp), nil, a)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !match(`{"scores":{"helpful":5,"accurate":2}}`, ans(4, 3)) {
		t.Error("each score within 1 should match")
	}
	if match(`{"scores":{"helpful":5,"accurate":2}}`, ans(3, 2)) {
		t.Error("a score 2 points off should not match")
	}
	if !match(`{"scores":{"accurate":1}}`, ans(1, 2)) {
		t.Error("only the expected criteria are graded")
	}
	if match(`{}`, ans(3, 3)) {
		t.Error("gold without scores never matches")
	}

	cfg := ProjectConfig{Criteria: rt.Criteria}
	if out, err := ValidateExpected(Rating, cfg, json.RawMessage(`{"scores":{"helpful":4}}`)); err != nil || string(out) != `{"scores":{"helpful":4}}` {
		t.Errorf("valid expected: %s %v", out, err)
	}
	for _, raw := range []string{`{}`, `{"scores":{}}`, `{"scores":{"style":3}}`, `{"scores":{"helpful":0}}`, `{"preference":"a"}`} {
		if _, err := ValidateExpected(Rating, cfg, json.RawMessage(raw)); !errors.Is(err, ErrInvalid) {
			t.Errorf("expected %s: want ErrInvalid, got %v", raw, err)
		}
	}
	if _, err := ValidateTolerance(Rating, json.RawMessage(`{"adequacy":2}`)); !errors.Is(err, ErrInvalid) {
		t.Error("rating gold takes no tolerance")
	}
}

func TestRatingSettingsAndContent(t *testing.T) {
	c, err := ValidateConfig(Rating, json.RawMessage(`{"criteria":["helpful","accurate"]}`))
	if err != nil || !slices.Equal(c.Scale, []int{1, 5}) {
		t.Fatalf("config: %+v %v", c, err)
	}
	for _, raw := range []string{`{}`, `{"criteria":["helpful"],"labels":["a","b"]}`, `{"criteria":["helpful"],"error_tags":["x"]}`} {
		if _, err := ValidateConfig(Rating, json.RawMessage(raw)); !errors.Is(err, ErrInvalid) {
			t.Errorf("config %s accepted", raw)
		}
	}
	if !slices.Contains(TaskTypes, Rating) {
		t.Error("rating is not a task type")
	}
	if _, err := ValidateContent(Rating, map[string]string{"prompt": "Sample question?", "response": "Sample answer."}); err != nil {
		t.Errorf("content: %v", err)
	}
	if _, err := ValidateContent(Rating, map[string]string{"prompt": "Q", "response_a": "A"}); !errors.Is(err, ErrInvalid) {
		t.Error("pairwise fields accepted on a rating item")
	}
	if _, err := ValidateQAChecks(Rating, json.RawMessage(`{"warn_rationale_mismatch":true}`)); !errors.Is(err, ErrInvalid) {
		t.Error("rationale mismatch has no meaning without a preference")
	}
	if d := DefaultQAChecks(Rating); !d.WarnPasted || d.WarnRationaleMismatch || d.BlockUnchangedTranslation {
		t.Errorf("defaults: %+v", d)
	}
}

func TestRatingCheck(t *testing.T) {
	a := Answer{Scores: map[string]int{"helpful": 3, "accurate": 3}, Rationale: "Too short."}
	blocks, _ := Check(Rating, CheckRules{MinRationaleLen: 40}, nil, a, Signals{TimeOnTaskMs: 60_000})
	if len(blocks) != 1 || blocks[0].Code != "rationale_too_short" {
		t.Fatalf("short rationale: %+v", blocks)
	}
	a.Rationale = "Answers the question but gives the wrong year for the event."
	_, warnings := Check(Rating, CheckRules{QA: QAChecks{WarnPasted: true}}, nil, a, Signals{TimeOnTaskMs: 60_000, PastedChars: 60})
	if len(warnings) != 1 || warnings[0].Code != "mostly_pasted" {
		t.Fatalf("pasted rationale: %+v", warnings)
	}
}

func TestRatingExport(t *testing.T) {
	cfg := ProjectConfig{Criteria: rt.Criteria}
	if cols := ExportColumns(Rating, cfg); !slices.Equal(cols, []string{"score:helpful", "score:accurate", "rationale"}) {
		t.Errorf("columns: %q", cols)
	}
	// Consensus: mean scores, no rationale. Reviewer: whole scores and a rationale.
	for raw, want := range map[string][]string{
		`{"scores":{"helpful":4.5,"accurate":3}}`:                       {"4.5", "3", ""},
		`{"scores":{"helpful":2},"rationale":"Reviewer's final view."}`: {"2", "", "Reviewer's final view."},
	} {
		got, err := ExportValues(Rating, cfg, []byte(raw))
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%s: %q %v, want %q", raw, got, err, want)
		}
	}
}

// Agreement per criterion uses the 22a figures; each item is one unit.
func TestRatingAgreement(t *testing.T) {
	ans := func(h, a int) Answer { return Answer{Scores: map[string]int{"helpful": h, "accurate": a}} }
	items := [][]Answer{{ans(5, 2), ans(4, 2)}, {ans(1, 4), ans(1, 4)}, {ans(3, 1), ans(5, 1)}}
	got := map[string]float64{}
	for _, s := range ProjectAgreement(Rating, rt, items, 2) {
		got[s.Label+"/"+s.Statistic] = s.Value
	}
	// helpful: (5,4) (1,1) (3,5): exact 1/3, within one 2/3, mean diff 1.
	near(t, "helpful percent", got["helpful/percent"], 1.0/3, 1e-9)
	near(t, "helpful within", got["helpful/within_one"], 2.0/3, 1e-9)
	near(t, "helpful mad", got["helpful/mean_abs_diff"], 1, 1e-9)
	want, _ := KrippendorffAlpha([][]float64{{5, 4}, {1, 1}, {3, 5}}, Ordinal)
	near(t, "helpful alpha", got["helpful/krippendorff_alpha"], want, 1e-12)
	// accurate: every pair identical.
	near(t, "accurate percent", got["accurate/percent"], 1, 1e-9)
	near(t, "accurate alpha", got["accurate/krippendorff_alpha"], 1, 1e-9)
	if _, ok := got["/percent"]; ok {
		t.Error("rating has no project-wide categorical figure")
	}
}
