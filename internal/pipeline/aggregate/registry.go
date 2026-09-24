// The family registry (ADR-0076 clauses 1, 3, 5 and 6): the one list of the
// metric families the report carries, in the order the report writes them,
// and the route by which each family's output reaches the report. The family
// declaration checker in internal/checks reads it rather than holding a list
// of its own, so no second list exists to disagree with this one.

package aggregate

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/metrics/aiarchaeology"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/coupling"
	"github.com/sinanganiz/commitography/internal/metrics/files"
	"github.com/sinanganiz/commitography/internal/metrics/hotspot"
	"github.com/sinanganiz/commitography/internal/metrics/messages"
	"github.com/sinanganiz/commitography/internal/metrics/ownership"
	"github.com/sinanganiz/commitography/internal/metrics/staticanalysis"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
	"github.com/sinanganiz/commitography/internal/metrics/worktype"
)

// entry is one family's registration.
type entry struct {
	family core.MetricFamily
	// section produces the family's section.
	section sectionFunc
	// identities marks a family whose values are attributed to identities:
	// ownership's lines by identity and bus factor, worktype's editor-by-owner
	// breakdown, and ai-archaeology's identity ratio (docs/metrics.md
	// sections 7 to 9). An author that cannot be resolved into an identity
	// degrades such a family with unresolved_identity: the author's commits
	// are counted, so values may be attributed wrongly, which is low
	// confidence (ADR-0032 clause 2, docs/metrics.md section 13). A skipped
	// family stays skipped, having computed nothing to distrust.
	identities bool
	// progress is the progress detail reported as the family is started, for
	// the families that begin a progress stage.
	progress string
}

// entries returns every registration, in the order the report writes the
// families (ADR-0076 clause 6).
func entries() []entry {
	return []entry{
		{
			family: temporal.Family{},
			section: computed(func(in core.Input) (core.Family[core.TemporalMetrics], []string) {
				return temporal.Build(in, in.Analyzed()), nil
			}),
			progress: "temporal",
		},
		{
			family: commitsize.Family{},
			section: computed(func(in core.Input) (core.Family[core.CommitSizeMetrics], []string) {
				return commitsize.Build(in, in.LineScoped()), nil
			}),
			progress: "code",
		},
		{
			family: messages.Family{},
			section: computed(func(in core.Input) (core.Family[core.MessagesMetrics], []string) {
				return messages.Build(in.Analyzed()), nil
			}),
			progress: "messages",
		},
		{
			family: files.Family{},
			// The files family reads the analysed commit's tree as replay
			// listed it; this stage lists no tree and reads no file
			// (ADR-0020 clause 3).
			section: computed(func(in core.Input) (core.Family[core.FilesMetrics], []string) {
				tree := files.Tree{Tracked: in.Replay.Tracked, TextFileCount: in.Replay.TextFileCount}
				return files.Build(in, in.LineScoped(), tree), nil
			}),
			progress: "code",
		},
		{
			family: coupling.Family{},
			section: computed(func(in core.Input) (core.Family[core.CouplingMetrics], []string) {
				return coupling.Build(core.ScopedCommits(in, in.LineScoped()))
			}),
			progress: "social",
		},
		{family: ownership.Family{}, section: notImplemented[core.OwnershipMetrics](), identities: true},
		{family: worktype.Family{}, section: notImplemented[core.WorktypeMetrics](), identities: true},
		{family: aiarchaeology.Family{}, section: notImplemented[core.AIArchaeologyMetrics](), identities: true},
		{
			family: hotspot.Family{},
			section: computed(func(in core.Input) (core.Family[core.HotspotMetrics], []string) {
				return hotspot.Build(core.ScopedCommits(in, in.LineScoped())), nil
			}),
		},
		{family: staticanalysis.Family{}, section: notImplemented[core.StaticAnalysisMetrics]()},
	}
}

// Registry returns every metric family the report carries, in the order the
// report writes them. It is the only list of families in the tree.
func Registry() []core.MetricFamily {
	registered := entries()
	out := make([]core.MetricFamily, 0, len(registered))
	for _, e := range registered {
		out = append(out, e.family)
	}
	return out
}

// sectionFunc produces one family's section from the input resolved for it:
// skipped for the reasons given, where there are any, and computed otherwise.
// It returns the warnings the family raised beside it.
type sectionFunc func(in core.Input, declared core.FamilyDeclaration, skip []core.Reason) (section, []string, error)

// computed is the section of a family whose status comes from its
// computation: run over the family's resolved input unless the family is
// skipped, in which case it is never run (ADR-0076 clause 3). It carries the
// method statement the family declares.
func computed[M any](run func(core.Input) (core.Family[M], []string)) sectionFunc {
	return func(in core.Input, declared core.FamilyDeclaration, skip []core.Reason) (section, []string, error) {
		if declared.Status != core.StatusFromComputation {
			return nil, nil, core.Internalf(nil, "the family %s declares the status source %s, and its "+
				"registration computes it", declared.Name, declared.Status)
		}
		if len(skip) > 0 {
			return declaredSection(core.Skipped[M](declared.Version, skip[0], skip[1:]...), declared), nil, nil
		}
		family, warnings := run(in)
		if family.Version != declared.Version {
			return nil, nil, core.Internalf(nil, "the family %s computed version %s and declares version %s",
				declared.Name, family.Version, declared.Version)
		}
		return declaredSection(family, declared), warnings, nil
	}
}

// notImplemented is the section of a family that computes nothing yet. It is
// skipped with reason not_implemented, beside any reason its inputs give, and
// carries the zero version its declaration gives (ADR-0032 clause 1).
func notImplemented[M any]() sectionFunc {
	return func(_ core.Input, declared core.FamilyDeclaration, skip []core.Reason) (section, []string, error) {
		if declared.Status != core.StatusNotImplemented {
			return nil, nil, core.Internalf(nil, "the family %s declares the status source %s, and its "+
				"registration computes nothing", declared.Name, declared.Status)
		}
		skipped := core.Skipped[M](declared.Version, core.ReasonNotImplemented, skip...)
		return declaredSection(skipped, declared), nil, nil
	}
}

// declaredSection returns a family's section carrying the method statement
// its declaration gives (ADR-0032 clause 8, ADR-0076 clause 1). The
// statement is carried whatever the family's status: it says how the family's
// values are derived, which a reader of a skipped family learns as well.
func declaredSection[M any](family core.Family[M], declared core.FamilyDeclaration) section {
	family.Method = declared.Method
	return &sectionOf[M]{family}
}

// section is one family's section on its way into the report, whatever the
// family's metric type.
type section interface {
	// degrade marks the section degraded for a condition of the analysis as
	// a whole. A skipped section stays skipped.
	degrade(core.Reason, core.Confidence)
	// place writes the section into the report under namespace.
	place(families *core.Families, namespace string) error
}

// sectionOf is a section of metric type M.
type sectionOf[M any] struct {
	family core.Family[M]
}

func (s *sectionOf[M]) degrade(reason core.Reason, confidence core.Confidence) {
	s.family.Degrade(reason, confidence)
}

func (s *sectionOf[M]) place(families *core.Families, namespace string) error {
	return core.Place(families, namespace, s.family)
}
