package identity

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNormaliseNationalID(t *testing.T) {
	ok := map[string]string{
		"12345678":    "12345678",
		" 1234 5678 ": "12345678",
		"1234-5678":   "12345678",
		"012345678":   "12345678",
		"123456":      "123456",
		"123456789":   "123456789",
	}
	for in, want := range ok {
		if got, err := NormaliseNationalID(in); err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "12345", "1234567890", "A1234567", "12 34 5x", "000000", "+254712345678"} {
		if got, err := NormaliseNationalID(in); err == nil {
			t.Errorf("%q: accepted as %q", in, got)
		}
	}
}

func TestFingerprints(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	p := Fingerprints(key, "12345678")
	if len(p.Current) != 32 || len(p.All) != 2 || !bytes.Equal(p.All[0], p.Current) {
		t.Fatalf("prints: %d bytes, %d schemes", len(p.Current), len(p.All))
	}
	// Version 1 is kept for numbers saved before version 2: HMAC-SHA256
	// computed here independently.
	m := hmac.New(sha256.New, key)
	m.Write([]byte("national_id:12345678"))
	v1 := m.Sum(nil)
	if hex.EncodeToString(p.All[1]) != hex.EncodeToString(v1) {
		t.Fatal("version 1 is not HMAC-SHA256 of the prefixed number")
	}
	if bytes.Equal(p.Current, v1) {
		t.Fatal("version 2 must not be the plain HMAC")
	}
	if !p.Matches(v1) || !p.Matches(p.Current) {
		t.Fatal("a number must match its fingerprint under either scheme")
	}
	// Deterministic, and different for another number or another key.
	if !bytes.Equal(p.Current, Fingerprints(key, "12345678").Current) {
		t.Fatal("not deterministic")
	}
	other := Fingerprints(key, "12345679")
	if bytes.Equal(p.Current, other.Current) || other.Matches(p.Current) || other.Matches(v1) {
		t.Fatal("different numbers match")
	}
	if Fingerprints(bytes.Repeat([]byte{8}, 32), "12345678").Matches(p.Current) {
		t.Fatal("different keys match")
	}
	if Last3("12345678") != "678" {
		t.Fatal("last3")
	}
}

// The last three digits are for display only: two different numbers that
// share them are different people to the platform.
func TestLast3NeverIdentifies(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	a, b := "12345678", "87654678"
	if Last3(a) != Last3(b) {
		t.Fatal("test numbers must share their last three digits")
	}
	if Fingerprints(key, a).Matches(Fingerprints(key, b).Current) {
		t.Fatal("numbers sharing the last three digits must not match")
	}
}

func TestParseKey(t *testing.T) {
	if _, err := ParseKey("c2hvcnQ="); err == nil {
		t.Fatal("short key accepted")
	}
	if _, err := ParseKey("not base64!"); err == nil {
		t.Fatal("garbage accepted")
	}
	k, err := ParseKey("AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=")
	if err != nil || len(k) != 32 || k[31] != 31 {
		t.Fatalf("valid key: %v %v", k, err)
	}
}
