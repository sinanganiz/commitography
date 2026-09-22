package hotspot

import "github.com/sinanganiz/commitography/internal/core"

// Family is the hotspot family's contract (ADR-0024 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061).
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name: "hotspot",
		// ADR-0024 clause 5's row. The churn files computed today read commit
		// records alone; whether worktree stays a distinct input is decided
		// when the family is rebuilt (ADR-0075 clause 3, WP-0026).
		Inputs:    []core.InputKind{core.InputWorktree, core.InputCommitRecords},
		Namespace: "hotspot",
		Version:   version(),
		Status:    core.StatusFromComputation,
	}
}
