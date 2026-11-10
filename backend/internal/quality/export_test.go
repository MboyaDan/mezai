package quality

import (
	"slices"
	"testing"
)

func TestExportValues(t *testing.T) {
	pw := ProjectConfig{Criteria: []string{"accuracy", "safety"}}
	cases := []struct {
		task   string
		cfg    ProjectConfig
		answer string
		want   []string
	}{
		{Classification, ProjectConfig{}, `{"labels":["Billing"]}`, []string{"Billing"}},
		{Classification, ProjectConfig{}, `{"labels":["Billing","Network"]}`, []string{"Billing|Network"}},
		// A consensus result: mean scores, no rationale.
		{Pairwise, pw, `{"preference":"a","scores_a":{"accuracy":4.33,"safety":5},"scores_b":{"accuracy":2,"safety":3.5}}`,
			[]string{"a", "4.33", "5", "2", "3.5", ""}},
		// A reviewer's answer: whole scores and a rationale; a missing score stays empty.
		{Pairwise, pw, `{"preference":"tie","scores_a":{"accuracy":3},"scores_b":{"accuracy":3,"safety":4},"rationale":"Both fine."}`,
			[]string{"tie", "3", "", "3", "4", "Both fine."}},
		{TranslationReview, ProjectConfig{}, `{"adequacy":4.5,"fluency":4,"corrected":"Habari ya asubuhi.","error_tags":["grammar","terms"]}`,
			[]string{"4.5", "4", "Habari ya asubuhi.", "grammar|terms"}},
	}
	for _, c := range cases {
		got, err := ExportValues(c.task, c.cfg, []byte(c.answer))
		if err != nil {
			t.Fatalf("%s %s: %v", c.task, c.answer, err)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s %s: got %q, want %q", c.task, c.answer, got, c.want)
		}
		if cols := ExportColumns(c.task, c.cfg); len(cols) != len(got) {
			t.Errorf("%s: %d columns for %d values", c.task, len(cols), len(got))
		}
	}
	if cols := ExportColumns(Pairwise, pw); !slices.Equal(cols, []string{"preference", "score_a:accuracy", "score_a:safety", "score_b:accuracy", "score_b:safety", "rationale"}) {
		t.Errorf("pairwise columns: %q", cols)
	}
	if _, err := ExportValues(Writing, ProjectConfig{}, []byte(`{}`)); err == nil {
		t.Error("writing has no export schema")
	}
}
