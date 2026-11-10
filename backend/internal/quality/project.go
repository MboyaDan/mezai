package quality

import (
	"slices"
	"strings"
)

// Stat is one agreement figure for a project (brief 8.2), as stored in
// agreement_snapshots. Label is "" for the whole project.
type Stat struct {
	Label     string
	Statistic string // percent, cohen_kappa, fleiss_kappa, mean_abs_diff
	Value     float64
	Items     int
	Raters    int
}

// categorical adds percent agreement and the chance-corrected kappa (Cohen
// for two raters, Fleiss for three or more) for one label or the project.
func categorical(label string, ratings [][]string, k int) []Stat {
	out := []Stat{}
	if p, ok := PercentAgreement(ratings); ok {
		out = append(out, Stat{Label: label, Statistic: "percent", Value: p, Items: len(ratings), Raters: k})
	}
	if k == 2 {
		a, b := make([]string, len(ratings)), make([]string, len(ratings))
		for i, r := range ratings {
			a[i], b[i] = r[0], r[1]
		}
		if v, ok := CohenKappa(a, b); ok {
			out = append(out, Stat{Label: label, Statistic: "cohen_kappa", Value: v, Items: len(ratings), Raters: k})
		}
	} else if k >= 3 {
		cats := []string{}
		for _, r := range ratings {
			for _, v := range r {
				if !slices.Contains(cats, v) {
					cats = append(cats, v)
				}
			}
		}
		counts := make([][]int, len(ratings))
		for i, r := range ratings {
			counts[i] = make([]int, len(cats))
			for _, v := range r {
				counts[i][slices.Index(cats, v)]++
			}
		}
		if v, ok := FleissKappa(counts); ok {
			out = append(out, Stat{Label: label, Statistic: "fleiss_kappa", Value: v, Items: len(ratings), Raters: k})
		}
	}
	return out
}

// ProjectAgreement computes brief 8.2's agreement figures from items that
// each have exactly k answers (gold items excluded by the caller):
// classification gets percent and kappa for the project and per label (the
// label chosen or not); pairwise gets them on the preference plus the mean
// absolute difference of rubric scores per criterion; translation review
// gets the mean absolute difference of adequacy and fluency.
func ProjectAgreement(taskType string, cfg Config, items [][]Answer, k int) []Stat {
	if k < 2 || len(items) == 0 {
		return nil
	}
	out := []Stat{}
	mad := func(label string, get func(Answer) float64) {
		scores := make([][]float64, len(items))
		for i, it := range items {
			for _, a := range it {
				scores[i] = append(scores[i], get(a))
			}
		}
		if v, ok := MeanAbsDiff(scores); ok {
			out = append(out, Stat{Label: label, Statistic: "mean_abs_diff", Value: v, Items: len(items), Raters: k})
		}
	}
	switch taskType {
	case Classification:
		whole := make([][]string, len(items))
		for i, it := range items {
			for _, a := range it {
				l := slices.Clone(a.Labels)
				slices.Sort(l)
				whole[i] = append(whole[i], strings.Join(l, "|"))
			}
		}
		out = append(out, categorical("", whole, k)...)
		for _, label := range cfg.Labels {
			per := make([][]string, len(items))
			for i, it := range items {
				for _, a := range it {
					v := "no"
					if slices.Contains(a.Labels, label) {
						v = "yes"
					}
					per[i] = append(per[i], v)
				}
			}
			out = append(out, categorical(label, per, k)...)
		}
	case Pairwise:
		prefs := make([][]string, len(items))
		for i, it := range items {
			for _, a := range it {
				prefs[i] = append(prefs[i], a.Preference)
			}
		}
		out = append(out, categorical("", prefs, k)...)
		for _, c := range cfg.Criteria {
			mad(c, func(a Answer) float64 { return float64(a.ScoresA[c]+a.ScoresB[c]) / 2 })
		}
	case TranslationReview:
		mad("adequacy", func(a Answer) float64 { return float64(a.Adequacy) })
		mad("fluency", func(a Answer) float64 { return float64(a.Fluency) })
	}
	return out
}
