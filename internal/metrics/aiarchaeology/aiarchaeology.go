// Package aiarchaeology is the ai-archaeology metric family (ADR-0076,
// ADR-0040): the traces assisted authorship leaves in the history. Its metrics
// are those of docs/metrics.md section 9 (ADR-0062).
//
// It has no implementation yet and computes nothing. It declares its contract
// only, with not_implemented as its status source, so the family is present
// in every report as skipped (ADR-0032 clause 1) until WP-0025 implements it.
package aiarchaeology

import "github.com/sinanganiz/commitography/internal/core"

// Family is the ai-archaeology family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061). The family
// computes nothing, so it carries the zero version.
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:   "ai-archaeology",
		Inputs: []core.InputKind{core.InputCommitRecords, core.InputReplayState},
		// The catalogue's namespace (ADR-0062 clause 3). The report still
		// writes the family under ai-archaeology until WP-0061 renames the key.
		Namespace: "ai_archaeology",
		Version:   core.Version{},
		Status:    core.StatusNotImplemented,
	}
}
