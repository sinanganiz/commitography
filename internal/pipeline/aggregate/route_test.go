package aggregate

import (
	"context"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

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

// routeOne routes a single registration into an empty report, over an input
// holding a commit record and a replay state.
func routeOne(e entry, in core.Input) (core.Families, []string, error) {
	var families core.Families
	warnings, err := route(in, &families, []entry{e})
	return families, warnings, err
}

// fullInput is an input carrying everything aggregation is ever given.
func fullInput() core.Input {
	commits := []model.Commit{authored("Ada", "ada@example.com", 1)}
	return core.Input{
		Context:    context.Background(),
		RepoPath:   "/the/repository",
		Repository: model.RepositoryInfo{Path: "/the/repository", Name: "repository"},
		Filtered:   filter.Summarize(commits),
		Replay:     &core.ReplayState{Tracked: []string{"a.go"}, TextFileCount: 1},
		Progress:   func(string, string, int, int) {},
	}
}

// TestAggregateSkipsAFamilyWhoseInputIsUnavailable is ADR-0076 clause 3: a
// family declaring an input the analysis cannot provide is skipped with the
// matching reason and never run. No external service is ever available.
func TestAggregateSkipsAFamilyWhoseInputIsUnavailable(t *testing.T) {
	t.Parallel()
	ran := false
	e := computedStandIn[core.HotspotMetrics]("hotspot", false, core.InputCommitRecords, core.InputExternalService)
	e.section = computed(func(core.Input) (core.Family[core.HotspotMetrics], []string) {
		ran = true
		return core.Computed(core.Version{Major: 1}, core.HotspotMetrics{}), nil
	})
	families, _, err := routeOne(e, fullInput())
	if err != nil {
		t.Fatalf("routing a family whose input is unavailable: %v", err)
	}
	if ran {
		t.Error("a family whose input is unavailable was run")
	}
	got := families.Hotspot
	if got.Status != core.StatusSkipped || len(got.Reasons) != 1 || got.Reasons[0] != core.ReasonExternalServiceUnavailable {
		t.Errorf("the family is %s %v, want skipped with external_service_unavailable", got.Status, got.Reasons)
	}
	if got.Version != (core.Version{Major: 1}) {
		t.Errorf("the skipped family carries version %s, not the one it declares", got.Version)
	}
}

// TestAggregateGivesAFamilyOnlyItsDeclaredInputs is ADR-0076 clauses 2 and 3:
// a family is run over the inputs it declares and no others, and over no
// repository location, since no input kind is the repository.
func TestAggregateGivesAFamilyOnlyItsDeclaredInputs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		inputs              []core.InputKind
		records, replayable bool
	}{
		{[]core.InputKind{core.InputCommitRecords}, true, false},
		{[]core.InputKind{core.InputReplayState}, false, true},
		{[]core.InputKind{core.InputCommitRecords, core.InputReplayState}, true, true},
	} {
		var given core.Input
		e := computedStandIn[core.FilesMetrics]("files", false, c.inputs...)
		e.section = computed(func(in core.Input) (core.Family[core.FilesMetrics], []string) {
			given = in
			return core.Computed(core.Version{Major: 1}, core.FilesMetrics{}), nil
		})
		if _, _, err := routeOne(e, fullInput()); err != nil {
			t.Fatalf("routing a family declaring %v: %v", c.inputs, err)
		}
		if got := len(given.Analyzed()) > 0; got != c.records {
			t.Errorf("a family declaring %v was given commit records: %t", c.inputs, got)
		}
		if got := given.Replay != nil; got != c.replayable {
			t.Errorf("a family declaring %v was given replay state: %t", c.inputs, got)
		}
		if given.RepoPath != "" || given.Repository.Path != "" || given.Progress != nil {
			t.Errorf("a family declaring %v was given the repository's location or the stage's progress", c.inputs)
		}
	}
}

