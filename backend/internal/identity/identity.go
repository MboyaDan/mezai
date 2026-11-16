// Package identity turns a Kenyan National ID number into what the platform
// stores instead of it: a keyed fingerprint of the whole number (the only
// thing used to find a second account for the same person) and the last 3
// digits (shown so the person and the interviewer can confirm which number is
// on file; many people share them, so they never identify anyone). The number
// itself is never stored or logged (D-078, Kenya DPA 2019 data minimisation).
package identity

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
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

// Version is the fingerprint scheme new numbers are stored with.
//
//	1  HMAC-SHA256(key, "national_id:" + number)
//	2  Argon2id over the version 1 value, with a salt derived from the key
//
// A National ID has at most nine digits, so anyone holding both the database
// and the key could try every number. Version 1 lets them do that in
// seconds; version 2 makes each try cost memory and time (about a CPU-month
// for the whole range, D-136). Without the key neither can be tried at all.
const Version = 2

// Argon2id cost for version 2 (OWASP's minimum profile: 19 MiB, 2 passes).
const (
	argonTime    = 2
	argonMemory  = 19 * 1024
	argonThreads = 1
)

// Prints are the fingerprints of one number: Current is what to store (with
// Version); All also holds older schemes, so a number saved under one still
// matches.
type Prints struct {
	Current []byte
	All     [][]byte
}

// Matches reports whether a stored fingerprint belongs to the same number.
func (p Prints) Matches(stored []byte) bool {
	for _, f := range p.All {
		if hmac.Equal(f, stored) {
			return true
		}
	}
	return false
}

// Fingerprints returns the fingerprints of a normalised number.
func Fingerprints(key []byte, normalised string) Prints {
	v1 := fingerprintV1(key, normalised)
	m := hmac.New(sha256.New, key)
	m.Write([]byte("national_id:salt:v2"))
	v2 := argon2.IDKey(v1, m.Sum(nil)[:16], argonTime, argonMemory, argonThreads, 32)
	return Prints{Current: v2, All: [][]byte{v2, v1}}
}

func fingerprintV1(key []byte, normalised string) []byte {
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
