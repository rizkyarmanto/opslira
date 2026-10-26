// Package uuid generates and validates RFC 4122 v4 identifiers — the same
// IDs Laravel's HasUuid trait wrote, without an external dependency.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// New returns a random v4 UUID string (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx).
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("uuid: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant

	dst := make([]byte, 36)
	encode(dst, b)

	return string(dst)
}

// Valid reports whether s looks like a canonical UUID string; used to reject
// malformed {id} path segments before they reach a uuid column.
func Valid(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}

	return true
}

func encode(dst []byte, src [16]byte) {
	hex.Encode(dst[0:8], src[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], src[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], src[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], src[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], src[10:16])
}
