package password

import (
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected format: %s", h)
	}
	if ok, err := Verify("correct horse battery staple", h); err != nil || !ok {
		t.Fatalf("expected match, got %v %v", ok, err)
	}
	if ok, _ := Verify("wrong", h); ok {
		t.Fatal("wrong password matched")
	}
	h2, _ := Hash("correct horse battery staple")
	if h == h2 {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=0,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=1,t=1,p=1$!!$a2V5"} {
		if _, err := Verify("x", bad); err != ErrMalformed {
			t.Errorf("%q: want ErrMalformed, got %v", bad, err)
		}
	}
}

// Verify must use the parameters stored in the hash, not the current
// constants, so hashes keep working after the parameters are raised.
func TestVerifyHonoursStoredParams(t *testing.T) {
	salt := []byte("somesalt")
	key := argon2.IDKey([]byte("password"), salt, 1, 64, 1, 16)
	h := "$argon2id$v=19$m=64,t=1,p=1$" + b64.EncodeToString(salt) + "$" + b64.EncodeToString(key)
	if ok, err := Verify("password", h); err != nil || !ok {
		t.Fatalf("verify with stored params failed: %v %v", ok, err)
	}
}
