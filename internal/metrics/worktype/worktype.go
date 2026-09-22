// Package worktype is the worktype metric family (ADR-0076, ADR-0040): how the
// analysed commits' line-level changes divide into new work, rework, help
// others and legacy refactor (ADR-0020 clause 4). Its metrics are those of
// docs/metrics.md section 8 (ADR-0062).
//
// It has no implementation yet and computes nothing. It declares its contract
// only, with not_implemented as its status source, so the family is present
// in every report as skipped (ADR-0032 clause 1) until WP-0024 implements it.
package worktype

import "github.com/sinanganiz/commitography/internal/core"

// Family is the worktype family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061). The family
// computes nothing, so it carries the zero version.
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:      "worktype",
		Inputs:    []core.InputKind{core.InputReplayState},
		Namespace: "worktype",
		Version:   core.Version{},
		Status:    core.StatusNotImplemented,
	}
}
