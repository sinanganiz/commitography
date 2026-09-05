package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/aggregate"
	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/identity"
	"github.com/sinanganiz/commitography/internal/render"
)

// Exit codes. These are part of the command's contract: a CI job can branch on
// them without parsing messages.
const (
	ExitOK         = 0
	ExitInternal   = 1
	ExitUsage      = 2
	ExitStrictWarn = 3 // reserved for --strict, not implemented in Phase 1
)

// minWrappedCommits is the smallest year worth summarizing. Below it the cards
// would be mostly blank and the page would say nothing.
const minWrappedCommits = 10

// UsageError is a problem with the invocation, the configuration, or the
// repository: something the user can fix. It maps to exit code 2.
type UsageError struct{ err error }

func (e *UsageError) Error() string { return e.err.Error() }
func (e *UsageError) Unwrap() error { return e.err }

func usageErrorf(format string, args ...any) error {
	return &UsageError{fmt.Errorf(format, args...)}
}

// Options is the fully resolved invocation.
type Options struct {
	RepoPath     string
	OutputDir    string
	ConfigPath   string
	Wrapped      int
	PerAuthor    bool
	Anonymize    bool
	Since        string
	Until        string
	JSONOnly     bool
	NoBlame      bool
	AllowShallow bool
	CountMerges  bool
	Quiet        bool
	Verbose      bool

	// outputDirSet and countMergesSet record whether the flag was given at all,
	// so a configuration file can supply the value when it was not. Cobra's
	// defaults are indistinguishable from an explicit value otherwise.
	outputDirSet   bool
	countMergesSet bool
}

// SetChanged records which flags the user actually passed.
func (o *Options) SetChanged(outputDir, countMerges bool) {
	o.outputDirSet = outputDir
	o.countMergesSet = countMerges
}

// Run performs one analysis. It returns an error; main maps it to an exit code.
func Run(opts Options) error {
	if opts.Quiet && opts.Verbose {
		return usageErrorf("--quiet and --verbose cannot be used together")
	}

	progress := NewProgress(opts.Quiet, opts.Verbose)
	// Configuration warnings belong on the same stream as everything else.
	config.Warn = func(format string, args ...any) { progress.Warn(format, args...) }

	repoPath, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return usageErrorf("resolving %s: %v", opts.RepoPath, err)
	}

	info, err := collect.Preflight(repoPath)
	if err != nil {
		return &UsageError{err}
	}
	if info.IsShallow && !opts.AllowShallow {
		return &UsageError{&collect.ShallowError{Path: repoPath}}
	}

	cfg, err := config.Load(opts.ConfigPath, repoPath)
	if err != nil {
		return &UsageError{err}
	}
	applyFlags(&cfg, opts)
	if err := cfg.Validate(); err != nil {
		return &UsageError{err}
	}

	outputDir := cfg.OutputDir
	if opts.outputDirSet {
		outputDir = opts.OutputDir
	}

	var warnings []string
	collectWarn := func(msg string) {
		warnings = append(warnings, msg)
		progress.Warn("%s", msg)
	}

	progress.Stage("Reading history", "…")
	history, err := collect.Collect(collect.Options{
		RepoPath:   repoPath,
		UseMailmap: cfg.UseMailmap,
		Since:      opts.Since,
		Until:      opts.Until,
		OnWarning:  collectWarn,
	})
	if err != nil {
		return err
	}
	progress.Stage("Reading history", fmt.Sprintf("%d commits", len(history.Commits)))

	resolver := identity.NewResolver(cfg, history.Commits)
	progress.Stage("Resolving identities", fmt.Sprintf("%d contributors", len(resolver.Identities())))

	pathFilter, err := filter.NewPathFilter(cfg, repoPath)
	if err != nil {
		return &UsageError{err}
	}

	filtered := filter.Apply(history.Commits, cfg, resolver, pathFilter)
	progress.Stage("Filtering", fmt.Sprintf("%d excluded", filtered.TotalCommits-filtered.AnalyzedCommits))

	in := aggregate.Input{
		RepoPath:   repoPath,
		Repository: history.Repository,
		Config:     cfg,
		Filtered:   filtered,
		Resolver:   resolver,
		PathFilter: pathFilter,
		NoBlame:    opts.NoBlame,
		PerAuthor:  opts.PerAuthor,
		Warnings:   warnings,
		Progress: func(stage, detail string) {
			switch stage {
			case "blame":
				progress.Stage("Sampling blame", detail)
			default:
				progress.Stage("Computing metrics", detail)
			}
		},
	}

	if opts.Wrapped != 0 {
		return runWrapped(in, outputDir, opts, progress)
	}

	report, err := aggregate.Build(in)
	if err != nil {
		return err
	}

	if opts.JSONOnly {
		path := filepath.Join(outputDir, render.ReportFile)
		progress.Stage("Rendering", path)
		if err := render.WriteReportJSON(report, path); err != nil {
			return err
		}
		progress.Done(path)
		return nil
	}

	progress.Stage("Rendering", filepath.Join(outputDir, render.IndexFile))
	if err := render.Render(report, outputDir); err != nil {
		return err
	}
	progress.Done(filepath.Join(outputDir, render.IndexFile))
	return nil
}

// runWrapped narrows the analysis to a single calendar year and writes the
// year-in-review page.
func runWrapped(in aggregate.Input, outputDir string, opts Options, progress *Progress) error {
	year := opts.Wrapped

	inYear := countInYear(in, year)
	if inYear < minWrappedCommits {
		return usageErrorf(
			"not enough commits in %d to generate a wrapped report (found %d, need at least %d)",
			year, inYear, minWrappedCommits)
	}

	in.Year = year
	report, err := aggregate.Build(in)
	if err != nil {
		return err
	}

	var previous *int
	if n := countInYear(in, year-1); n > 0 {
		previous = &n
	}

	path := filepath.Join(outputDir, render.WrappedFileName(year))
	progress.Stage("Rendering", path)
	if err := render.RenderWrapped(report, outputDir, year, previous); err != nil {
		return err
	}
	progress.Done(path)
	return nil
}

// countInYear counts analyzed commits whose author-local date falls in a year.
func countInYear(in aggregate.Input, year int) int {
	count := 0
	for _, c := range in.Filtered.Commits {
		if c.Excluded {
			continue
		}
		if filter.CommitDate(c, in.Config).Year() == year {
			count++
		}
	}
	return count
}

// applyFlags folds command-line overrides over the loaded configuration. Flags
// always win, which is the last step of the documented resolution order.
//
// --per-author has no configuration counterpart; it is carried on
// aggregate.Input instead.
func applyFlags(cfg *config.Config, opts Options) {
	if opts.Anonymize {
		cfg.Anonymize = true
	}
	if opts.countMergesSet {
		cfg.CountMerges = opts.CountMerges
	}
	if opts.outputDirSet {
		cfg.OutputDir = opts.OutputDir
	}
}

// ExitCode maps an error onto the command's documented exit codes.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return ExitUsage
	}
	return ExitInternal
}

// Report prints an error in the form the exit code implies.
func Report(err error) {
	var shallow *collect.ShallowError
	if errors.As(err, &shallow) {
		fmt.Fprintf(os.Stderr, "Error: %s\n", shallow.Error())
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}
