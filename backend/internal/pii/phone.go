// Package pii normalises and masks personal data that must not appear in
// full in logs or lists (CLAUDE.md section 2, Kenya DPA 2019).
package pii

import (
	"errors"
	"strings"
)

// ErrInvalidPhone is returned for anything that is not a Kenyan mobile number.
var ErrInvalidPhone = errors.New("pii: not a Kenyan mobile number")

// NormaliseKenyanMobile accepts 07XXXXXXXX, 01XXXXXXXX, 2547XXXXXXXX or
// +2547XXXXXXXX (spaces and dashes ignored) and returns E.164 (+2547XXXXXXXX),
// the form contributor_profiles.mpesa_phone stores.
func NormaliseKenyanMobile(in string) (string, error) {
	s := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(in))
	switch {
	case strings.HasPrefix(s, "+254"):
		s = s[4:]
	case strings.HasPrefix(s, "254"):
		s = s[3:]
	case strings.HasPrefix(s, "0"):
		s = s[1:]
	default:
		return "", ErrInvalidPhone
	}
	if len(s) != 9 || (s[0] != '7' && s[0] != '1') {
		return "", ErrInvalidPhone
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", ErrInvalidPhone
		}
	}
	return "+254" + s, nil
}

// MaskPhone keeps the country code and the last three digits:
// +254712345678 → "+254 7•• ••• 678". Empty stays empty.
func MaskPhone(e164 string) string {
	if e164 == "" {
		return ""
	}
	if len(e164) != 13 || !strings.HasPrefix(e164, "+254") {
		return "•••"
	}
	return "+254 " + e164[4:5] + "•• ••• " + e164[10:]
}
