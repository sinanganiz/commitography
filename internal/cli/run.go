package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/analysis"
	"github.com/sinanganiz/commitography/internal/collect"
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

	analysisOpts := analysis.Options{
		RepoPath:       opts.RepoPath,
		ConfigPath:     opts.ConfigPath,
		Since:          opts.Since,
		Until:          opts.Until,
		PerAuthor:      opts.PerAuthor,
		Anonymize:      opts.Anonymize,
		NoBlame:        opts.NoBlame,
		AllowShallow:   opts.AllowShallow,
		CountMerges:    opts.CountMerges,
		CountMergesSet: opts.countMergesSet,
		Year:           opts.Wrapped,
		OnWarning:      func(message string) { progress.Warn("%s", message) },
	}
	result, err := analysis.Run(context.Background(), analysisOpts, func(event analysis.ProgressEvent) {
		progress.Stage(cliStage(event.Stage), event.Detail)
	})
	if err != nil {
		return adaptAnalysisError(err)
	}

	outputDir := result.Config.OutputDir
	if opts.outputDirSet {
		outputDir = opts.OutputDir
	}

	if opts.Wrapped != 0 {
		path := filepath.Join(outputDir, render.WrappedFileName(opts.Wrapped))
		progress.Stage("Rendering", path)
		if err := render.RenderWrapped(result.Report, outputDir, opts.Wrapped, result.PreviousYearCommits); err != nil {
			return err
		}
		progress.Done(path)
		return nil
	}

	if opts.JSONOnly {
		path := filepath.Join(outputDir, render.ReportFile)
		progress.Stage("Rendering", path)
		if err := render.WriteReportJSON(result.Report, path); err != nil {
			return err
		}
		progress.Done(path)
		return nil
	}

	progress.Stage("Rendering", filepath.Join(outputDir, render.IndexFile))
	if err := render.Render(result.Report, outputDir); err != nil {
		return err
	}
	progress.Done(filepath.Join(outputDir, render.IndexFile))
	return nil
}

func adaptAnalysisError(err error) error {
	var usage *analysis.UsageError
	if errors.As(err, &usage) {
		return &UsageError{err}
	}
	var year *analysis.YearError
	if errors.As(err, &year) {
		return &UsageError{err}
	}
	return err
}

func cliStage(stage string) string {
	switch stage {
	case analysis.StagePreflight:
		return "Validating repository"
	case analysis.StageCollecting:
		return "Reading history"
	case analysis.StageIdentity:
		return "Resolving identities"
	case analysis.StageFiltering:
		return "Filtering"
	case analysis.StageCode:
		return "Computing metrics"
	case analysis.StageFinalizing:
		return "Finalizing"
	default:
		return "Computing metrics"
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
