package core

import "testing"

func TestHashEmailIsStableAndCaseInsensitive(t *testing.T) {
	t.Parallel()
	a := HashEmail("Ada@Example.COM")
	b := HashEmail("  ada@example.com  ")
	if a != b {
		t.Errorf("HashEmail is not normalizing: %q != %q", a, b)
	}
	if len(a) != emailHashLength {
		t.Errorf("HashEmail returned %q, want a %d-character digest", a, emailHashLength)
	}
	if a == HashEmail("grace@example.com") {
		t.Error("different addresses collided")
	}
	if HashEmail("") != "" {
		t.Error("an empty address should hash to nothing")
	}
}
