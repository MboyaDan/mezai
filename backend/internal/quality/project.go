package quality

import (
	"slices"
	"strconv"
	"strings"
)

// Stat is one agreement figure for a project (brief 8.2), as stored in
// agreement_snapshots. Label is "" for the whole project.
type Stat struct {
	Label     string
	Statistic string // percent, cohen_kappa, fleiss_kappa, krippendorff_alpha, within_one, mean_abs_diff
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
// rubric criterion figures (scored, below); rating gets the criterion
// figures only; translation review gets them
// for adequacy and fluency.
func ProjectAgreement(taskType string, cfg Config, items [][]Answer, k int) []Stat {
	if k < 2 || len(items) == 0 {
		return nil
	}
	out := []Stat{}
	// scored adds the figures for one rubric criterion (D-163): exact
	// agreement, agreement within one point, ordinal Krippendorff's alpha
	// and the mean absolute difference. Each side of a pairwise item is its
	// own unit, so A and B scores are compared side by side, not averaged.
	scored := func(label string, gets ...func(Answer) float64) {
		units := [][]float64{}
		for _, it := range items {
			for _, get := range gets {
				u := make([]float64, 0, len(it))
				for _, a := range it {
					u = append(u, get(a))
				}
				units = append(units, u)
			}
		}
		stat := func(name string, v float64, ok bool) {
			if ok {
				out = append(out, Stat{Label: label, Statistic: name, Value: v, Items: len(items), Raters: k})
			}
		}
		exact := make([][]string, len(units))
		for i, u := range units {
			for _, v := range u {
				exact[i] = append(exact[i], strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
		v, ok := PercentAgreement(exact)
		stat("percent", v, ok)
		v, ok = WithinOne(units)
		stat("within_one", v, ok)
		v, ok = KrippendorffAlpha(units, Ordinal)
		stat("krippendorff_alpha", v, ok)
		v, ok = MeanAbsDiff(units)
		stat("mean_abs_diff", v, ok)
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
			scored(c, func(a Answer) float64 { return float64(a.ScoresA[c]) },
				func(a Answer) float64 { return float64(a.ScoresB[c]) })
		}
	case Rating:
		for _, c := range cfg.Criteria {
			scored(c, func(a Answer) float64 { return float64(a.Scores[c]) })
		}
	case TranslationReview:
		scored("adequacy", func(a Answer) float64 { return float64(a.Adequacy) })
		scored("fluency", func(a Answer) float64 { return float64(a.Fluency) })
	}
	return out
}
