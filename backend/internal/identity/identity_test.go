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

func TestFingerprint(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	a := Fingerprint(key, "12345678")
	if len(a) != 32 {
		t.Fatalf("length %d", len(a))
	}
	// Known value: HMAC-SHA256 computed independently of Fingerprint.
	m := hmac.New(sha256.New, key)
	m.Write([]byte("national_id:12345678"))
	if hex.EncodeToString(a) != hex.EncodeToString(m.Sum(nil)) {
		t.Fatal("fingerprint is not HMAC-SHA256 of the prefixed number")
	}
	if bytes.Equal(a, Fingerprint(key, "12345679")) {
		t.Fatal("different numbers, same fingerprint")
	}
	if bytes.Equal(a, Fingerprint(bytes.Repeat([]byte{8}, 32), "12345678")) {
		t.Fatal("different keys, same fingerprint")
	}
	if Last3("12345678") != "678" {
		t.Fatal("last3")
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
