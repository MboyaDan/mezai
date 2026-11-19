package quality

import (
	"math"
	"testing"
)

// Krippendorff (2011), "Computing Krippendorff's Alpha-Reliability", the
// worked example with four observers, twelve units and missing values
// (section C). Published results: nominal 0.743, ordinal 0.815,
// interval 0.849.
func krippendorffExample() [][]float64 {
	const x = -1 // missing
	obs := [4][12]float64{
		{1, 2, 3, 3, 2, 1, 4, 1, 2, x, x, x},
		{1, 2, 3, 3, 2, 2, 4, 1, 2, 5, x, 3},
		{x, 3, 3, 3, 2, 3, 4, 2, 2, 5, 1, x},
		{1, 2, 3, 3, 2, 4, 4, 1, 2, 5, 1, x},
	}
	units := make([][]float64, 12)
	for u := range units {
		for r := range obs {
			if obs[r][u] != x {
				units[u] = append(units[u], obs[r][u])
			}
		}
	}
	return units
}

func TestKrippendorffAlphaPublishedExample(t *testing.T) {
	units := krippendorffExample()
	for _, c := range []struct {
		name   string
		metric AlphaMetric
		want   float64
	}{{"nominal", Nominal, 0.743}, {"ordinal", Ordinal, 0.815}, {"interval", Interval, 0.849}} {
		got, ok := KrippendorffAlpha(units, c.metric)
		if !ok || math.Abs(got-c.want) > 0.0005 {
			t.Errorf("%s alpha = %.4f (ok %v), want %.3f", c.name, got, ok, c.want)
		}
	}
}

func TestKrippendorffAlphaEdges(t *testing.T) {
	// Perfect agreement with variation across units: alpha 1.
	if a, ok := KrippendorffAlpha([][]float64{{1, 1}, {3, 3}, {5, 5, 5}}, Ordinal); !ok || a != 1 {
		t.Errorf("perfect agreement: %v %v", a, ok)
	}
	// Everyone gave the same score everywhere: no expected disagreement, undefined.
	if _, ok := KrippendorffAlpha([][]float64{{4, 4}, {4, 4}}, Ordinal); ok {
		t.Error("constant scores should be undefined")
	}
	// Units with one value cannot be paired.
	if _, ok := KrippendorffAlpha([][]float64{{1}, {5}}, Ordinal); ok {
		t.Error("single values should be undefined")
	}
	// Systematic disagreement goes below zero.
	if a, ok := KrippendorffAlpha([][]float64{{1, 5}, {5, 1}, {1, 5}, {5, 1}}, Interval); !ok || a >= 0 {
		t.Errorf("disagreement should be negative: %v", a)
	}
}

func TestWithinOne(t *testing.T) {
	// Pairs: (3,4) yes, (3,5) no, (4,5) yes; (1,1) yes. 3 of 4.
	if v, ok := WithinOne([][]float64{{3, 4, 5}, {1, 1}}); !ok || v != 0.75 {
		t.Errorf("within one = %v %v", v, ok)
	}
	if _, ok := WithinOne([][]float64{{2}}); ok {
		t.Error("no pairs should be undefined")
	}
}
