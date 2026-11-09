// Package quality checks answers against expected (gold) answers. Stage 7
// uses it to score qualification tests; Stage 10 reuses it for gold scoring.
package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Task types, as stored in projects.task_type and qualification_tests.task_type.
const (
	Classification    = "classification"
	Pairwise          = "pairwise"
	TranslationReview = "translation_review"
	// Writing is a short free-text sample (the core check, D-100). A person
	// rates it; it is never auto-scored.
	Writing = "writing"
)

// Writing samples are 60 to 250 words (the prompt asks for 80 to 150).
const (
	MinWritingWords = 60
	MaxWritingWords = 250
)

// ErrInvalidAnswer wraps every shape or range problem in an answer.
var ErrInvalidAnswer = errors.New("quality: invalid answer")

// Config is the part of a project's (or test's) settings an answer depends on:
// labels for classification, rubric criteria for pairwise (scored 1 to 5).
type Config struct {
	Labels     []string `json:"labels,omitempty"`
	MultiLabel bool     `json:"multi_label,omitempty"`
	Criteria   []string `json:"criteria,omitempty"`
}

// Answer is the union of the three task types' answers, in the shape of the
// task templates' submission schemas (migration 00004). Unused fields stay empty.
type Answer struct {
	Labels     []string       `json:"labels,omitempty"`     // classification
	Confidence int            `json:"confidence,omitempty"` // classification, optional 1 to 5
	Note       string         `json:"note,omitempty"`       // classification, optional
	ScoresA    map[string]int `json:"scores_a,omitempty"`   // pairwise, each criterion 1 to 5
	ScoresB    map[string]int `json:"scores_b,omitempty"`   // pairwise
	Preference string         `json:"preference,omitempty"` // pairwise: a, b or tie
	Rationale  string         `json:"rationale,omitempty"`  // pairwise
	Adequacy   int            `json:"adequacy,omitempty"`   // translation review, 1 to 5
	Fluency    int            `json:"fluency,omitempty"`    // translation review, 1 to 5
	Corrected  string         `json:"corrected,omitempty"`  // translation review
	ErrorTags  []string       `json:"error_tags,omitempty"` // translation review, optional
	Text       string         `json:"text,omitempty"`       // writing sample
}

const maxText = 4000

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidAnswer, fmt.Sprintf(format, a...))
}

// Parse decodes raw and checks it is a complete answer for the task type.
func Parse(taskType string, cfg Config, raw json.RawMessage) (Answer, error) {
	var a Answer
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return Answer{}, invalid("not an answer object")
	}
	switch taskType {
	case Classification:
		if len(a.Labels) == 0 {
			return Answer{}, invalid("choose a label")
		}
		if !cfg.MultiLabel && len(a.Labels) > 1 {
			return Answer{}, invalid("choose one label")
		}
		if a.Confidence != 0 && (a.Confidence < 1 || a.Confidence > 5) {
			return Answer{}, invalid("confidence from 1 to 5")
		}
		if len(a.Note) > maxText {
			return Answer{}, invalid("note too long")
		}
		seen := map[string]bool{}
		for _, l := range a.Labels {
			if seen[l] || (len(cfg.Labels) > 0 && !slices.Contains(cfg.Labels, l)) {
				return Answer{}, invalid("unknown or repeated label")
			}
			seen[l] = true
		}
	case Pairwise:
		if a.Preference != "a" && a.Preference != "b" && a.Preference != "tie" {
			return Answer{}, invalid("choose a, b or tie")
		}
		for side, scores := range map[string]map[string]int{"A": a.ScoresA, "B": a.ScoresB} {
			for _, c := range cfg.Criteria {
				if v, ok := scores[c]; !ok || v < 1 || v > 5 {
					return Answer{}, invalid("score %s for response %s from 1 to 5", c, side)
				}
			}
			for k := range scores {
				if !slices.Contains(cfg.Criteria, k) {
					return Answer{}, invalid("unknown rubric criterion")
				}
			}
		}
		if t := strings.TrimSpace(a.Rationale); t == "" || len(t) > maxText {
			return Answer{}, invalid("write a rationale")
		}
	case TranslationReview:
		if a.Adequacy < 1 || a.Adequacy > 5 || a.Fluency < 1 || a.Fluency > 5 {
			return Answer{}, invalid("score adequacy and fluency from 1 to 5")
		}
		if len(a.Corrected) > maxText || len(a.ErrorTags) > 20 {
			return Answer{}, invalid("correction or tags too long")
		}
	case Writing:
		n := len(strings.Fields(a.Text))
		if n < MinWritingWords || n > MaxWritingWords || len(a.Text) > maxText {
			return Answer{}, invalid("write %d to %d words", MinWritingWords, MaxWritingWords)
		}
	default:
		return Answer{}, invalid("unknown task type %q", taskType)
	}
	return a, nil
}

// AutoScored reports whether answers of this type are checked against an
// expected answer (everything except writing samples).
func AutoScored(taskType string) bool { return taskType != Writing }

// Tolerance loosens a match. Translation scores default to ±1 point.
type Tolerance struct {
	Adequacy *int `json:"adequacy,omitempty"`
	Fluency  *int `json:"fluency,omitempty"`
}

// Matches reports whether a valid answer agrees with the expected answer:
// the same label set (classification), the same preference (pairwise; rubric
// scores and rationale are not graded), or adequacy and fluency each within
// tolerance (translation review).
func Matches(taskType string, expected, tolerance json.RawMessage, a Answer) (bool, error) {
	var want Answer
	if err := json.Unmarshal(expected, &want); err != nil {
		return false, fmt.Errorf("quality: expected answer: %w", err)
	}
	switch taskType {
	case Classification:
		x, y := slices.Clone(a.Labels), slices.Clone(want.Labels)
		slices.Sort(x)
		slices.Sort(y)
		return len(y) > 0 && slices.Equal(x, y), nil
	case Pairwise:
		return want.Preference != "" && a.Preference == want.Preference, nil
	case TranslationReview:
		tol := Tolerance{}
		if len(tolerance) > 0 {
			if err := json.Unmarshal(tolerance, &tol); err != nil {
				return false, fmt.Errorf("quality: tolerance: %w", err)
			}
		}
		within := func(got, exp int, t *int) bool {
			d := 1
			if t != nil {
				d = *t
			}
			return math.Abs(float64(got-exp)) <= float64(d)
		}
		return want.Adequacy > 0 && within(a.Adequacy, want.Adequacy, tol.Adequacy) &&
			want.Fluency > 0 && within(a.Fluency, want.Fluency, tol.Fluency), nil
	}
	return false, fmt.Errorf("quality: unknown task type %q", taskType)
}
