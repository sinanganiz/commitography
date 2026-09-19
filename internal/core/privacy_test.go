package core

import "testing"

func TestIdentityDigestIsStableAndCaseInsensitive(t *testing.T) {
	t.Parallel()
	a := IdentityDigest("Ada@Example.COM")
	b := IdentityDigest("  ada@example.com  ")
	if a != b {
		t.Errorf("IdentityDigest is not normalizing: %q != %q", a, b)
	}
	if len(a) != 16 {
		t.Errorf("IdentityDigest returned %q, want a 16-character digest", a)
	}
	if a == IdentityDigest("grace@example.com") {
		t.Error("different addresses collided")
	}
}

func TestIdentityDigestGivesEveryIdentityAnID(t *testing.T) {
	t.Parallel()
	// The SHA-256 of the empty string, truncated.
	if got := IdentityDigest(""); got != "e3b0c44298fc1c14" {
		t.Errorf("IdentityDigest(\"\") = %q, want the digest of the empty address", got)
	}
}
