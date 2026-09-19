package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// digestLength is how much of the SHA-256 digest is kept: sixteen hexadecimal
// characters (docs/metrics.md section 14).
const digestLength = 16

// Unserialisable is the error every marshalling method on Address returns. An
// Address that reaches an encoder fails the encoding rather than writing the
// address, or silently writing nothing.
type Unserialisable struct{}

func (Unserialisable) Error() string {
	return "a raw address has no serialised form (ADR-0033 clause 3)"
}

// Address is a raw email address, trimmed and lowercased. It belongs to the
// internal working layer (ADR-0033 clause 1) and has no path out of it: its
// one field is unexported, every encoder it could reach refuses it, and fmt
// prints a placeholder in its place (ADR-0033 clauses 2 and 3).
//
// TestIdentityRawAddressUnserialisable in internal/checks fails if this type,
// or any field in this package holding an address, gains a marshalling path.
type Address struct {
	value string
}

// ParseAddress normalises an address the way the digest requires: trimmed and
// lowercased, so casing never splits one person into two identities.
func ParseAddress(raw string) Address {
	return Address{value: strings.ToLower(strings.TrimSpace(raw))}
}

// Empty reports whether the commit carried no address at all.
func (a Address) Empty() bool { return a.value == "" }

// Digest returns the address's stable digest (ADR-0033 clause 4).
func (a Address) Digest() string { return Digest(a.value) }

// localPart returns everything before the last @, or nothing where there is no
// @ to split on.
func (a Address) localPart() string {
	at := strings.LastIndex(a.value, "@")
	if at <= 0 {
		return ""
	}
	return a.value[:at]
}

// Format prints a placeholder under every verb, so that a raw address placed
// in a log line or an error message by accident shows as redacted rather than
// as itself.
func (a Address) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, "[address]")
}

// MarshalJSON refuses: an address has no JSON form.
func (a Address) MarshalJSON() ([]byte, error) { return nil, Unserialisable{} }

// MarshalText refuses, which also covers every encoder that falls back to text,
// such as a JSON map key or a YAML value.
func (a Address) MarshalText() ([]byte, error) { return nil, Unserialisable{} }

// MarshalBinary refuses.
func (a Address) MarshalBinary() ([]byte, error) { return nil, Unserialisable{} }

// GobEncode refuses.
func (a Address) GobEncode() ([]byte, error) { return nil, Unserialisable{} }

// Digest returns the stable digest of an address: the first 16 hexadecimal
// characters of the SHA-256 of the address after trimming and lowercasing
// (docs/metrics.md section 14). It depends on nothing but the address, so the
// same address has the same digest in every repository, which is what lets
// identities be matched across repositories without the address leaving the
// internal layer (ADR-0033 clause 4).
//
// An empty address has a digest too, of the empty string, so that an identity
// whose commits carry no address still has an id.
func Digest(address string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(address))))
	return hex.EncodeToString(sum[:])[:digestLength]
}
