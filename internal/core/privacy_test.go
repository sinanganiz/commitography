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

func TestIdentityDigestGivesEveryIdentityAnID(t *testing.T) {
	t.Parallel()
	if IdentityDigest("ada@example.com") != HashEmail("ada@example.com") {
		t.Error("an identity with an address is not identified by the address's digest")
	}
	// The SHA-256 of the empty string, truncated.
	if got := IdentityDigest(""); got != "e3b0c44298fc1c14" {
		t.Errorf("IdentityDigest(\"\") = %q, want the digest of the empty address", got)
	}
}
