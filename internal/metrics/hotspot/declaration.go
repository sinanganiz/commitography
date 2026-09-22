package hotspot

import "github.com/sinanganiz/commitography/internal/core"

// Family is the hotspot family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061).
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name: "hotspot",
		// ADR-0076 clause 6's row. The churn files computed today read commit
		// records alone; the complexity proxy will read line content at the
		// analysed commit from replay state when WP-0026 rebuilds the family.
		Inputs:    []core.InputKind{core.InputCommitRecords, core.InputReplayState},
		Namespace: "hotspot",
		Version:   version(),
		Status:    core.StatusFromComputation,
	}
}
