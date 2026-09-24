package aggregate

import "github.com/sinanganiz/commitography/internal/core"

// standIn is a family that exists only in a test, declaring what the test
// gives it. A stand-in takes the place of a registered family, so that what
// the route does with a family can be observed where no registered family
// does it yet.
type standIn core.FamilyDeclaration

func (s standIn) Declaration() core.FamilyDeclaration { return core.FamilyDeclaration(s) }

// computedStandIn registers a stand-in that computes an ok section of metric
// type M, and writes it under namespace as the family owning it would.
func computedStandIn[M any](namespace string, identities bool, inputs ...core.InputKind) entry {
	version := core.Version{Major: 1}
	if len(inputs) == 0 {
		inputs = []core.InputKind{core.InputCommitRecords}
	}
	return entry{
		family: standIn{Name: "stand-in " + namespace, Inputs: inputs, Namespace: namespace, Version: version,
			Status: core.StatusFromComputation},
		section: computed(func(core.Input) (core.Family[M], []string) {
			var metrics M
			return core.Computed(version, metrics), nil
		}),
		identities: identities,
	}
}

// notImplementedStandIn registers a stand-in that computes nothing, under
// namespace.
func notImplementedStandIn[M any](namespace string, identities bool) entry {
	return entry{
		family: standIn{Name: "stand-in " + namespace, Inputs: []core.InputKind{core.InputCommitRecords},
			Namespace: namespace, Status: core.StatusNotImplemented},
		section:    notImplemented[M](),
		identities: identities,
	}
}
