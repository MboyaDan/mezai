package quality

import (
	"math"
	"slices"
	"testing"
)

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.6f, want %.6f", name, got, want)
	}
}

// Cohen's kappa, Wikipedia "Cohen's kappa", worked example: 50 proposals,
// both yes 20, A yes/B no 5, A no/B yes 10, both no 15. po = 0.7, pe = 0.5,
// kappa = 0.4.
func TestCohenKappaTextbook(t *testing.T) {
	var a, b []string
	add := func(n int, x, y string) {
		for range n {
			a, b = append(a, x), append(b, y)
		}
	}
	add(20, "yes", "yes")
	add(5, "yes", "no")
	add(10, "no", "yes")
	add(15, "no", "no")
	k, ok := CohenKappa(a, b)
	if !ok {
		t.Fatal("undefined")
	}
	near(t, "kappa", k, 0.4, 1e-9)

	// Perfect agreement over two labels is 1; one label for everyone is undefined.
	if k, ok := CohenKappa([]string{"x", "y"}, []string{"x", "y"}); !ok || k != 1 {
		t.Errorf("perfect: %v %v", k, ok)
	}
	if _, ok := CohenKappa([]string{"x", "x"}, []string{"x", "x"}); ok {
		t.Error("single label should be undefined")
	}
}

// Fleiss' kappa, Wikipedia "Fleiss' kappa", worked example: 10 subjects,
// 14 raters, 5 categories. P-bar = 0.378, Pe-bar = 0.213, kappa = 0.210.
func TestFleissKappaTextbook(t *testing.T) {
	counts := [][]int{
		{0, 0, 0, 0, 14},
		{0, 2, 6, 4, 2},
		{0, 0, 3, 5, 6},
		{0, 3, 9, 2, 0},
		{2, 2, 8, 1, 1},
		{7, 7, 0, 0, 0},
		{3, 2, 6, 3, 0},
		{2, 5, 3, 2, 2},
		{6, 5, 2, 1, 0},
		{0, 2, 2, 3, 7},
	}
	k, ok := FleissKappa(counts)
	if !ok {
		t.Fatal("undefined")
	}
	near(t, "kappa", k, 0.210, 0.0005)

	if _, ok := FleissKappa([][]int{{2, 1}, {3}}); ok {
		t.Error("ragged rows accepted")
	}
	if _, ok := FleissKappa([][]int{{1, 0}}); ok {
		t.Error("one rater accepted")
	}
}

func TestPercentAgreementAndMeanAbsDiff(t *testing.T) {
	// Item 1: 3 pairs all agree; item 2: 1 of 3 pairs agree. 4/6.
	p, ok := PercentAgreement([][]string{{"a", "a", "a"}, {"a", "b", "b"}, {"solo"}})
	if !ok {
		t.Fatal("undefined")
	}
	near(t, "percent", p, 4.0/6, 1e-9)
	// |1-3| = 2; pairs of {2,2,5}: 0, 3, 3. Mean of 2,0,3,3 = 2.
	d, ok := MeanAbsDiff([][]float64{{1, 3}, {2, 2, 5}})
	if !ok {
		t.Fatal("undefined")
	}
	near(t, "mad", d, 2, 1e-9)
}

func TestMajority(t *testing.T) {
	cases := []struct {
		votes []string
		want  string
		ok    bool
	}{
		{[]string{"a", "a", "b"}, "a", true},
		{[]string{"a", "b"}, "", false},      // tie
		{[]string{"a", "b", "c"}, "", false}, // plurality is not a majority
		{[]string{"tie", "tie", "a"}, "tie", true},
		{[]string{"a"}, "a", true},
	}
	for _, c := range cases {
		got, ok := Majority(c.votes)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("%v: %q %v", c.votes, got, ok)
		}
	}
	labels, ok := MajorityLabels([][]string{{"x", "y"}, {"x"}, {"x", "z"}})
	if !ok || !slices.Equal(labels, []string{"x"}) {
		t.Errorf("multi-label: %v %v", labels, ok)
	}
	if _, ok := MajorityLabels([][]string{{"x", "y"}, {"x"}}); ok {
		t.Error("label chosen by exactly half should go to review")
	}
}

func TestJaccardMeanSpreadBand(t *testing.T) {
	near(t, "jaccard", Jaccard([]string{"a", "b", "c", "d", "e"}, []string{"a", "b", "c", "d"}), 0.8, 1e-9)
	near(t, "jaccard disjoint", Jaccard([]string{"a"}, []string{"b"}), 0, 1e-9)
	m, s := MeanSpread([]float64{2, 4, 5})
	near(t, "mean", m, 11.0/3, 1e-9)
	near(t, "spread", s, 3, 1e-9)
	for k, want := range map[float64]string{0.2: "weak", 0.4: "weak", 0.5: "moderate", 0.7: "substantial", 0.81: "strong"} {
		if Band(k) != want {
			t.Errorf("band(%v) = %s", k, Band(k))
		}
	}
}

