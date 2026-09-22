package coupling

import "github.com/sinanganiz/commitography/internal/core"

// Family is the coupling family's contract (ADR-0024 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061).
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:      "coupling",
		Inputs:    []core.InputKind{core.InputCommitRecords},
		Namespace: "coupling",
		Version:   version(),
		Status:    core.StatusFromComputation,
	}
}
