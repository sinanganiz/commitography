// Package aggregate is the aggregate stage (ADR-0020): it builds the report
// document from the collect stage's records and the replay stage's state,
// reading no file and starting no process (clause 5). Every metric family
// reaches the report through the stage's registry and by no other route:
// each family's declared inputs are resolved, the families run concurrently
// (ADR-0052 clause 3), and each section is placed under the namespace its
// family declares (ADR-0076 clauses 3 and 5, ADR-0040). Every family of
// ADR-0076 clause 6 is present with a status (ADR-0032 clause 1); a family
// with no implementation yet is skipped with reason not_implemented.
// Generation values go to the metadata section alone (ADR-0021 clause 6).
// Beside the families it writes the identities section, the resolved
// contributors a reader selects from (ADR-0010 clause 2, docs/metrics.md
// section 14).
package aggregate

import (
	"github.com/sinanganiz/commitography/internal/core"
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

	warnings, err := route(in, &r.Families, entries())
	if err != nil {
		return nil, nil, err
	}
	// Every family is present in every report (ADR-0032 clause 1).
	if missing := r.Families.Unplaced(); len(missing) > 0 {
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
