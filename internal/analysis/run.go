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

	configWarn := func(format string, args ...any) {
		if opts.OnWarning != nil {
			opts.OnWarning(fmt.Sprintf(format, args...))
		}
	}
	cfg, err := config.LoadWithWarn(opts.ConfigPath, repoPath, configWarn)
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
		OnProgress: func(current, total int) {
			if total > 0 {
				emit.emitCount(StageCollecting, fmt.Sprintf("%d of %d commits", current, total), current, total)
			}
		},
	})
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	emit.emitCount(StageCollecting, fmt.Sprintf("%d commits", len(history.Commits)), len(history.Commits), len(history.Commits))

	emit.emit(StageIdentity, "resolving identities")
	resolver := identity.NewResolver(cfg, history.Commits)
	identities := resolver.Identities()
	emit.emitCount(StageIdentity, fmt.Sprintf("%d contributors", len(identities)), len(identities), len(identities))

	pathFilter, err := filter.NewPathFilter(cfg, repoPath)
	if err != nil {
		return nil, &UsageError{Err: err}
	}
	filtered := filter.Apply(history.Commits, cfg, resolver, pathFilter)
	emit.emitCount(StageFiltering, fmt.Sprintf("%d excluded", filtered.TotalCommits-filtered.AnalyzedCommits), filtered.TotalCommits-filtered.AnalyzedCommits, filtered.TotalCommits)

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
		Progress: func(stage, detail string, current, total int) {
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
			emit.emitProgress(mapped, detail, current, total)
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
	e.emitProgress(stage, detail, 0, 0)
}

func (e *eventEmitter) emitCount(stage, detail string, current, total int) {
	e.emitProgress(stage, detail, current, total)
}

func (e *eventEmitter) emitProgress(stage, detail string, current, total int) {
	e.seq++
	if e.sink != nil {
		fraction, estimated := progressFraction(stage, current, total)
		e.sink(ProgressEvent{
			Sequence:  e.seq,
			Stage:     stage,
			Detail:    detail,
			Fraction:  fraction,
			Current:   current,
			Total:     total,
			Estimated: estimated,
		})
	}
}

func progressFraction(stage string, current, total int) (*float64, bool) {
	start, end, ok := progressWindow(stage)
	if !ok {
		return nil, true
	}
	if stage == StageFinalizing {
		value := end
		return &value, false
	}
	value := start
	if total > 0 && current >= 0 {
		ratio := float64(current) / float64(total)
		if ratio > 1 {
			ratio = 1
		}
		value = start + (end-start)*ratio
	}
	return &value, true
}

func progressWindow(stage string) (start, end float64, ok bool) {
	switch stage {
	case StagePreflight:
		return 0.00, 0.05, true
	case StageCollecting:
		return 0.05, 0.55, true
	case StageIdentity:
		return 0.55, 0.60, true
	case StageFiltering:
		return 0.60, 0.65, true
	case StageTemporal:
		return 0.65, 0.70, true
	case StageCode:
		return 0.70, 0.85, true
	case StageMessages:
		return 0.85, 0.90, true
	case StageSocial:
		return 0.90, 0.95, true
	case StageNotables:
		return 0.95, 0.98, true
	case StageFinalizing:
		return 0.98, 1.00, true
	default:
		return 0, 0, false
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
