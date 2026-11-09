package quality

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Each task type defines four pieces (brief 7): the import schema (item
// content, checked here), the submission schema (Parse), the gold comparison
// (Matches) and the export schema (Stage 11). This file holds the import side
// and the project settings the other pieces depend on.

// ErrInvalid wraps every problem with a project setting, item or gold answer.
// The message is safe to show to Ops and never quotes item content.
var ErrInvalid = errors.New("quality: invalid")

// Invalid is a problem with one named field.
type Invalid struct{ Field, Message string }

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }
func (e *Invalid) Unwrap() error { return ErrInvalid }

func bad(field, format string, a ...any) error {
	return &Invalid{Field: field, Message: fmt.Sprintf(format, a...)}
}

// TaskTypes are the pilot task types, in display order.
var TaskTypes = []string{Classification, Pairwise, TranslationReview}

// MaxFieldChars bounds one text field of an item (characters, not bytes).
const MaxFieldChars = 20000

var (
	criterionRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	// BCP 47-ish: "sw", "en", "sw-KE", "luo". Display names live in the UI.
	langRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
)

// ProjectConfig is a project's task settings as Ops enters them. Config (the
// subset answers need) is derived from it.
type ProjectConfig struct {
	Labels     []string `json:"labels,omitempty"`
	MultiLabel bool     `json:"multi_label,omitempty"`
	Criteria   []string `json:"criteria,omitempty"`
	Scale      []int    `json:"scale,omitempty"` // always [1, 5] in the pilot
	ErrorTags  []string `json:"error_tags,omitempty"`
}

// Answers returns the settings Parse and Matches need.
func (c ProjectConfig) Answers() Config {
	return Config{Labels: c.Labels, MultiLabel: c.MultiLabel, Criteria: c.Criteria}
}

func strictDecode(raw []byte, v any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

func uniqueTrimmed(field string, in []string, min, max, maxLen int, re *regexp.Regexp) ([]string, error) {
	if len(in) < min || len(in) > max {
		return nil, bad(field, "list %d to %d values", min, max)
	}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || utf8.RuneCountInString(v) > maxLen {
			return nil, bad(field, "each value 1 to %d characters", maxLen)
		}
		if re != nil && !re.MatchString(v) {
			return nil, bad(field, "use lower-case letters, digits and underscores, starting with a letter")
		}
		if slices.Contains(out, v) {
			return nil, bad(field, "%q is listed twice", v)
		}
		out = append(out, v)
	}
	return out, nil
}

// ValidateConfig checks a project's task settings and returns them cleaned.
//
//	classification      labels (2 to 50), multi_label
//	pairwise            criteria (1 to 8 rubric keys), scale [1, 5]
//	translation_review  error_tags (optional, up to 30); criteria and scale may
//	                    be given but are fixed to adequacy and fluency, 1 to 5
func ValidateConfig(taskType string, raw json.RawMessage) (ProjectConfig, error) {
	var c ProjectConfig
	if err := strictDecode(raw, &c); err != nil {
		return c, bad("config", "not a settings object for this task type")
	}
	if len(c.Scale) > 0 && !slices.Equal(c.Scale, []int{1, 5}) {
		return c, bad("config.scale", "the pilot scores from 1 to 5")
	}
	var err error
	switch taskType {
	case Classification:
		if len(c.Criteria) > 0 || len(c.Scale) > 0 || len(c.ErrorTags) > 0 {
			return c, bad("config", "classification takes labels and multi_label only")
		}
		if c.Labels, err = uniqueTrimmed("config.labels", c.Labels, 2, 50, 80, nil); err != nil {
			return c, err
		}
	case Pairwise:
		if len(c.Labels) > 0 || c.MultiLabel || len(c.ErrorTags) > 0 {
			return c, bad("config", "pairwise takes criteria and scale only")
		}
		if c.Criteria, err = uniqueTrimmed("config.criteria", c.Criteria, 1, 8, 40, criterionRe); err != nil {
			return c, err
		}
		c.Scale = []int{1, 5}
	case TranslationReview:
		if len(c.Labels) > 0 || c.MultiLabel {
			return c, bad("config", "translation review takes error_tags only")
		}
		if len(c.Criteria) > 0 && !slices.Equal(c.Criteria, []string{"adequacy", "fluency"}) {
			return c, bad("config.criteria", "translation review scores adequacy and fluency")
		}
		if len(c.ErrorTags) > 0 {
			if c.ErrorTags, err = uniqueTrimmed("config.error_tags", c.ErrorTags, 1, 30, 40, criterionRe); err != nil {
				return c, err
			}
		}
		c.Criteria, c.Scale = []string{"adequacy", "fluency"}, []int{1, 5}
	default:
		return c, bad("task_type", "use classification, pairwise or translation_review")
	}
	return c, nil
}

