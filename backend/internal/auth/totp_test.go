package auth

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1 rows (8 digits; the 6-digit code is the last six).
func TestTOTPRFCVectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, c := range []struct {
		unix int64
		want string
	}{
		{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"},
		{1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"},
	} {
		if got := totpCode(secret, c.unix/30, 8); got != c.want {
			t.Errorf("T=%d: got %s, want %s", c.unix, got, c.want)
		}
		if got := totpCode(secret, c.unix/30, 6); got != c.want[2:] {
			t.Errorf("T=%d 6 digits: got %s, want %s", c.unix, got, c.want[2:])
		}
	}
}

func TestVerifyTOTP(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111111, 0)
	cur := totpStep(now)
	code := totpCode(secret, cur, 6)
	if s, ok := verifyTOTP(secret, code, now, 0); !ok || s != cur {
		t.Fatal("current code refused")
	}
	if _, ok := verifyTOTP(secret, code[:3]+" "+code[3:], now, 0); !ok {
		t.Fatal("code with a space refused")
	}
	if _, ok := verifyTOTP(secret, totpCode(secret, cur-1, 6), now, 0); !ok {
		t.Fatal("previous step refused (clock skew)")
	}
	if _, ok := verifyTOTP(secret, totpCode(secret, cur-2, 6), now, 0); ok {
		t.Fatal("code two steps old accepted")
	}
	if _, ok := verifyTOTP(secret, code, now, cur); ok {
		t.Fatal("code reused in its own step")
	}
	if _, ok := verifyTOTP(secret, "000000x", now, 0); ok {
		t.Fatal("malformed code accepted")
	}
}

func TestSealer(t *testing.T) {
	s, err := newSealer(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	secret := newTOTPSecret()
	box := s.seal(secret)
	if bytes.Contains(box, secret) {
		t.Fatal("secret stored in clear")
	}
	got, err := s.open(box)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("open: %v", err)
	}
	other, _ := newSealer(bytes.Repeat([]byte{8}, 32))
	if _, err := other.open(box); err == nil {
		t.Fatal("another key opened the secret")
	}
	if _, err := newSealer([]byte("short")); err == nil {
		t.Fatal("short key accepted")
	}
}

func TestRecoveryCodes(t *testing.T) {
	codes := newRecoveryCodes()
	seen := map[string]bool{}
	for _, c := range codes {
		n, ok := normRecovery(strings.ToUpper(c))
		if !ok || len(c) != 9 || c[4] != '-' || seen[n] {
			t.Fatalf("bad code %q", c)
		}
		seen[n] = true
	}
	if len(codes) != 10 {
		t.Fatalf("%d codes", len(codes))
	}
	if _, ok := normRecovery("123456"); ok {
		t.Fatal("a TOTP code read as a recovery code")
	}
}

func TestTOTPURI(t *testing.T) {
	u := totpURI("ops@sample.invalid", []byte("12345678901234567890"))
	for _, want := range []string{"otpauth://totp/MezAI:ops@sample.invalid?", "issuer=MezAI", "secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "digits=6", "period=30"} {
		if !strings.Contains(u, want) {
			t.Errorf("uri %s lacks %s", u, want)
		}
	}
}
