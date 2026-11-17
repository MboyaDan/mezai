package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/MboyaDan/mezai/backend/internal/brand"
)

// Time-based one-time codes (RFC 6238 with the defaults every authenticator
// app uses: HMAC-SHA1, 30-second steps, 6 digits).
const (
	totpPeriod = 30
	totpDigits = 6
	// totpSkew accepts the step before and after the current one, so a phone
	// clock a little off still works.
	totpSkew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// totpCode is the code for one time step.
func totpCode(secret []byte, step int64, digits int) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	mod := uint32(1)
	for range digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, bin%mod)
}

func totpStep(t time.Time) int64 { return t.Unix() / totpPeriod }

// verifyTOTP returns the step a code matches, if it is within the skew and
// later than the last step used (a code works once).
func verifyTOTP(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.TrimSpace(strings.ReplaceAll(code, " ", ""))
	if len(code) != totpDigits {
		return 0, false
	}
	cur := totpStep(now)
	for s := cur - totpSkew; s <= cur+totpSkew; s++ {
		if s <= lastStep {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(totpCode(secret, s, totpDigits)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}

// newTOTPSecret returns 20 random bytes (the RFC's recommended length).
func newTOTPSecret() []byte {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return b
}

// totpURI is the otpauth link an authenticator app reads from the QR code.
func totpURI(email string, secret []byte) string {
	label := url.PathEscape(brand.Name + ":" + email)
	v := url.Values{"secret": {b32.EncodeToString(secret)}, "issuer": {brand.Name},
		"algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// sealer encrypts TOTP secrets at rest with a key derived from the
// identity key, so a database dump alone cannot produce codes.
type sealer struct{ aead cipher.AEAD }

func newSealer(key []byte) (*sealer, error) {
	if len(key) < 32 {
		return nil, errors.New("auth: TOTP key must be at least 32 bytes")
	}
	k := sha256.Sum256(append([]byte("mezai totp secret v1\x00"), key...))
	block, err := aes.NewCipher(k[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &sealer{aead: aead}, nil
}

func (s *sealer) seal(plain []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	_, _ = rand.Read(nonce)
	return s.aead.Seal(nonce, nonce, plain, []byte("totp"))
}

func (s *sealer) open(box []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(box) < n {
		return nil, errors.New("auth: sealed secret too short")
	}
	return s.aead.Open(nil, box[:n], box[n:], []byte("totp"))
}

// Recovery codes: ten one-time codes such as "k7pq-3mzd", from an alphabet
// without look-alike characters. Stored as SHA-256 hashes (40 bits of
// randomness each, used at most once, behind the sign-in limits).
const (
	recoveryCount    = 10
	recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"
)

func newRecoveryCodes() []string {
	out := make([]string, recoveryCount)
	for i := range out {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		var sb strings.Builder
		for j, x := range b {
			if j == 4 {
				sb.WriteByte('-')
			}
			sb.WriteByte(recoveryAlphabet[int(x)%len(recoveryAlphabet)])
		}
		out[i] = sb.String()
	}
	return out
}

// normRecovery lower-cases a typed recovery code and drops spaces and
// dashes; ok is false when it cannot be one.
func normRecovery(code string) (string, bool) {
	c := strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	if len(c) != 8 {
		return "", false
	}
	for _, r := range c {
		if !strings.ContainsRune(recoveryAlphabet, r) {
			return "", false
		}
	}
	return c, true
}

func hashRecovery(norm string) []byte {
	h := sha256.Sum256([]byte("mezai recovery v1\x00" + norm))
	return h[:]
}
