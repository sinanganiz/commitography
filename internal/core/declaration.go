// The metric family contract (ADR-0076 clause 1): what every family declares
// about itself, through one interface, before anything runs it.
//
// A declaration is a shape, not a composition. Assembling the ten families
// needs imports only the pipeline may make (ADR-0040), so the registry that
// holds them, and the routing of each family's output through its declaration,
// belong to the aggregate stage (WP-0061). Until then nothing reads a
// declaration but the family declaration checker, and the report is written as
// it was.

package core

// InputKind is one of the inputs a family may require (ADR-0076 clause 1).
type InputKind string

// The three input kinds. The working tree is not one: no family reads it, and
// file content at the analysed commit reaches a family through replay state
// (ADR-0076 clause 2).
const (
	InputCommitRecords   InputKind = "commit-records"
	InputReplayState     InputKind = "replay-state"
	InputExternalService InputKind = "external-service"
)

// InputKinds returns the three input kinds, in the order ADR-0076 clause 1
// lists them.
func InputKinds() []InputKind {
	return []InputKind{InputCommitRecords, InputReplayState, InputExternalService}
}

// StatusSource is where a family's status comes from.
type StatusSource string

const (
	// StatusFromComputation is a family that computes its values and sets its
	// own status from them.
	StatusFromComputation StatusSource = "computed"
	// StatusNotImplemented is a family that computes nothing yet. It is
	// present in every report as skipped with reason not_implemented
	// (ADR-0032 clause 1), and carries the zero version.
	StatusNotImplemented StatusSource = StatusSource(ReasonNotImplemented)
)

// FamilyDeclaration is what a family states about itself.
type FamilyDeclaration struct {
	// Name is the family's name in ADR-0076 clause 6.
	Name string
	// Inputs are the inputs the family requires, from the three kinds.
	Inputs []InputKind
	// Namespace is the report namespace the family owns, as docs/metrics.md
	// gives it (ADR-0076 clauses 1 and 5, ADR-0062 clause 3).
	Namespace string
	// Version is the family version (ADR-0031 clause 2). A family that
	// computes nothing carries the zero version, which is what the report
	// holds for it; a family that computes carries a version above it.
	Version Version
	// Method states how the family's values are derived, where that differs
	// from a well-known reference or from a measurement (ADR-0032 clause 8).
	// It is empty for a family that has no such statement.
	Method string
	// Status is where the family's status comes from.
	Status StatusSource
}

// MetricFamily is the interface every metric family implements
// (ADR-0076 clause 1).
type MetricFamily interface {
	// Declaration returns what the family declares about itself.
	Declaration() FamilyDeclaration
}
