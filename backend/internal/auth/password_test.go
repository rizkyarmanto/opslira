package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestVerifyPasswordAcceptsLaravelHashes: Laravel stores $2y$ hashes; the
// x/crypto parser only takes $2a$/$2b$, so the marker is normalized first.
func TestVerifyPasswordAcceptsLaravelHashes(t *testing.T) {
	plain := "ChangeMe!2026"

	generated, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	hash := string(generated)
	if hash[:4] == "$2y$" {
		t.Fatalf("test premise: Go produced %q", hash[:4])
	}
	// Rewrite the marker to Laravel's $2y$ form (same algorithm).
	laravel := "$2y$" + hash[4:]

	if !VerifyPassword(laravel, plain) {
		t.Error("$2y$ hash did not verify")
	}
	if !VerifyPassword(hash, plain) {
		t.Error("$2a$ hash did not verify")
	}
	if VerifyPassword(laravel, "wrong-password") {
		t.Error("wrong password verified")
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "s3cret-pass") {
		t.Error("hash did not verify")
	}
}
