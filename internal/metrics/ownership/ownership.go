// Package ownership is the ownership metric family (ADR-0024, ADR-0040): where
// knowledge of the code sits, measured over surviving lines.
//
// docs/metrics.md section 7 defines every ownership metric over the line
// ownership that forward replay produces (ADR-0020), and the replay stage
// does not exist yet. The figures this package computed before were counted
// over commits and over sampled blame, which that section does not define,
// so they are not computed (ADR-0062 clauses 1 and 3). Until WP-0023
// implements the family over replay-state, it is present in every report as
// skipped with reason not_implemented (ADR-0032 clause 1).
package ownership

import "github.com/sinanganiz/commitography/internal/core"

// Build returns the ownership family.
func Build() core.Family[core.OwnershipMetrics] {
	return core.Skipped[core.OwnershipMetrics](core.Version{}, core.ReasonNotImplemented)
}
