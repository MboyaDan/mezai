package identity

import "testing"

func TestCompareNames(t *testing.T) {
	cases := []struct{ profile, registered, want string }{
		{"Jane Wanjiru Doe", "254712345678 - JANE WANJIRU DOE", NameMatch},
		{"Jane Doe", "254712345678 - JANE WANJIRU DOE", NameMatch},          // a middle name left out
		{"Doe Jane Wanjiru", "254712345678 - JANE WANJIRU DOE", NameMatch},  // another order
		{"jane  w. doe", "JANE WANJIRU DOE", NameMatch},                     // initials ignored
		{"Jane O'Connor-Doe", "254700000000 - JANE OCONNOR DOE", NameMatch}, // punctuation
		{"Wanjiru", "254712345678 - JANE WANJIRU DOE", NameMatch},           // one word on the profile
		{"Jane Wanjiru Doe", "254712345678 - WANJIRU", NameMatch},           // one word at M-Pesa
		{"Jane Doe", "254712345678 - JOHN DOE", NameMismatch},               // a family member's line
		{"Jane Wanjiru Doe", "254712345678 - PETER OTIENO", NameMismatch},
		{"Jane Doe", "254712345678 - ", NameUnavailable},
		{"Jane Doe", "", NameUnavailable},
		{"Jane Doe", "254712345678", NameUnavailable}, // a number only
		{"", "254712345678 - JANE DOE", NameMismatch},
	}
	for _, c := range cases {
		if got := CompareNames(c.profile, c.registered); got != c.want {
			t.Errorf("CompareNames(%q, %q) = %s, want %s", c.profile, c.registered, got, c.want)
		}
	}
}
