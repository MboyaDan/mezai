package quality

import (
	"strings"
	"testing"
)

func codes(is []Issue) string {
	out := []string{}
	for _, i := range is {
		out = append(out, i.Code)
	}
	return strings.Join(out, ",")
}

func TestCheckPairwise(t *testing.T) {
	rules := CheckRules{QA: QAChecks{WarnRationaleMismatch: true, WarnFastSeconds: 5, WarnPasted: true}, MinRationaleLen: 20}
	slow := Signals{TimeOnTaskMs: 60_000}
	cases := []struct {
		name          string
		a             Answer
		sig           Signals
		blocks, warns string
	}{
		{"clean", Answer{Preference: "a", Rationale: "A answers the question and B does not."}, slow, "", ""},
		{"short", Answer{Preference: "a", Rationale: "A is fine."}, slow, "rationale_too_short", ""},
		{"mismatch", Answer{Preference: "a", Rationale: "Honestly response B is much better here."}, slow, "", "rationale_mismatch"},
		{"mismatch b", Answer{Preference: "b", Rationale: "A seems clearer but it misses the point."}, slow, "", "rationale_mismatch"},
		{"tie never mismatches", Answer{Preference: "tie", Rationale: "B is better on tone, A on accuracy."}, slow, "", ""},
		{"fast and pasted", Answer{Preference: "a", Rationale: "A answers the question and B does not."}, Signals{TimeOnTaskMs: 3000, PastedChars: 30}, "", "too_fast,mostly_pasted"},
	}
	for _, c := range cases {
		b, w := Check(Pairwise, rules, nil, c.a, c.sig)
		if codes(b) != c.blocks || codes(w) != c.warns {
			t.Errorf("%s: blocks %q warnings %q", c.name, codes(b), codes(w))
		}
	}
}

func TestCheckTranslation(t *testing.T) {
	rules := CheckRules{QA: QAChecks{BlockUnchangedTranslation: true}}
	content := map[string]string{"translation": "Habari  ya asubuhi"}
	slow := Signals{TimeOnTaskMs: 60_000}
	cases := []struct {
		a      Answer
		blocks string
	}{
		{Answer{Adequacy: 5, Fluency: 5}, ""}, // perfect: nothing to correct
		{Answer{Adequacy: 3, Fluency: 5, Corrected: "habari ya asubuhi"}, "translation_unchanged"},
		{Answer{Adequacy: 3, Fluency: 4}, "translation_unchanged"}, // empty correction
		{Answer{Adequacy: 3, Fluency: 4, Corrected: "Habari za asubuhi"}, ""},
	}
	for i, c := range cases {
		if b, _ := Check(TranslationReview, rules, content, c.a, slow); codes(b) != c.blocks {
			t.Errorf("case %d: blocks %q", i, codes(b))
		}
	}
	// Off: nothing blocks.
	if b, _ := Check(TranslationReview, CheckRules{}, content, Answer{Adequacy: 2, Fluency: 2}, slow); len(b) != 0 {
		t.Error("check off still blocks")
	}
}

func TestCheckClassificationOnlyTimes(t *testing.T) {
	rules := CheckRules{QA: QAChecks{WarnFastSeconds: 5, WarnPasted: true}}
	_, w := Check(Classification, rules, nil, Answer{Labels: []string{"x"}}, Signals{TimeOnTaskMs: 1000, PastedChars: 500})
	if codes(w) != "too_fast" {
		t.Fatalf("warnings %q", codes(w))
	}
}

func TestWritingAnswer(t *testing.T) {
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }
	for n, ok := range map[int]bool{59: false, 60: true, 150: true, 250: true, 251: false} {
		_, err := Parse(Writing, Config{}, []byte(`{"text":"`+words(n)+`"}`))
		if (err == nil) != ok {
			t.Errorf("%d words: %v", n, err)
		}
	}
	if AutoScored(Writing) || !AutoScored(Classification) {
		t.Fatal("only writing is read by a person")
	}
}
