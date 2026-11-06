package quality

import (
	"encoding/json"
	"errors"
	"testing"
)

var (
	cls = Config{Labels: []string{"Billing", "Network", "Other"}}
	pw  = Config{Criteria: []string{"accuracy", "safety"}}
)

func TestParseRejectsBadAnswers(t *testing.T) {
	bad := []struct {
		task string
		cfg  Config
		raw  string
	}{
		{Classification, cls, `{}`},
		{Classification, cls, `{"labels":["Billing","Network"]}`}, // single label
		{Classification, cls, `{"labels":["Weather"]}`},
		{Classification, Config{Labels: cls.Labels, MultiLabel: true}, `{"labels":["Billing","Billing"]}`},
		{Classification, cls, `{"labels":["Billing"],"extra":1}`},
		{Classification, cls, `[]`},
		{Classification, cls, `{"labels":["Billing"],"confidence":9}`},
		{Pairwise, pw, `{"preference":"c","scores_a":{"accuracy":3,"safety":3},"scores_b":{"accuracy":3,"safety":3},"rationale":"x"}`},
		{Pairwise, pw, `{"preference":"a","scores_a":{"accuracy":3,"safety":3},"scores_b":{"accuracy":3},"rationale":"x"}`},
		{Pairwise, pw, `{"preference":"a","scores_a":{"accuracy":6,"safety":3},"scores_b":{"accuracy":3,"safety":3},"rationale":"x"}`},
		{Pairwise, pw, `{"preference":"a","scores_a":{"accuracy":3,"safety":3,"style":2},"scores_b":{"accuracy":3,"safety":3},"rationale":"x"}`},
		{Pairwise, pw, `{"preference":"a","scores_a":{"accuracy":3,"safety":3},"scores_b":{"accuracy":3,"safety":3},"rationale":"   "}`},
		{TranslationReview, Config{}, `{"adequacy":0,"fluency":3}`},
		{TranslationReview, Config{}, `{"adequacy":3,"fluency":6}`},
		{"audio", Config{}, `{}`},
	}
	for _, b := range bad {
		if _, err := Parse(b.task, b.cfg, json.RawMessage(b.raw)); !errors.Is(err, ErrInvalidAnswer) {
			t.Errorf("%s %s: want ErrInvalidAnswer, got %v", b.task, b.raw, err)
		}
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		task          string
		cfg           Config
		expected, tol string
		answer        string
		want          bool
	}{
		{Classification, cls, `{"labels":["Billing"]}`, `{}`, `{"labels":["Billing"]}`, true},
		{Classification, cls, `{"labels":["Billing"]}`, `{}`, `{"labels":["Network"]}`, false},
		{Classification, Config{Labels: cls.Labels, MultiLabel: true}, `{"labels":["Network","Billing"]}`, `{}`, `{"labels":["Billing","Network"]}`, true},
		{Classification, Config{Labels: cls.Labels, MultiLabel: true}, `{"labels":["Network","Billing"]}`, `{}`, `{"labels":["Billing"]}`, false},
		{Pairwise, pw, `{"preference":"a"}`, `{}`, `{"preference":"a","scores_a":{"accuracy":4,"safety":5},"scores_b":{"accuracy":1,"safety":1},"rationale":"A is safer."}`, true},
		{Pairwise, pw, `{"preference":"a"}`, `{}`, `{"preference":"tie","scores_a":{"accuracy":4,"safety":4},"scores_b":{"accuracy":4,"safety":4},"rationale":"Both fine."}`, false},
		{TranslationReview, Config{}, `{"adequacy":5,"fluency":5}`, `{}`, `{"adequacy":4,"fluency":5}`, true},  // default ±1
		{TranslationReview, Config{}, `{"adequacy":5,"fluency":5}`, `{}`, `{"adequacy":3,"fluency":5}`, false}, // 2 off
		{TranslationReview, Config{}, `{"adequacy":4,"fluency":3}`, `{"fluency":0}`, `{"adequacy":4,"fluency":4}`, false},
		{TranslationReview, Config{}, `{"adequacy":4,"fluency":3}`, `{"adequacy":2}`, `{"adequacy":2,"fluency":3}`, true},
	}
	for _, c := range cases {
		a, err := Parse(c.task, c.cfg, json.RawMessage(c.answer))
		if err != nil {
			t.Fatalf("%s %s: %v", c.task, c.answer, err)
		}
		got, err := Matches(c.task, json.RawMessage(c.expected), json.RawMessage(c.tol), a)
		if err != nil || got != c.want {
			t.Errorf("%s expected %s answer %s: got %v, %v; want %v", c.task, c.expected, c.answer, got, err, c.want)
		}
	}
}
