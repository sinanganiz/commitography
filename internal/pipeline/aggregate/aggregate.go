// Package aggregate is the aggregate stage (ADR-0020): it builds the report
// document by running the metric families over the filtered commits
// (ADR-0024, ADR-0040). Every family of ADR-0024 clause 5 is placed in the
// document with a status (ADR-0032 clause 1); a family with no implementation
// yet is skipped with reason not_implemented. Generation values go to the
// metadata section alone (ADR-0021 clause 6). Beside the families it writes
// the identities section, the resolved contributors a reader selects from
// (ADR-0010 clause 2, docs/metrics.md section 14).
package aggregate

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/coupling"
	"github.com/sinanganiz/commitography/internal/metrics/hotspot"
	"github.com/sinanganiz/commitography/internal/metrics/messages"
	"github.com/sinanganiz/commitography/internal/metrics/ownership"
)

// Builder is the aggregate stage. It holds the clock that stamps the report's
// generation time, injected (ADR-0042 clause 1).
type Builder struct {
	clock core.Clock
}

// New constructs the aggregate stage. The file access it takes is not read:
// the stage reads no file (ADR-0020 clause 3), and the parameter remains only
// for the composition locations that still pass it.
func New(clock core.Clock, _ core.Filesystem) *Builder {
	return &Builder{clock: clock}
}

// Build computes the complete report, and returns with it the diagnostics
// aggregation raised. Diagnostics are not report content: the caller shows
// them where it shows its other warnings.
func (b *Builder) Build(in core.Input) (*core.Report, []string, error) {
	analyzed := in.Analyzed()
	lineScoped := in.LineScoped()
	var warnings []string

	r := &core.Report{
		DocumentVersion: core.DocumentVersion(),
		Sections:        core.CurrentSectionVersions(),
		Metadata: core.Metadata{
			GeneratedAt: b.clock.Now().UTC(),
			ToolVersion: in.ToolVersion,
		},
		Repository: core.Repository{
			Name:   in.Repository.Name,
			Commit: in.Repository.HeadCommit,
		},
		Identities: buildIdentities(in, analyzed),
	}
	// The configuration section is written after the identities section,
	// because anonymised output writes an identity's pseudonym in place of a
	// configured name, and a pseudonym only means something where the reader
	// can see the entry it names (ADR-0068 clause 4).
	r.Configuration = core.EmbedConfiguration(in.Config, in.Resolver, r.Identities)
	f := &r.Families

	routed, err := runFamilies(in, f)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, routed...)

	progress(in, "metrics", "code", 0, 0)
	f.CommitSize = commitsize.Build(in, lineScoped)
	files, err := b.buildFiles(in, lineScoped)
	if err != nil {
		return nil, nil, err
	}
	f.Files = files

	progress(in, "metrics", "messages", 0, 0)
	f.Messages = messages.Build(analyzed)

	progress(in, "metrics", "social", 0, 0)
	scoped := core.ScopedCommits(in, lineScoped)
	var couplingWarnings []string
	f.Coupling, couplingWarnings = coupling.Build(scoped)
	warnings = append(warnings, couplingWarnings...)
	f.Hotspot = hotspot.Build(scoped)
	f.Ownership = ownership.Build()

	// The families below have no implementation yet. Each is present, as
	// skipped, so the document's shape is final and the package that
	// implements a family replaces one line here (ADR-0032 clause 1).
	f.Worktype = core.Skipped[core.WorktypeMetrics](core.Version{}, core.ReasonNotImplemented)
	f.AIArchaeology = core.Skipped[core.AIArchaeologyMetrics](core.Version{}, core.ReasonNotImplemented)
	f.StaticAnalysis = core.Skipped[core.StaticAnalysisMetrics](core.Version{}, core.ReasonNotImplemented)

	// An author that cannot be resolved into an identity is neither dropped
	// nor split: every family attributing values to identities is marked
	// degraded instead (ADR-0032, docs/metrics.md section 13).
	if unresolvedAuthor(in, analyzed) {
		degradeIdentityAttributed(f)
	}

	// An analysis the operator let proceed on a shallow clone has computed
	// every family over an incomplete history (docs/metrics.md section 13).
	if in.Repository.IsShallow {
		f.Degrade(core.ReasonShallowClone, core.ConfidenceLow)
	}

	// Every family is present in every report (ADR-0032 clause 1).
	if missing := f.Unplaced(); len(missing) > 0 {
		return nil, nil, core.Internalf(nil, "the report has no section for the families %v", missing)
	}
	return r, warnings, nil
}

// progress reports that a stage has begun, when the caller asked to hear.
func progress(in core.Input, stage, detail string, current, total int) {
	if in.Progress != nil {
		in.Progress(stage, detail, current, total)
	}
}
