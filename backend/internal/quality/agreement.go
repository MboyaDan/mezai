package quality

import (
	"math"
	"slices"
)

// Agreement statistics (brief 8.2). Every function is pure and checked
// against published worked examples in agreement_test.go (AC-08).

// CohenKappa is Cohen's kappa for two raters who labelled the same items:
// a[i] and b[i] are the labels for item i. ok is false when it is undefined
// (no items, or expected agreement of 1, e.g. both raters always used one label).
func CohenKappa(a, b []string) (kappa float64, ok bool) {
	if len(a) == 0 || len(a) != len(b) {
		return 0, false
	}
	n := float64(len(a))
	countA, countB := map[string]float64{}, map[string]float64{}
	agree := 0.0
	for i := range a {
		countA[a[i]]++
		countB[b[i]]++
		if a[i] == b[i] {
			agree++
		}
	}
	po := agree / n
	pe := 0.0
	for label, ca := range countA {
		pe += (ca / n) * (countB[label] / n)
	}
	if pe >= 1 {
		return 0, false
	}
	return (po - pe) / (1 - pe), true
}

// FleissKappa is Fleiss' kappa for items each rated by the same number of
// raters: counts[i][j] is how many raters put item i in category j. ok is
// false when undefined (fewer than 2 raters, no items, or chance agreement 1).
func FleissKappa(counts [][]int) (kappa float64, ok bool) {
	if len(counts) == 0 {
		return 0, false
	}
	n := 0
	for _, c := range counts[0] {
		n += c
	}
	if n < 2 {
		return 0, false
	}
	categories := len(counts[0])
	pj := make([]float64, categories)
	sumPi := 0.0
	for _, row := range counts {
		if len(row) != categories {
			return 0, false
		}
		total, sq := 0, 0
		for j, c := range row {
			total += c
			sq += c * c
			pj[j] += float64(c)
		}
		if total != n {
			return 0, false // every item must have the same number of raters
		}
		sumPi += float64(sq-n) / float64(n*(n-1))
	}
	N := float64(len(counts))
	pBar := sumPi / N
	pe := 0.0
	for j := range pj {
		p := pj[j] / (N * float64(n))
		pe += p * p
	}
	if pe >= 1 {
		return 0, false
	}
	return (pBar - pe) / (1 - pe), true
}

// PercentAgreement is the share of rater pairs, over all items, that gave
// the same label: ratings[i] are the labels item i received. Items with
// fewer than two ratings are skipped.
func PercentAgreement(ratings [][]string) (float64, bool) {
	pairs, agree := 0, 0
	for _, r := range ratings {
		for x := 0; x < len(r); x++ {
			for y := x + 1; y < len(r); y++ {
				pairs++
				if r[x] == r[y] {
					agree++
				}
			}
		}
	}
	if pairs == 0 {
		return 0, false
	}
	return float64(agree) / float64(pairs), true
}

// MeanAbsDiff is the mean absolute difference between rater pairs on a
// numeric score (brief 8.2: rubric agreement in the pilot).
func MeanAbsDiff(scores [][]float64) (float64, bool) {
	pairs, sum := 0, 0.0
	for _, s := range scores {
		for x := 0; x < len(s); x++ {
			for y := x + 1; y < len(s); y++ {
				pairs++
				sum += math.Abs(s[x] - s[y])
			}
		}
	}
	if pairs == 0 {
		return 0, false
	}
	return sum / float64(pairs), true
}

// Majority returns the value chosen by more than half of the votes. ok is
// false on a tie or when no value has a majority (brief 7: ties go to review).
func Majority(votes []string) (winner string, ok bool) {
	count := map[string]int{}
	for _, v := range votes {
		count[v]++
		if 2*count[v] > len(votes) {
			winner, ok = v, true
		}
	}
	return winner, ok
}

// MajorityLabels is multi-label majority voting: a label is in the result
// when more than half of the raters chose it. ok is false when any label is
// chosen by exactly half (a tie per label goes to review).
func MajorityLabels(sets [][]string) (labels []string, ok bool) {
	count := map[string]int{}
	for _, s := range sets {
		for _, l := range s {
			count[l]++
		}
	}
	n := len(sets)
	ok = n > 0
	for l, c := range count {
		switch {
		case 2*c > n:
			labels = append(labels, l)
		case 2*c == n:
			ok = false
		}
	}
	slices.Sort(labels)
	return labels, ok && len(labels) > 0
}

// Jaccard is |A ∩ B| / |A ∪ B| for two label sets (1 when both are empty).
func Jaccard(a, b []string) float64 {
	in := map[string]bool{}
	for _, x := range a {
		in[x] = true
	}
	union := len(in)
	inter := 0
	seen := map[string]bool{}
	for _, y := range b {
		if seen[y] {
			continue
		}
		seen[y] = true
		if in[y] {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(inter) / float64(union)
}

// MeanSpread returns the mean and the spread (max − min) of scores.
func MeanSpread(scores []float64) (mean, spread float64) {
	if len(scores) == 0 {
		return 0, 0
	}
	lo, hi, sum := scores[0], scores[0], 0.0
	for _, s := range scores {
		sum += s
		lo, hi = min(lo, s), max(hi, s)
	}
	return sum / float64(len(scores)), hi - lo
}

// Band is the interpretation shown next to a kappa (brief 8.2). The number
// is always shown too.
func Band(kappa float64) string {
	switch {
	case kappa > 0.80:
		return "strong"
	case kappa > 0.60:
		return "substantial"
	case kappa > 0.40:
		return "moderate"
	}
	return "weak"
}

// TrustWeights are the trust score weights (settings key trust.weights).
type TrustWeights struct {
	GoldAccuracy     float64 `json:"gold_accuracy"`
	Agreement        float64 `json:"agreement"`
	ReviewAcceptance float64 `json:"review_acceptance"`
}

// TrustInputs are the three rates over the rolling window; nil means no
// data yet for that input.
type TrustInputs struct {
	GoldAccuracy     *float64
	Agreement        *float64
	ReviewAcceptance *float64
}

// TrustScore is brief 8.4: 0.5 × gold accuracy + 0.3 × agreement with the
// final answer + 0.2 × review acceptance. An input without data is left out
// and the remaining weights are rescaled to sum to 1 (D-106), so a
// contributor is not marked down for reviews that have not happened yet.
// ok is false when no input has data.
func TrustScore(w TrustWeights, in TrustInputs) (score float64, ok bool) {
	sum, weight := 0.0, 0.0
	for _, p := range []struct {
		w float64
		v *float64
	}{{w.GoldAccuracy, in.GoldAccuracy}, {w.Agreement, in.Agreement}, {w.ReviewAcceptance, in.ReviewAcceptance}} {
		if p.v != nil && p.w > 0 {
			sum += p.w * *p.v
			weight += p.w
		}
	}
	if weight == 0 {
		return 0, false
	}
	return math.Min(1, math.Max(0, sum/weight)), true
}
