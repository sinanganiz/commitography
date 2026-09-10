package analysis

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/aggregate"
	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/identity"
	"github.com/sinanganiz/commitography/internal/model"
)

const minWrappedCommits = 10

// Run performs one complete analysis without rendering or writing output
// files. It keeps the Phase 1 pipeline order so CLI and server callers receive
// the same report for the same inputs.
func Run(ctx context.Context, opts Options, sink ProgressSink) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	repoPath, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return nil, &UsageError{Err: fmt.Errorf("resolving %s: %w", opts.RepoPath, err)}
	}

	emit := eventEmitter{sink: sink}
	emit.emit(StagePreflight, "validating repository")
	info, err := collect.Preflight(repoPath)
	if err != nil {
		return nil, &UsageError{Err: err}
	}
	if info.IsShallow && !opts.AllowShallow {
		return nil, &UsageError{Err: &collect.ShallowError{Path: repoPath}}
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	config.Warn = func(format string, args ...any) {
		if opts.OnWarning != nil {
			opts.OnWarning(fmt.Sprintf(format, args...))
		}
	}
	cfg, err := config.Load(opts.ConfigPath, repoPath)
	if err != nil {
		return nil, &UsageError{Err: err}
	}
	if opts.Anonymize {
		cfg.Anonymize = true
	}
	if opts.CountMergesSet {
		cfg.CountMerges = opts.CountMerges
	}
	if err := cfg.Validate(); err != nil {
		return nil, &UsageError{Err: err}
	}

	warnings := make([]string, 0)
	collectWarn := func(message string) {
		warnings = append(warnings, message)
		if opts.OnWarning != nil {
			opts.OnWarning(message)
		}
	}
	emit.emit(StageCollecting, "reading history")
	history, err := collect.Collect(collect.Options{
		RepoPath:   repoPath,
		UseMailmap: cfg.UseMailmap,
		Since:      opts.Since,
		Until:      opts.Until,
		Context:    ctx,
		OnWarning:  collectWarn,
	})
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	emit.emit(StageCollecting, fmt.Sprintf("%d commits", len(history.Commits)))

	emit.emit(StageIdentity, "resolving identities")
	resolver := identity.NewResolver(cfg, history.Commits)
	emit.emit(StageIdentity, fmt.Sprintf("%d contributors", len(resolver.Identities())))

	pathFilter, err := filter.NewPathFilter(cfg, repoPath)
	if err != nil {
		return nil, &UsageError{Err: err}
	}
	filtered := filter.Apply(history.Commits, cfg, resolver, pathFilter)
	emit.emit(StageFiltering, fmt.Sprintf("%d excluded", filtered.TotalCommits-filtered.AnalyzedCommits))

	if opts.Year != 0 {
		inYear := countInYear(filtered.Commits, opts.Year, cfg)
		if inYear < minWrappedCommits {
			return nil, &YearError{Year: opts.Year, Found: inYear, Need: minWrappedCommits}
		}
	}

	input := aggregate.Input{
		Context:    ctx,
		RepoPath:   repoPath,
		Repository: history.Repository,
		Config:     cfg,
		Filtered:   filtered,
		Resolver:   resolver,
		PathFilter: pathFilter,
		NoBlame:    opts.NoBlame,
		PerAuthor:  opts.PerAuthor,
		Year:       opts.Year,
		Warnings:   warnings,
		Progress: func(stage, detail string) {
			mapped := StageCode
			if stage == "metrics" {
				switch detail {
				case "temporal":
					mapped = StageTemporal
				case "code":
					mapped = StageCode
				case "messages":
					mapped = StageMessages
				case "social":
					mapped = StageSocial
				case "notables":
					mapped = StageNotables
				}
			} else if stage == "blame" {
				mapped = StageCode
			}
			emit.emit(mapped, detail)
		},
	}

	if err := contextError(ctx); err != nil {
		return nil, err
	}
	report, err := aggregate.Build(input)
	if err != nil {
		return nil, err
	}

	var previousYearCommits *int
	if opts.Year != 0 {
		previous := countInYear(filtered.Commits, opts.Year-1, cfg)
		if previous > 0 {
			previousYearCommits = &previous
		}
	}
	emit.emit(StageFinalizing, "analysis complete")

	return &Result{
		Report:              report,
		Repository:          history.Repository,
		Config:              cfg,
		Warnings:            append([]string(nil), report.Warnings...),
		PreviousYearCommits: previousYearCommits,
	}, nil
}

type eventEmitter struct {
	sink ProgressSink
	seq  uint64
}

func (e *eventEmitter) emit(stage, detail string) {
	e.seq++
	if e.sink != nil {
		e.sink(ProgressEvent{Sequence: e.seq, Stage: stage, Detail: detail})
	}
}

func countInYear(commits []model.Commit, year int, cfg config.Config) int {
	count := 0
	for _, commit := range commits {
		if commit.Excluded {
			continue
		}
		if filter.CommitDate(commit, cfg).Year() == year {
			count++
		}
	}
	return count
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Ensure the error types remain discoverable by callers without forcing them
// to import implementation details from the CLI package.
var _ error = (*UsageError)(nil)
var _ error = (*YearError)(nil)
