// Package identity turns a Kenyan National ID number into what the platform
// stores instead of it: a keyed fingerprint (to find a second account for the
// same person) and the last 3 digits (to show). The number itself is never
// stored or logged (D-078, Kenya DPA 2019 data minimisation).
package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// ErrInvalidNationalID is returned for anything that is not 6 to 9 digits.
var ErrInvalidNationalID = errors.New("identity: not a National ID number")

// MinKeyBytes is the shortest fingerprint key accepted.
const MinKeyBytes = 32

// DevKey is the fingerprint key in development and test only. Config refuses
// to start elsewhere without IDENTITY_HMAC_KEY.
var DevKey = []byte("mezai-development-identity-key-not-secret")

// NormaliseNationalID strips spaces, dashes and dots and leading zeros, and
// accepts 6 to 9 digits (A-050). The same person typing "0123 4567" and
// "1234567" gets the same result.
func NormaliseNationalID(in string) (string, error) {
	s := strings.NewReplacer(" ", "", "-", "", ".", "").Replace(strings.TrimSpace(in))
	for _, r := range s {
		if r < '0' || r > '9' {
			return "", ErrInvalidNationalID
		}
	}
	s = strings.TrimLeft(s, "0")
	if len(s) < 6 || len(s) > 9 {
		return "", ErrInvalidNationalID
	}
	return s, nil
}

// Fingerprint is HMAC-SHA256(key, "national_id:" + normalised). Without the
// key, a fingerprint cannot be matched to a number by trying every ID.
func Fingerprint(key []byte, normalised string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte("national_id:" + normalised))
	return m.Sum(nil)
}

// Last3 returns the last three digits of a normalised number.
func Last3(normalised string) string { return normalised[len(normalised)-3:] }

// ParseKey decodes IDENTITY_HMAC_KEY (standard or URL-safe base64) and checks
// its length.
func ParseKey(b64 string) ([]byte, error) {
	b64 = strings.TrimSpace(b64)
	k, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		if k, err = base64.URLEncoding.DecodeString(b64); err != nil {
			return nil, errors.New("identity: key is not base64")
		}
	}
	if len(k) < MinKeyBytes {
		return nil, errors.New("identity: key must be at least 32 bytes")
	}
	return k, nil
}
