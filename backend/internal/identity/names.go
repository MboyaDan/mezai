package identity

import (
	"strings"
	"unicode"
)

// Results of comparing a profile name with the name M-Pesa has for a number.
const (
	NameMatch       = "match"
	NameMismatch    = "mismatch"
	NameUnavailable = "unavailable" // M-Pesa sent no name
)

// CompareNames compares the contributor's profile name with the registered
// name M-Pesa returns with a payment ("254712345678 - JANE WANJIRU DOE").
// Only the result is kept; the registered name is never stored (D-139).
//
// People register two or three names in any order and often leave one out
// on a form, so the rule is: the names match when they share at least two
// words, ignoring case, order and punctuation. A one-word name on either
// side matches when the other contains that word. Anything else is a
// mismatch for a person to look at; it never blocks a payment already made.
func CompareNames(profile, registered string) string {
	if i := strings.LastIndex(registered, " - "); i >= 0 {
		registered = registered[i+3:]
	}
	a, b := nameWords(profile), nameWords(registered)
	if len(b) == 0 {
		return NameUnavailable
	}
	if len(a) == 0 {
		return NameMismatch
	}
	common := 0
	for w := range a {
		if b[w] {
			common++
		}
	}
	need := 2
	if len(a) == 1 || len(b) == 1 {
		need = 1
	}
	if common >= need {
		return NameMatch
	}
	return NameMismatch
}

// nameWords are the distinct words of a name: letters only, upper case,
// at least two letters (initials are ignored).
func nameWords(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToUpper(s), func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }) {
		w = strings.ReplaceAll(w, "'", "")
		if len([]rune(w)) >= 2 {
			out[w] = true
		}
	}
	return out
}
