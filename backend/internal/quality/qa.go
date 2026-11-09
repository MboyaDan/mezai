package quality

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Issue is one automatic QA check result (D-077). Code is stable for the UI;
// Message is plain language for the contributor (blocks) or reviewer (warnings).
type Issue struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Signals are what the task screen reports about how an answer was made.
// They are the contributor's own browser's word, so they only ever warn.
type Signals struct {
	TimeOnTaskMs int64 `json:"time_on_task_ms"`
	// PastedChars is how many characters of the free-text answer (rationale
	// or corrected translation) arrived by paste.
	PastedChars int `json:"pasted_chars"`
}

// CheckRules are a project's QA settings: the D-077 checks plus the minimum
// pairwise rationale length.
type CheckRules struct {
	QA              QAChecks
	MinRationaleLen int
}

// The other response named as better, for the rationale mismatch warning.
var (
	praiseA = regexp.MustCompile(`(?i)\b(response\s+)?a\s+(is|was|seems|reads)\s+(much\s+|clearly\s+|far\s+)?(better|clearer|more\s+(helpful|accurate|correct))`)
	praiseB = regexp.MustCompile(`(?i)\b(response\s+)?b\s+(is|was|seems|reads)\s+(much\s+|clearly\s+|far\s+)?(better|clearer|more\s+(helpful|accurate|correct))`)
)

func norm(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// Check runs the QA checks on a valid answer (Parse first). content is the
// item's fields. Blocks stop the submission; warnings are stored with it.
func Check(taskType string, rules CheckRules, content map[string]string, a Answer, sig Signals) (blocks, warnings []Issue) {
	blocks, warnings = []Issue{}, []Issue{}
	free := ""
	switch taskType {
	case Pairwise:
		free = a.Rationale
		if n := utf8.RuneCountInString(strings.TrimSpace(a.Rationale)); n < rules.MinRationaleLen {
			blocks = append(blocks, Issue{Code: "rationale_too_short", Field: "rationale",
				Message: "Explain your choice in a little more detail before submitting."})
		}
		if rules.QA.WarnRationaleMismatch {
			if (a.Preference == "a" && praiseB.MatchString(a.Rationale)) || (a.Preference == "b" && praiseA.MatchString(a.Rationale)) {
				warnings = append(warnings, Issue{Code: "rationale_mismatch", Field: "rationale",
					Message: "The rationale seems to favour the response that was not chosen."})
			}
		}
	case TranslationReview:
		free = a.Corrected
		if rules.QA.BlockUnchangedTranslation && (a.Adequacy < 5 || a.Fluency < 5) {
			if c := norm(a.Corrected); c == "" || c == norm(content["translation"]) {
				blocks = append(blocks, Issue{Code: "translation_unchanged", Field: "corrected",
					Message: "You scored the translation below 5, so correct it before submitting."})
			}
		}
	}
	if s := rules.QA.WarnFastSeconds; s > 0 && sig.TimeOnTaskMs < int64(s)*1000 {
		warnings = append(warnings, Issue{Code: "too_fast", Message: "Submitted faster than the project's minimum time."})
	}
	if rules.QA.WarnPasted {
		if n := utf8.RuneCountInString(strings.TrimSpace(free)); n >= 20 && sig.PastedChars*2 > n {
			warnings = append(warnings, Issue{Code: "mostly_pasted", Message: "Most of the written answer was pasted rather than typed."})
		}
	}
	return blocks, warnings
}
