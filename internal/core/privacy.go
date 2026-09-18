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

// HashEmail returns a stable, non-reversible stand-in for an address.
//
// The report document carries no identity data at present: the sections that
// did (the per-contributor section and the named knowledge-concentration
// leader) are not metrics docs/metrics.md defines, so they left the report
// with it (ADR-0062 clause 1). Identity returns to the report with the
// identity model of WP-0009, which uses this digest.
func HashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:emailHashLength]
}
