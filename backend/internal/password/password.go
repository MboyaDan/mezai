// Package password hashes and verifies passwords with Argon2id.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameters from the OWASP Password Storage Cheat Sheet (Argon2id, 19 MiB,
// 2 iterations, 1 lane). Stored in each hash, so they can be raised later.
const (
	memoryKiB = 19 * 1024
	time      = 2
	threads   = 1
	saltLen   = 16
	keyLen    = 32
)

var ErrMalformed = errors.New("password: malformed hash")

var b64 = base64.RawStdEncoding

// Hash returns a PHC string: $argon2id$v=19$m=...,t=...,p=...$salt$key.
func Hash(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, time, memoryKiB, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, time, threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify reports whether plain matches the PHC string in encoded.
func Verify(plain, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, ErrMalformed
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, ErrMalformed
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m == 0 || t == 0 || p == 0 {
		return false, ErrMalformed
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, ErrMalformed
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, ErrMalformed
	}
	got := argon2.IDKey([]byte(plain), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
