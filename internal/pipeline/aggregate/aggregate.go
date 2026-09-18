// Package aggregate is the aggregate stage (ADR-0020): it builds the report by
// running the metric families over the filtered commits (ADR-0024, ADR-0040).
package aggregate

import (
	"context"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/messages"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
)

// Builder is the aggregate stage. It holds the clock that stamps the report's
// generation time and the file access its working tree reads use, both
// injected (ADR-0042 clause 1).
type Builder struct {
	clock core.Clock
	files core.Filesystem
}

// New constructs the aggregate stage.
func New(clock core.Clock, files core.Filesystem) *Builder {
	return &Builder{clock: clock, files: files}
}

// Build computes the complete report.
func (b *Builder) Build(in core.Input) (*core.Report, error) {
	analyzed := in.Analyzed()
	lineScoped := in.LineScoped()

	r := &core.Report{
		SchemaVersion: core.SchemaVersion,
		GeneratedAt:   b.clock.Now().UTC(),
		ToolVersion:   in.ToolVersion,
		Warnings:      append([]string(nil), in.Warnings...),
	}
	if r.Warnings == nil {
		r.Warnings = []string{}
	}

	progress(in, "metrics", "temporal", 0, 0)
	r.Temporal = temporal.BuildTemporal(in, analyzed)

	progress(in, "metrics", "code", 0, 0)
	code, codeWarnings, err := b.buildCode(in, analyzed, lineScoped)
	if err != nil {
		return nil, err
	}
	r.Code = code
	r.Warnings = append(r.Warnings, codeWarnings...)

	progress(in, "metrics", "messages", 0, 0)
	r.Messages = messages.BuildMessages(analyzed)

	progress(in, "metrics", "social", 0, 0)
	social, socialWarnings := buildSocial(in, lineScoped)
	r.Social = social
	r.Warnings = append(r.Warnings, socialWarnings...)

	progress(in, "metrics", "notables", 0, 0)
	r.Notables = buildNotables(in, analyzed)

	if in.PerAuthor {
		r.PerAuthor = temporal.BuildPerAuthor(in, analyzed)
	}

	r.Repository = buildSummary(in, analyzed, r)

	if in.Repository.IsShallow {
		r.Warnings = append([]string{
			"This repository is a shallow clone. Its history is incomplete, so every number below is wrong.",
		}, r.Warnings...)
	}

	core.ApplyPrivacy(r, in.Config)
	return r, nil
}

// buildNotables assembles the notable events from the temporal family and the
// commit-size family's bulk commit list.
func buildNotables(in core.Input, analyzed []model.Commit) core.Notables {
	n := temporal.BuildNotables(in, analyzed)
	n.BulkCommits = commitsize.BulkCommits(in)
	return n
}

func buildSummary(in core.Input, analyzed []model.Commit, r *core.Report) core.RepositorySummary {
	s := core.RepositorySummary{
		Name:            in.Repository.Name,
		DefaultBranch:   in.Repository.DefaultBranch,
		HeadCommit:      in.Repository.HeadCommit,
		CommitsTotal:    in.Filtered.TotalCommits,
		CommitsAnalyzed: len(analyzed),
		CommitsExcluded: core.ExclusionBreakdown{
			Merges: in.Filtered.ExcludedMerges,
			Bots:   in.Filtered.ExcludedBots,
			Total:  in.Filtered.TotalCommits - in.Filtered.AnalyzedCommits,
		},
		IsShallow:    in.Repository.IsShallow,
		TrackedFiles: r.Code.TrackedFiles,
		TrackedLines: r.Code.TrackedLines,
	}

	s.FirstCommit = r.Temporal.FirstCommit
	s.LastCommit = r.Temporal.LastCommit
	if s.FirstCommit != nil && s.LastCommit != nil {
		s.AgeDays = int(s.LastCommit.Sub(*s.FirstCommit).Hours()/24) + 1
	}

	contributors := map[string]bool{}
	for _, c := range analyzed {
		contributors[c.IdentityID] = true
	}
	s.Contributors = len(contributors)

	return s
}

// progress reports that a stage has begun, when the caller asked to hear.
func progress(in core.Input, stage, detail string, current, total int) {
	if in.Progress != nil {
		in.Progress(stage, detail, current, total)
	}
}

// contextOf returns the input's context, or a background context when none
// was given.
func contextOf(in core.Input) context.Context {
	if in.Context == nil {
		return context.Background()
	}
	return in.Context
}
