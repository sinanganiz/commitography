// Package staticanalysis is the static-analysis metric family (ADR-0076,
// ADR-0040): a non-priority family (ADR-0012 clauses 2 and 3) that reads file
// content at the analysed commit through replay state, never the working tree
// (ADR-0076 clause 2). Its metrics are those of docs/metrics.md section 11
// (ADR-0062).
//
// It has no implementation yet and computes nothing. It declares its contract
// only, with not_implemented as its status source, so the family is present
// in every report as skipped (ADR-0032 clause 1) until WP-0027 implements it.
package staticanalysis

import "github.com/sinanganiz/commitography/internal/core"

// Family is the static-analysis family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061). The family
// computes nothing, so it carries the zero version.
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:   "static-analysis",
		Inputs: []core.InputKind{core.InputReplayState},
		// The catalogue's namespace (ADR-0062 clause 3). The report still
		// writes the family under static-analysis until WP-0061 renames the key.
		Namespace: "static_analysis",
		Version:   core.Version{},
		Status:    core.StatusNotImplemented,
	}
}
