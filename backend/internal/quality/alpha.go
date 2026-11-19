package quality

import (
	"math"
	"slices"
)

// Krippendorff's alpha (Stage 22a, D-163): chance-corrected agreement for
// scores. It takes any number of raters per unit and ignores units with
// fewer than two values, so it works for overlap k of 2 or more and for a
// criterion someone left blank. Computed from the coincidence matrix as in
// Krippendorff, "Computing Krippendorff's Alpha-Reliability" (2011).

// AlphaMetric is the difference function between two values.
type AlphaMetric int

const (
	// Nominal: any two different values disagree equally.
	Nominal AlphaMetric = iota
	// Ordinal: the distance between two ranks grows with how many values
	// were observed between them (1 to 5 rubric scores).
	Ordinal
	// Interval: squared numeric difference.
	Interval
)

// KrippendorffAlpha returns alpha for units of numeric values. ok is false
// when fewer than two values can be paired or every paired value is the
// same (expected disagreement is zero, so alpha is undefined).
func KrippendorffAlpha(units [][]float64, metric AlphaMetric) (alpha float64, ok bool) {
	// Distinct values, ascending.
	values := []float64{}
	for _, u := range units {
		if len(u) < 2 {
			continue
		}
		for _, v := range u {
			if !slices.Contains(values, v) {
				values = append(values, v)
			}
		}
	}
	slices.Sort(values)
	idx := func(v float64) int { return slices.Index(values, v) }
	n := len(values)
	if n == 0 {
		return 0, false
	}

	// Coincidence matrix: each ordered pair of values within a unit, from
	// different raters, weighted 1/(m_u - 1).
	o := make([][]float64, n)
	for i := range o {
		o[i] = make([]float64, n)
	}
	for _, u := range units {
		m := len(u)
		if m < 2 {
			continue
		}
		w := 1 / float64(m-1)
		for x := range u {
			for y := range u {
				if x != y {
					o[idx(u[x])][idx(u[y])] += w
				}
			}
		}
	}
	marg := make([]float64, n)
	total := 0.0
	for c := range o {
		for k := range o[c] {
			marg[c] += o[c][k]
		}
		total += marg[c]
	}
	if total < 2 {
		return 0, false
	}

	delta := func(c, k int) float64 {
		switch metric {
		case Ordinal:
			lo, hi := min(c, k), max(c, k)
			s := 0.0
			for g := lo; g <= hi; g++ {
				s += marg[g]
			}
			s -= (marg[c] + marg[k]) / 2
			return s * s
		case Interval:
			d := values[c] - values[k]
			return d * d
		}
		if c == k {
			return 0
		}
		return 1
	}

	observed, expected := 0.0, 0.0
	for c := 0; c < n; c++ {
		for k := 0; k < n; k++ {
			d := delta(c, k)
			observed += o[c][k] * d
			expected += marg[c] * marg[k] * d
		}
	}
	if expected == 0 {
		return 0, false
	}
	return 1 - (total-1)*observed/expected, true
}

// WithinOne is the share of rater pairs, within each unit, whose scores
// differ by at most one point.
func WithinOne(units [][]float64) (float64, bool) {
	pairs, close := 0, 0
	for _, u := range units {
		for x := 0; x < len(u); x++ {
			for y := x + 1; y < len(u); y++ {
				pairs++
				if math.Abs(u[x]-u[y]) <= 1 {
					close++
				}
			}
		}
	}
	if pairs == 0 {
		return 0, false
	}
	return float64(close) / float64(pairs), true
}