// Trust, brief 8.4 worked by hand: 0.5×0.90 + 0.3×0.80 + 0.2×0.75 = 0.84.
// Without review data the other two are rescaled: (0.45 + 0.24) / 0.8 = 0.8625.
func TestTrustScore(t *testing.T) {
	w := TrustWeights{GoldAccuracy: 0.5, Agreement: 0.3, ReviewAcceptance: 0.2}
	f := func(v float64) *float64 { return &v }
	s, ok := TrustScore(w, TrustInputs{f(0.9), f(0.8), f(0.75)})
	if !ok {
		t.Fatal("undefined")
	}
	near(t, "full", s, 0.84, 1e-9)
	s, _ = TrustScore(w, TrustInputs{GoldAccuracy: f(0.9), Agreement: f(0.8)})
	near(t, "no reviews", s, 0.8625, 1e-9)
	s, _ = TrustScore(w, TrustInputs{GoldAccuracy: f(0.6)})
	near(t, "gold only", s, 0.6, 1e-9)
	if _, ok := TrustScore(w, TrustInputs{}); ok {
		t.Error("no data should be undefined")
	}
}

// Gold comparison rules, brief 8.1.
func TestGoldRules(t *testing.T) {
	m := func(tt, exp string, a Answer) bool {
		ok, err := Matches(tt, []byte(exp), nil, a)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !m(Classification, `{"labels":["a"]}`, Answer{Labels: []string{"a"}}) || m(Classification, `{"labels":["a"]}`, Answer{Labels: []string{"b"}}) {
		t.Error("single label must match exactly")
	}
	// 4 of 5 labels shared: Jaccard 0.8 passes; 3 of 5 (0.6) fails.
	five := `{"labels":["a","b","c","d","e"]}`
	if !m(Classification, five, Answer{Labels: []string{"a", "b", "c", "d"}}) || m(Classification, five, Answer{Labels: []string{"a", "b", "c"}}) {
		t.Error("multi-label Jaccard 0.8 rule")
	}
	crit := func(v int) map[string]int { return map[string]int{"accuracy": v} }
	exp := `{"preference":"a","scores_a":{"accuracy":5},"scores_b":{"accuracy":2}}`
	if !m(Pairwise, exp, Answer{Preference: "a", ScoresA: crit(4), ScoresB: crit(3)}) {
		t.Error("scores within 1 should match")
	}
	if m(Pairwise, exp, Answer{Preference: "a", ScoresA: crit(3), ScoresB: crit(2)}) {
		t.Error("a score 2 points off should not match")
	}
	if !m(Pairwise, `{"preference":"b"}`, Answer{Preference: "b", ScoresA: crit(1), ScoresB: crit(5)}) {
		t.Error("preference-only gold ignores scores")
	}
}

func TestProjectAgreement(t *testing.T) {
	lbl := func(l ...string) Answer { return Answer{Labels: l} }
	// Two raters on four items: agree on three. Cohen per Wikipedia rules.
	items := [][]Answer{{lbl("a"), lbl("a")}, {lbl("b"), lbl("b")}, {lbl("a"), lbl("b")}, {lbl("b"), lbl("b")}}
	stats := ProjectAgreement(Classification, Config{Labels: []string{"a", "b"}}, items, 2)
	find := func(label, stat string) (float64, bool) {
		for _, s := range stats {
			if s.Label == label && s.Statistic == stat {
				return s.Value, true
			}
		}
		return 0, false
	}
	if v, ok := find("", "percent"); !ok || v != 0.75 {
		t.Fatalf("percent %v %v", v, ok)
	}
	// po = 0.75; rater 1: a 2, b 2; rater 2: a 1, b 3; pe = 0.5×0.25 + 0.5×0.75 = 0.5; kappa = 0.5.
	if v, ok := find("", "cohen_kappa"); !ok || math.Abs(v-0.5) > 1e-9 {
		t.Fatalf("cohen %v %v", v, ok)
	}
	if _, ok := find("a", "percent"); !ok {
		t.Fatal("per-label figures missing")
	}
	// Three raters use Fleiss; one rater gives nothing.
	three := [][]Answer{{lbl("a"), lbl("a"), lbl("b")}, {lbl("b"), lbl("b"), lbl("b")}}
	s3 := ProjectAgreement(Classification, Config{}, three, 3)
	if len(s3) != 2 || s3[1].Statistic != "fleiss_kappa" {
		t.Fatalf("three raters: %+v", s3)
	}
	if ProjectAgreement(Classification, Config{}, [][]Answer{{lbl("a")}}, 1) != nil {
		t.Fatal("k = 1 has no agreement")
	}
	tr := ProjectAgreement(TranslationReview, Config{}, [][]Answer{{{Adequacy: 4, Fluency: 5}, {Adequacy: 2, Fluency: 5}}}, 2)
	if len(tr) != 2 || tr[0].Value != 2 || tr[1].Value != 0 {
		t.Fatalf("translation MAD: %+v", tr)
	}
}