// ContentFields lists an item's fields per task type (the import schemas in
// migration 00004). All are required, non-empty strings.
func ContentFields(taskType string) []string {
	switch taskType {
	case Classification:
		return []string{"text"}
	case Pairwise:
		return []string{"prompt", "response_a", "response_b"}
	case TranslationReview:
		return []string{"source", "translation", "source_lang", "target_lang"}
	case Writing:
		return []string{"prompt"}
	}
	return nil
}

// ValidateContent checks one item's fields and returns its JSON content.
// Unknown fields are refused, so stray columns (which may hold personal
// data) never enter the platform.
func ValidateContent(taskType string, fields map[string]string) (json.RawMessage, error) {
	want := ContentFields(taskType)
	if want == nil {
		return nil, bad("task_type", "unknown task type")
	}
	for k := range fields {
		if !slices.Contains(want, k) {
			return nil, bad(k, "is not a field of this task type (expected %s)", strings.Join(want, ", "))
		}
	}
	out := make(map[string]string, len(want))
	for _, k := range want {
		v := fields[k]
		if strings.TrimSpace(v) == "" {
			return nil, bad(k, "is required")
		}
		if !utf8.ValidString(v) {
			return nil, bad(k, "is not valid UTF-8 text")
		}
		if utf8.RuneCountInString(v) > MaxFieldChars {
			return nil, bad(k, "is longer than %d characters", MaxFieldChars)
		}
		if (k == "source_lang" || k == "target_lang") && !langRe.MatchString(strings.TrimSpace(v)) {
			return nil, bad(k, "use a language code such as sw, en or sw-KE")
		}
		if k == "source_lang" || k == "target_lang" {
			v = strings.TrimSpace(v)
		}
		out[k] = v
	}
	if taskType == TranslationReview && out["source_lang"] == out["target_lang"] {
		return nil, bad("target_lang", "must differ from source_lang")
	}
	b, err := json.Marshal(out)
	return b, err
}

// ValidateExpected checks a gold item's expected answer. Only the graded
// parts are required: the label set (classification), the preference
// (pairwise), adequacy and fluency (translation review; a reference
// correction is optional).
func ValidateExpected(taskType string, cfg ProjectConfig, raw json.RawMessage) (json.RawMessage, error) {
	var a Answer
	if err := strictDecode(raw, &a); err != nil {
		return nil, bad("expected", "not an answer for this task type")
	}
	var out any
	switch taskType {
	case Classification:
		if len(a.Labels) == 0 {
			return nil, bad("expected.labels", "give the correct label")
		}
		if !cfg.MultiLabel && len(a.Labels) > 1 {
			return nil, bad("expected.labels", "this project takes one label per item")
		}
		seen := map[string]bool{}
		for _, l := range a.Labels {
			if seen[l] || !slices.Contains(cfg.Labels, l) {
				return nil, bad("expected.labels", "%q is not one of the project's labels, or is repeated", l)
			}
			seen[l] = true
		}
		out = map[string]any{"labels": a.Labels}
	case Pairwise:
		if a.Preference != "a" && a.Preference != "b" && a.Preference != "tie" {
			return nil, bad("expected.preference", "use a, b or tie")
		}
		out = map[string]any{"preference": a.Preference}
	case TranslationReview:
		if a.Adequacy < 1 || a.Adequacy > 5 || a.Fluency < 1 || a.Fluency > 5 {
			return nil, bad("expected", "give adequacy and fluency from 1 to 5")
		}
		if utf8.RuneCountInString(a.Corrected) > MaxFieldChars {
			return nil, bad("expected.corrected", "is longer than %d characters", MaxFieldChars)
		}
		m := map[string]any{"adequacy": a.Adequacy, "fluency": a.Fluency}
		if strings.TrimSpace(a.Corrected) != "" {
			m["corrected"] = a.Corrected
		}
		out = m
	default:
		return nil, bad("task_type", "unknown task type")
	}
	return json.Marshal(out)
}

