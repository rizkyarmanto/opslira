package uuid

import "testing"

func TestNewIsValidV4(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := New()
		if !Valid(id) {
			t.Fatalf("New() = %q, not a valid UUID", id)
		}
		if id[14] != '4' {
			t.Errorf("version nibble wrong in %q", id)
		}
		switch id[19] {
		case '8', '9', 'a', 'b':
		default:
			t.Errorf("variant nibble wrong in %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestValidRejectsGarbage(t *testing.T) {
	for _, value := range []string{
		"", "not-a-uuid", "zzzzzzzz-ffff-4fff-8fff-ffffffffffff",
		"123e4567-e89b-12d3-a456-426614174000", // v1: wrong version nibble? no — Valid only checks shape
		"123e4567-e89b-12d3-a456-42661417400",
		"123e4567e89b12d3a456426614174000",
	} {
		if value == "123e4567-e89b-12d3-a456-426614174000" {
			// Shape-only validator: a v1 UUID is still structurally valid.
			if !Valid(value) {
				t.Errorf("v1-shaped uuid rejected: %q", value)
			}
			continue
		}
		if Valid(value) {
			t.Errorf("Valid(%q) = true, want false", value)
		}
	}
}
