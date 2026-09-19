package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDigestIsTheTruncatedSHA256OfTheNormalisedAddress(t *testing.T) {
	t.Parallel()
	// printf 'ada@example.com' | sha256sum, first 16 characters.
	const want = "b5fc85e55755f9e0"
	got := Digest("  Ada@Example.COM ")
	if got != want {
		t.Fatalf("Digest = %q, want %q", got, want)
	}
	if got != Digest("ada@example.com") || got != ParseAddress("ADA@example.com").Digest() {
		t.Errorf("Digest does not normalise: %q", got)
	}
	if got == Digest("grace@example.com") {
		t.Error("different addresses share a digest")
	}
}

func TestAddressHasNoSerialisedForm(t *testing.T) {
	t.Parallel()
	a := ParseAddress("someone@example.com")
	for _, value := range []any{a, struct{ A Address }{a}, map[Address]int{a: 1}} {
		if out, err := json.Marshal(value); err == nil || !errors.As(err, new(Unserialisable)) {
			t.Errorf("json.Marshal(%T) = %s, %v; want the Unserialisable error", value, out, err)
		}
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if out := fmt.Sprintf(verb, a); strings.Contains(out, "someone") {
			t.Errorf("fmt %s prints the address: %s", verb, out)
		}
	}
}

func TestAddressLocalPart(t *testing.T) {
	t.Parallel()
	a := ParseAddress("First.Last+tag@Mail.Example.com")
	if a.localPart() != "first.last+tag" {
		t.Errorf("local part = %q", a.localPart())
	}
	if b := ParseAddress("no-at-sign"); b.localPart() != "" {
		t.Errorf("an address without @ has the local part %q", b.localPart())
	}
}
