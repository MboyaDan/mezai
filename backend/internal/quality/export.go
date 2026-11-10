package quality

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The export schema (brief 7's fourth piece): how a final answer becomes
// columns in a delivered CSV. JSONL carries the stored answer unchanged.

// ExportSchemaVersion is written into every manifest and data file. Raise it
// whenever a column or field changes meaning.
const ExportSchemaVersion = 1

// finalAnswer reads both kinds of final answer: a consensus result (pairwise
// and translation scores are means, so decimals) and a reviewer's answer.
type finalAnswer struct {
	Labels     []string           `json:"labels"`
	Preference string             `json:"preference"`
	ScoresA    map[string]float64 `json:"scores_a"`
	ScoresB    map[string]float64 `json:"scores_b"`
	Rationale  string             `json:"rationale"`
	Adequacy   float64            `json:"adequacy"`
	Fluency    float64            `json:"fluency"`
	Corrected  string             `json:"corrected"`
	ErrorTags  []string           `json:"error_tags"`
}

// ExportColumns names the answer columns of a CSV export, in order.
func ExportColumns(taskType string, cfg ProjectConfig) []string {
	switch taskType {
	case Classification:
		return []string{"labels"}
	case Pairwise:
		cols := []string{"preference"}
		for _, c := range cfg.Criteria {
			cols = append(cols, "score_a:"+c)
		}
		for _, c := range cfg.Criteria {
			cols = append(cols, "score_b:"+c)
		}
		return append(cols, "rationale")
	case TranslationReview:
		return []string{"adequacy", "fluency", "corrected", "error_tags"}
	}
	return nil
}

// ExportValues turns a stored final answer into the ExportColumns values.
// Lists are joined with "|"; a missing score is an empty cell.
func ExportValues(taskType string, cfg ProjectConfig, raw json.RawMessage) ([]string, error) {
	var a finalAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("quality: final answer: %w", err)
	}
	num := func(m map[string]float64, k string) string {
		if v, ok := m[k]; ok {
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return ""
	}
	switch taskType {
	case Classification:
		return []string{strings.Join(a.Labels, "|")}, nil
	case Pairwise:
		vals := []string{a.Preference}
		for _, c := range cfg.Criteria {
			vals = append(vals, num(a.ScoresA, c))
		}
		for _, c := range cfg.Criteria {
			vals = append(vals, num(a.ScoresB, c))
		}
		return append(vals, a.Rationale), nil
	case TranslationReview:
		return []string{strconv.FormatFloat(a.Adequacy, 'f', -1, 64), strconv.FormatFloat(a.Fluency, 'f', -1, 64),
			a.Corrected, strings.Join(a.ErrorTags, "|")}, nil
	}
	return nil, fmt.Errorf("quality: no export schema for %q", taskType)
}
