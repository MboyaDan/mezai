package quality

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	ok := map[string]string{
		Classification:    `{"labels":[" Billing ","Network"],"multi_label":true}`,
		Pairwise:          `{"criteria":["accuracy","helpfulness"],"scale":[1,5]}`,
		TranslationReview: `{"criteria":["adequacy","fluency"],"scale":[1,5],"error_tags":["omission"]}`,
	}
	for tt, raw := range ok {
		if _, err := ValidateConfig(tt, json.RawMessage(raw)); err != nil {
			t.Errorf("%s: %v", tt, err)
		}
	}
	c, _ := ValidateConfig(Classification, json.RawMessage(ok[Classification]))
	if c.Labels[0] != "Billing" {
		t.Errorf("labels not trimmed: %q", c.Labels)
	}
	c, _ = ValidateConfig(TranslationReview, nil)
	if len(c.Criteria) != 2 || len(c.Scale) != 2 {
		t.Errorf("translation defaults = %+v", c)
	}
	bad := map[string][2]string{
		"one label":      {Classification, `{"labels":["Only"]}`},
		"repeated label": {Classification, `{"labels":["A","A"]}`},
		"rubric on cls":  {Classification, `{"labels":["A","B"],"criteria":["x"]}`},
		"no criteria":    {Pairwise, `{}`},
		"bad criterion":  {Pairwise, `{"criteria":["Has Space"]}`},
		"other scale":    {Pairwise, `{"criteria":["a"],"scale":[0,10]}`},
		"unknown key":    {Pairwise, `{"criteria":["a"],"colour":"red"}`},
		"tr criteria":    {TranslationReview, `{"criteria":["style"]}`},
		"unknown type":   {"ranking", `{}`},
	}
	for name, c := range bad {
		if _, err := ValidateConfig(c[0], json.RawMessage(c[1])); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestValidateContent(t *testing.T) {
	if _, err := ValidateContent(Pairwise, map[string]string{"prompt": "Q", "response_a": "A", "response_b": "B"}); err != nil {
		t.Fatal(err)
	}
	for name, f := range map[string]map[string]string{
		"missing":     {"prompt": "Q", "response_a": "A"},
		"blank":       {"prompt": " ", "response_a": "A", "response_b": "B"},
		"extra field": {"prompt": "Q", "response_a": "A", "response_b": "B", "email": "x@y.invalid"},
	} {
		if _, err := ValidateContent(Pairwise, f); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	tr := map[string]string{"source": "Hi", "translation": "Habari", "source_lang": "en", "target_lang": "Swahili"}
	if _, err := ValidateContent(TranslationReview, tr); err == nil {
		t.Error("language name accepted as a code")
	}
}

func TestValidateExpectedAndTolerance(t *testing.T) {
	cfg := ProjectConfig{Labels: []string{"A", "B"}}
	if out, err := ValidateExpected(Classification, cfg, json.RawMessage(`{"labels":["A"]}`)); err != nil || string(out) != `{"labels":["A"]}` {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := ValidateExpected(Classification, cfg, json.RawMessage(`{"labels":["A","B"]}`)); err == nil {
		t.Error("two labels accepted on a single-label project")
	}
	if _, err := ValidateExpected(Pairwise, ProjectConfig{}, json.RawMessage(`{"preference":"c"}`)); err == nil {
		t.Error("bad preference accepted")
	}
	out, err := ValidateExpected(TranslationReview, ProjectConfig{}, json.RawMessage(`{"adequacy":4,"fluency":5,"corrected":"Habari"}`))
	if err != nil || string(out) != `{"adequacy":4,"corrected":"Habari","fluency":5}` {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := ValidateTolerance(Classification, json.RawMessage(`{"adequacy":1}`)); err == nil {
		t.Error("tolerance accepted on classification")
	}
	if _, err := ValidateTolerance(TranslationReview, json.RawMessage(`{"fluency":5}`)); err == nil {
		t.Error("tolerance above 4 accepted")
	}
}

func TestQAChecks(t *testing.T) {
	d, err := ValidateQAChecks(TranslationReview, nil)
	if err != nil || !d.BlockUnchangedTranslation || d.WarnRationaleMismatch {
		t.Fatalf("defaults = %+v %v", d, err)
	}
	if _, err := ValidateQAChecks(Classification, json.RawMessage(`{"block_unchanged_translation":true}`)); err == nil {
		t.Error("translation check accepted on classification")
	}
	if _, err := ValidateQAChecks(Pairwise, json.RawMessage(`{"warn_fast_seconds":-1}`)); err == nil {
		t.Error("negative seconds accepted")
	}
	if _, err := ValidateQAChecks(Pairwise, json.RawMessage(`{"spellcheck":true}`)); err == nil {
		t.Error("unknown check accepted")
	}
}
