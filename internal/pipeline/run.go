package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

const minWrappedCommits = 10

// configurationRemedy is the remedy for every configuration refusal: they all
// come from the same file and are all fixed the same way.
const configurationRemedy = "Correct the setting in " + config.FileName +
	", or remove the file to use the defaults; --config points at another file."

// Analyzer is the shared analysis service. It composes the stages it is
// given, and holds nothing else but the file access configuration loading and
// path filtering read through (ADR-0042 clause 1, ADR-0060 clause 3).
type Analyzer struct {
	collector *collect.Collector
	builder   *aggregate.Builder
	files     core.Filesystem
}

// New constructs the analysis service from its stages and file access.
func New(collector *collect.Collector, builder *aggregate.Builder, files core.Filesystem) *Analyzer {
	return &Analyzer{collector: collector, builder: builder, files: files}
}

// Run performs one complete analysis without rendering or writing output
// files. It runs one fixed stage order so CLI and server callers receive the
// same report for the same inputs.
func (a *Analyzer) Run(ctx context.Context, opts Options, sink ProgressSink) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	// repoPath is the resolved form, used for every git invocation and every
	// containment check. opts.RepoPath is the form the operator supplied, and
	// is the only one a message may name (ADR-0067 clause 5).
	repoPath, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return nil, core.Internalf(err, "resolving the repository path")
	}

	emit := eventEmitter{sink: sink}
	emit.emit(StagePreflight, "validating repository")
	info, err := a.collector.Preflight(ctx, repoPath, opts.SuppliedPath())
	if err != nil {
		return nil, err
	}
	if info.IsShallow && !opts.AllowShallow {
		return nil, collect.ShallowError(opts.SuppliedPath())
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	configWarn := func(format string, args ...any) {
		if opts.OnWarning != nil {
			opts.OnWarning(fmt.Sprintf(format, args...))
		}
	}
	settings, err := config.Load(a.files, opts.ConfigPath, repoPath, configWarn)
	if err != nil {
		// The configuration package may not import core (ADR-0066 clause 3),
		// so its errors are classified here, by their single consumer. Its
		// message names the resolved file, so the cause is kept for errors.As
		// and the offending value is the file as the operator named it.
		return nil, core.NewUserError(core.ReasonInvalidConfiguration, opts.suppliedConfigPath(),
			configurationRemedy, "the configuration could not be read").Wrapping(err)
	}
	// The run's explicit parameters are the last layer (ADR-0026 clause 6).
	cfg := opts.parameters().Apply(settings.Analysis)
	operational := settings.Operational
	operational.AllowShallow = opts.AllowShallow
	if err := cfg.Validate(); err != nil {
		// Validate's message names the offending setting and its value and no
		// path, so it is the offending value itself.
		return nil, core.NewUserError(core.ReasonInvalidConfiguration, err.Error(),
			configurationRemedy, "the configuration carries a value the analysis cannot use")
	}
	// Cardinality limits are verified, never applied (ADR-0062 clause 6); the
	// resolved plane carries the catalogue's values whatever was supplied.
	if err := core.VerifyCardinalityLimits(cfg.CardinalityLimits); err != nil {
		return nil, core.NewUserError(core.ReasonInvalidConfiguration, err.Error(),
			configurationRemedy, "the configuration supplies a cardinality limit the catalogue does not define")
	}
	cfg.CardinalityLimits = core.LimitValues()
	if cfg.Since, cfg.Until, err = a.collector.ResolveDateBounds(ctx, repoPath, cfg.Since, cfg.Until); err != nil {
		return nil, err
	}
	// The exclusion patterns are the operator's to get right, so they are
	// compiled before the history is read: the collect stage applies them
	// with the analysed commit's attributes and treats a failure as its own.
	if _, err := filter.NewPathFilterFromAttributes(cfg, nil); err != nil {
		return nil, core.NewUserError(core.ReasonInvalidConfiguration, err.Error(),
			configurationRemedy, "an exclude_paths pattern could not be compiled")
	}

	warnings := make([]string, 0)
	collectWarn := func(message string) {
		warnings = append(warnings, message)
		if opts.OnWarning != nil {
			opts.OnWarning(message)
		}
	}
	emit.emit(StageCollecting, "reading history")
	history, err := a.collector.Collect(collect.Options{
		RepoPath:     repoPath,
		SuppliedPath: opts.SuppliedPath(),
		Analysis:     &cfg,
		ToolVersion:  opts.ToolVersion,
		Context:      ctx,
		OnWarning:    collectWarn,
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

	// The records carry every per-commit definition of docs/metrics.md
	// section 1, decided by the collect stage. What follows reads them and
	// rebuilds, from the artifact alone, what the aggregate stage still takes
	// beside them: the identity layer, for the identities section, and the
	// path filter, which the stage applies to the tree until WP-0013 moves the
	// tree to replay. Neither reads the repository.
	emit.emit(StageIdentity, "resolving identities")
	resolver := identity.NewResolver(cfg, history.Commits)
	identities := resolver.Identities()
	emit.emitCount(StageIdentity, fmt.Sprintf("%d contributors", len(identities)), len(identities), len(identities))

	pathFilter, err := filter.NewPathFilterFromAttributes(cfg, []byte(history.Attributes))
	if err != nil {
		return nil, core.Internalf(err, "building the path filter from the collected attributes")
	}
	filtered := filter.Summarize(history.Commits)
	emit.emitCount(StageFiltering, fmt.Sprintf("%d excluded", filtered.TotalCommits-filtered.AnalyzedCommits), filtered.TotalCommits-filtered.AnalyzedCommits, filtered.TotalCommits)

	// The year is an analysis value, from --wrapped or from the configuration,
	// and filters every metric. Deviation, removed by WP-0017: ADR-0008
	// clause 2 requires Wrapped to be generated from the same report as the
	// dashboard, so the year must stop reaching the pipeline at all. Removing
	// it changes what callers see, which is that package's to do.
	if cfg.Year != 0 {
		inYear := countInYear(filtered.Commits, cfg.Year, cfg)
		if inYear < minWrappedCommits {
			return nil, core.NewUserError(core.ReasonYearBelowThreshold, strconv.Itoa(cfg.Year),
				fmt.Sprintf("Choose a year with at least %d analysed commits, or drop --wrapped and the year setting.",
					minWrappedCommits),
				"the requested year has %d analysed commits and the year in review needs %d",
				inYear, minWrappedCommits)
		}
	}

	input := core.Input{
		Context:     ctx,
		RepoPath:    repoPath,
		Repository:  history.Repository,
		Config:      cfg,
		Filtered:    filtered,
		Resolver:    resolver,
		PathFilter:  pathFilter,
		ToolVersion: opts.ToolVersion,
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
			}
			emit.emitProgress(mapped, detail, current, total)
		},
	}

	if err := contextError(ctx); err != nil {
		return nil, err
	}
	report, buildWarnings, err := a.builder.Build(input)
	if err != nil {
		return nil, err
	}
	for _, message := range buildWarnings {
		collectWarn(message)
	}

	var previousYearCommits *int
	if cfg.Year != 0 {
		previous := countInYear(filtered.Commits, cfg.Year-1, cfg)
		if previous > 0 {
			previousYearCommits = &previous
		}
	}
	emit.emit(StageFinalizing, "analysis complete")

	result := &Result{
		Report:              report,
		Repository:          history.Repository,
		Analysis:            cfg,
		Operational:         operational,
		Warnings:            append([]string(nil), warnings...),
		PreviousYearCommits: previousYearCommits,
	}
	if opts.CheckConsistency {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		end, err := a.collector.Preflight(ctx, repoPath, opts.SuppliedPath())
		if err != nil {
			result.Stale = true
			result.StaleReason = fmt.Sprintf("%s: %s", StaleRevalidationFailed, core.Artifact(err))
		} else {
			result.EndRepository = &end
			result.Stale, result.StaleReason = repositoryChanged(history.Repository, end)
		}
	}
	return result, nil
}

func repositoryChanged(start, end model.RepositoryInfo) (bool, string) {
	if start.HeadCommit != end.HeadCommit {
		return true, StaleHeadChanged
	}
	if start.DefaultBranch != end.DefaultBranch {
		return true, StaleCheckoutChanged
	}
	if start.IsShallow != end.IsShallow || start.HasGrafts != end.HasGrafts {
		return true, StaleHistoryChanged
	}
	return false, ""
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

func countInYear(commits []model.Commit, year int, cfg config.Analysis) int {
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
