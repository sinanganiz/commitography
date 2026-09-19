package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// emailHashLength is how much of the SHA-256 digest is kept. Sixteen hex
// characters is enough to keep identities distinct in any real repository
// while producing a value nobody can mail.
const emailHashLength = 16

// HashEmail returns a stable, non-reversible stand-in for an address. It is
// the identity digest the report's identities section carries as an entry's
// id (docs/metrics.md section 14, ADR-0033 clauses 2 and 4).
func HashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	return digest(normalized)
}

// IdentityDigest returns the id of an identity whose canonical address is
// given. It is HashEmail, except that an identity whose commits carry no
// address still receives the digest section 14 defines, of the empty
// address, rather than no id at all.
func IdentityDigest(canonical string) string {
	if id := HashEmail(canonical); id != "" {
		return id
	}
	return digest("")
}

func digest(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:emailHashLength]
}
