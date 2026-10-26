package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost mirrors config/hashing.php: hashing.bcrypt.rounds defaults to 12.
const bcryptCost = 12

// HashPassword bcrypt-hashes a plaintext password for the password_hash
// column. The hash Go writes carries the $2a$ marker, which PHP's
// password_verify accepts alongside $2y$; Laravel-written $2y$ hashes verify
// through VerifyPassword below.
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)

	return string(hash), err
}

// VerifyPassword checks a plaintext password against a stored hash.
//
// Laravel emits the $2y$ bcrypt variant marker, which x/crypto/bcrypt does not
// accept; the marker only documents the PHP crypt() bugfix generation and the
// algorithm is identical, so it is normalized to $2a$ first. $2a$, $2b$ and
// $2y$ hashes therefore all verify here, and hashes written by this package
// verify in PHP.
func VerifyPassword(hash, plaintext string) bool {
	if len(hash) > 3 && hash[:4] == "$2y$" {
		hash = "$2a$" + hash[4:]
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}
