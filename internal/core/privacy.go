package core

import "github.com/sinanganiz/commitography/internal/core/identity"

// IdentityDigest returns the identity digest of an address: the id the
// report's identities section carries for an identity whose canonical address
// is given (docs/metrics.md section 14, ADR-0033 clauses 2 and 4). The
// identity layer computes it, because that is where the address is held; this
// is the name the report side uses for it.
func IdentityDigest(address string) string {
	return identity.Digest(address)
}
