package commitsize

import "github.com/sinanganiz/commitography/internal/core"

// method states how effective lines are counted (docs/metrics.md section 1),
// where that differs from counting a file's lines (ADR-0032 clause 8). The
// pipeline root writes the same text into the report until the aggregate
// stage reads it from the declaration (WP-0061).
const method = "Effective lines are the lines git's diff adds and removes in each file that is not " +
	"excluded. A file git detects as binary, by a NUL byte within its first 8 000 bytes unless a repository " +
	"attribute says otherwise, contributes none. Rename detection is on, so a renamed file contributes the " +
	"lines its content changed, and a file moved without change contributes none."

// Family is the commit-size family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061).
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:   "commit-size",
		Inputs: []core.InputKind{core.InputCommitRecords},
		// The catalogue's namespace (ADR-0062 clause 3). The report still
		// writes the family under commit-size until WP-0061 renames the key.
		Namespace: "commit_size",
		Version:   version(),
		Method:    method,
		Status:    core.StatusFromComputation,
	}
}