// TestAggregateRequiresTheReplayStateAFamilyDeclares keeps replay state an
// input the stage is always given: a family declaring it, with none to give,
// is an internal error rather than a skip, since no reason code names it.
func TestAggregateRequiresTheReplayStateAFamilyDeclares(t *testing.T) {
	t.Parallel()
	in := fullInput()
	in.Replay = nil
	_, _, err := routeOne(computedStandIn[core.FilesMetrics]("files", false, core.InputReplayState), in)
	if err == nil || core.ClassOf(err) != core.ClassInternal {
		t.Errorf("a family declaring replay state was routed without one: %v", err)
	}
}

// TestAggregateRefusesAWriteOutsideTheNamespace is ADR-0076 clause 5 on the
// route: a family whose section is not of the type its declared namespace
// holds, two families declaring one namespace, and a namespace the report
// has no family for are each refused, and nothing is written for them.
func TestAggregateRefusesAWriteOutsideTheNamespace(t *testing.T) {
	t.Parallel()
	foreign := computedStandIn[core.HotspotMetrics]("files", false)
	twice := []entry{computedStandIn[core.FilesMetrics]("files", false), computedStandIn[core.FilesMetrics]("files", false)}
	retired := computedStandIn[core.CommitSizeMetrics]("commit-size", false)
	for name, registered := range map[string][]entry{
		"a hotspot section under files": {foreign},
		"two families under files":      twice,
		"a section under commit-size":   {retired},
	} {
		var families core.Families
		if _, err := route(fullInput(), &families, registered); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// TestAggregateRefusesARegistrationItsDeclarationContradicts keeps the
// declaration the source of truth: a registration computing a family that
// declares itself not implemented, one computing nothing for a family that
// declares a computed status, a section carrying another version than the
// declared one, and a registration with no section are each refused.
func TestAggregateRefusesARegistrationItsDeclarationContradicts(t *testing.T) {
	t.Parallel()
	computesTheUnimplemented := notImplementedStandIn[core.WorktypeMetrics]("worktype", true)
	computesTheUnimplemented.section = computedStandIn[core.WorktypeMetrics]("worktype", true).section
	skipsTheComputed := computedStandIn[core.TemporalMetrics]("temporal", false)
	skipsTheComputed.section = notImplemented[core.TemporalMetrics]()
	otherVersion := computedStandIn[core.MessagesMetrics]("messages", false)
	otherVersion.section = computed(func(core.Input) (core.Family[core.MessagesMetrics], []string) {
		return core.Computed(core.Version{Major: 2}, core.MessagesMetrics{}), nil
	})
	noSection := computedStandIn[core.CouplingMetrics]("coupling", false)
	noSection.section = nil
	for name, e := range map[string]entry{
		"a computed not-implemented family": computesTheUnimplemented,
		"a skipped computed family":         skipsTheComputed,
		"a version other than the declared": otherVersion,
		"a registration producing nothing":  noSection,
	} {
		if _, _, err := routeOne(e, fullInput()); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// TestAggregateCarriesTheDeclaredMethod is ADR-0032 clause 8 through the
// declaration: a section carries the method statement its family declares,
// computed or skipped.
func TestAggregateCarriesTheDeclaredMethod(t *testing.T) {
	t.Parallel()
	computedFamily := computedStandIn[core.CommitSizeMetrics]("commit_size", false)
	d := computedFamily.family.Declaration()
	d.Method = "how commit sizes are counted"
	computedFamily.family = standIn(d)
	skippedFamily := notImplementedStandIn[core.OwnershipMetrics]("ownership", true)
	s := skippedFamily.family.Declaration()
	s.Method = "how ownership is derived"
	skippedFamily.family = standIn(s)

	var families core.Families
	if _, err := route(fullInput(), &families, []entry{computedFamily, skippedFamily}); err != nil {
		t.Fatalf("routing: %v", err)
	}
	if families.CommitSize.Method != d.Method || families.Ownership.Method != s.Method {
		t.Errorf("methods = %q and %q, want the declared %q and %q", families.CommitSize.Method,
			families.Ownership.Method, d.Method, s.Method)
	}
}