// ValidateTolerance checks a gold item's tolerance: translation review only,
// each from 0 to 4 points. Empty means the default (±1).
func ValidateTolerance(taskType string, raw json.RawMessage) (json.RawMessage, error) {
	var t Tolerance
	if err := strictDecode(raw, &t); err != nil {
		return nil, bad("tolerance", "use adequacy and fluency only")
	}
	if t.Adequacy == nil && t.Fluency == nil {
		return json.RawMessage(`{}`), nil
	}
	if taskType != TranslationReview {
		return nil, bad("tolerance", "only translation review gold takes a tolerance")
	}
	for _, v := range []*int{t.Adequacy, t.Fluency} {
		if v != nil && (*v < 0 || *v > 4) {
			return nil, bad("tolerance", "use 0 to 4 points")
		}
	}
	return json.Marshal(t)
}

// QAChecks are the checks the task screen runs before a contributor submits
// (D-077). Blocking checks stop the submission; warnings let it through and
// are stored with it for reviewers. Stage 9 enforces them.
type QAChecks struct {
	// Block: a translation correction identical to the machine translation
	// when adequacy or fluency is below 5.
	BlockUnchangedTranslation bool `json:"block_unchanged_translation"`
	// Warn when a task is submitted faster than this many seconds (0 = off).
	WarnFastSeconds int `json:"warn_fast_seconds"`
	// Warn when the pairwise rationale prefers the other response by name.
	WarnRationaleMismatch bool `json:"warn_rationale_mismatch"`
	// Warn when most of a free-text answer was pasted rather than typed.
	WarnPasted bool `json:"warn_pasted"`
}

// DefaultQAChecks applies to new projects.
func DefaultQAChecks(taskType string) QAChecks {
	return QAChecks{
		BlockUnchangedTranslation: taskType == TranslationReview,
		WarnFastSeconds:           5,
		WarnRationaleMismatch:     taskType == Pairwise,
		WarnPasted:                taskType != Classification,
	}
}

// ValidateQAChecks checks the settings; empty input gives the defaults.
func ValidateQAChecks(taskType string, raw json.RawMessage) (QAChecks, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "{}" || string(bytes.TrimSpace(raw)) == "null" {
		return DefaultQAChecks(taskType), nil
	}
	var q QAChecks
	if err := strictDecode(raw, &q); err != nil {
		return q, bad("qa_checks", "unknown check")
	}
	switch {
	case q.WarnFastSeconds < 0 || q.WarnFastSeconds > 600:
		return q, bad("qa_checks.warn_fast_seconds", "use 0 to 600 seconds")
	case q.BlockUnchangedTranslation && taskType != TranslationReview:
		return q, bad("qa_checks.block_unchanged_translation", "applies to translation review only")
	case q.WarnRationaleMismatch && taskType != Pairwise:
		return q, bad("qa_checks.warn_rationale_mismatch", "applies to pairwise comparison only")
	}
	return q, nil
}
