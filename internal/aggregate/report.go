// Package aggregate turns a filtered commit set into the report that the
// dashboard renders and that other tools consume.
package aggregate

import (
	"context"
	"time"

	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/identity"
	"github.com/sinanganiz/commitography/internal/model"
	"github.com/sinanganiz/commitography/internal/version"
)

// SchemaVersion is the version of report.json produced by this build. Within a
// version, fields are never removed or repurposed; consumers may rely on that.
const SchemaVersion = 1

// Report is the complete analysis artifact.
type Report struct {
	SchemaVersion int               `json:"schemaVersion"`
	GeneratedAt   time.Time         `json:"generatedAt"`
	ToolVersion   string            `json:"toolVersion"`
	Repository    RepositorySummary `json:"repository"`
	Temporal      TemporalMetrics   `json:"temporal"`
	Code          CodeMetrics       `json:"code"`
	Messages      MessageMetrics    `json:"messages"`
	Social        SocialMetrics     `json:"social"`
	Notables      Notables          `json:"notables"`
	PerAuthor     *PerAuthor        `json:"perAuthor,omitempty"`
	Warnings      []string          `json:"warnings"`
}

// ExclusionBreakdown explains, per reason, what was left out of the analysis,
// so no number on the dashboard is unexplained.
//
// The reasons overlap: a merge commit authored by a bot is counted under both.
// Total is computed independently and is the authoritative figure.
type ExclusionBreakdown struct {
	Merges int `json:"merges"`
	Bots   int `json:"bots"`
	Total  int `json:"total"`
}

// RepositorySummary is the headline description of what was measured.
type RepositorySummary struct {
	Name            string             `json:"name"`
	DefaultBranch   string             `json:"defaultBranch"`
	HeadCommit      string             `json:"headCommit"`
	FirstCommit     *time.Time         `json:"firstCommit"`
	LastCommit      *time.Time         `json:"lastCommit"`
	AgeDays         int                `json:"ageDays"`
	CommitsTotal    int                `json:"commitsTotal"`
	CommitsAnalyzed int                `json:"commitsAnalyzed"`
	CommitsExcluded ExclusionBreakdown `json:"commitsExcluded"`
	Contributors    int                `json:"contributors"`
	TrackedFiles    int                `json:"trackedFiles"`
	TrackedLines    int                `json:"trackedLines"`
	IsShallow       bool               `json:"isShallow"`
}

// Input is everything the aggregation stage needs. It is assembled by the CLI
// once collection and filtering have run.
type Input struct {
	Context    context.Context
	RepoPath   string
	Repository model.RepositoryInfo
	Config     config.Config
	Filtered   filter.Result
	Resolver   *identity.Resolver
	PathFilter *filter.PathFilter

	// NoBlame skips the sampled-blame metrics, which dominate runtime on large
	// repositories.
	NoBlame bool
	// PerAuthor populates the opt-in per-contributor section.
	PerAuthor bool
	// Year, when non-zero, restricts the analysis to one calendar year of
	// author-local activity.
	Year int

	// Warnings carries diagnostics raised by earlier stages.
	Warnings []string
	// Progress, when set, is called as each stage begins.
	Progress func(stage, detail string)
}

func (in Input) progress(stage, detail string) {
	if in.Progress != nil {
		in.Progress(stage, detail)
	}
}

func (in Input) context() context.Context {
	if in.Context == nil {
		return context.Background()
	}
	return in.Context
}

// analyzed returns the commits that count toward commit and temporal metrics:
// everything not excluded, bulk commits included.
func (in Input) analyzed() []model.Commit {
	out := make([]model.Commit, 0, len(in.Filtered.Commits))
	for _, c := range in.Filtered.Commits {
		if c.Excluded {
			continue
		}
		if in.Year != 0 && in.date(c).Year() != in.Year {
			continue
		}
		out = append(out, c)
	}
	return out
}

// lineScoped returns the commits that count toward line-based, coupling and
// churn metrics: analyzed commits minus bulk outliers.
func (in Input) lineScoped() []model.Commit {
	out := make([]model.Commit, 0, len(in.Filtered.Commits))
	for _, c := range in.analyzed() {
		if c.IsBulk {
			continue
		}
		out = append(out, c)
	}
	return out
}

// date returns the timestamp a commit is attributed to, in the author's own
// timezone. Hour-of-day analysis in UTC is meaningless, so the offset git
// recorded is preserved all the way through.
func (in Input) date(c model.Commit) time.Time {
	return filter.CommitDate(c, in.Config)
}

// Build computes the complete report.
func Build(in Input) (*Report, error) {
	analyzed := in.analyzed()
	lineScoped := in.lineScoped()

	r := &Report{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		ToolVersion:   version.Version,
		Warnings:      append([]string(nil), in.Warnings...),
	}
	if r.Warnings == nil {
		r.Warnings = []string{}
	}

	in.progress("metrics", "temporal")
	r.Temporal = buildTemporal(in, analyzed)

	in.progress("metrics", "code")
	code, codeWarnings, err := buildCode(in, analyzed, lineScoped)
	if err != nil {
		return nil, err
	}
	r.Code = code
	r.Warnings = append(r.Warnings, codeWarnings...)

	in.progress("metrics", "messages")
	r.Messages = buildMessages(analyzed)

	in.progress("metrics", "social")
	social, socialWarnings := buildSocial(in, lineScoped)
	r.Social = social
	r.Warnings = append(r.Warnings, socialWarnings...)

	in.progress("metrics", "notables")
	r.Notables = buildNotables(in, analyzed)

	if in.PerAuthor {
		r.PerAuthor = buildPerAuthor(in, analyzed)
	}

	r.Repository = buildSummary(in, analyzed, r)

	if in.Repository.IsShallow {
		r.Warnings = append([]string{
			"This repository is a shallow clone. Its history is incomplete, so every number below is wrong.",
		}, r.Warnings...)
	}

	ApplyPrivacy(r, in.Config)
	return r, nil
}

func buildSummary(in Input, analyzed []model.Commit, r *Report) RepositorySummary {
	s := RepositorySummary{
		Name:            in.Repository.Name,
		DefaultBranch:   in.Repository.DefaultBranch,
		HeadCommit:      in.Repository.HeadCommit,
		CommitsTotal:    in.Filtered.TotalCommits,
		CommitsAnalyzed: len(analyzed),
		CommitsExcluded: ExclusionBreakdown{
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
