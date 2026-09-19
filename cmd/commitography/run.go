package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// minWrappedCommits is the smallest year worth summarizing. Below it the cards
// would be mostly blank and the page would say nothing.
const minWrappedCommits = 10

// invocationRemedy is the remedy for anything wrong with the command line.
const invocationRemedy = "Run commitography --help for the accepted flags and arguments."

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

	// ToolVersion is the build's version, injected by main.
	ToolVersion string

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

// Run performs one analysis with the analysis service and logger composeRun
// constructed. It returns an error; main maps it to an exit code.
func Run(opts Options, analyzer *pipeline.Analyzer, progress *core.Logger) error {
	if opts.Quiet && opts.Verbose {
		return core.NewUserError(core.ReasonInvalidInvocation, "--quiet --verbose", invocationRemedy,
			"--quiet and --verbose cannot be used together")
	}

	analysisOpts := pipeline.Options{
		RepoPath:   opts.RepoPath,
		ConfigPath: opts.ConfigPath,

		// The command line is the operator's own, so a diagnostic may repeat
		// the paths it carries, in the form they were typed (ADR-0067 clause 3).
		OperatorSupplied: true,

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
		ToolVersion:    opts.ToolVersion,
	}
	result, err := analyzer.Run(context.Background(), analysisOpts, func(event pipeline.ProgressEvent) {
		progress.Stage(cliStage(event.Stage), event.Detail)
	})
	if err != nil {
		// The pipeline classifies its own errors, so nothing is adapted here
		// and no exit code is decided here either.
		return err
	}

	// The output directory is used for the progress lines below, unresolved in
	// both cases: as the operator typed it when the flag was given, and as the
	// configuration file spells it otherwise. Neither is absolutised for
	// display (ADR-0067 clause 5).
	//
	// The configuration case is the one residual: a configuration file setting
	// output_dir to an absolute path is a path read from a repository, which
	// ADR-0067 clause 3 does not admit. It reaches standard error only, never
	// an artifact. Removing it changes what the command prints, which WP-0007
	// could not do; the leak scan's log half does not drive this line.
	outputDir := result.Operational.OutputDir
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

func cliStage(stage string) string {
	switch stage {
	case pipeline.StagePreflight:
		return "Validating repository"
	case pipeline.StageCollecting:
		return "Reading history"
	case pipeline.StageIdentity:
		return "Resolving identities"
	case pipeline.StageFiltering:
		return "Filtering"
	case pipeline.StageCode:
		return "Computing metrics"
	case pipeline.StageFinalizing:
		return "Finalizing"
	default:
		return "Computing metrics"
	}
}

// report prints an error on standard error, which is where diagnostics go
// (ADR-0034 clause 4). The exit code is not decided here; core.ExitCode is the
// only site that decides one.
//
// It prints the error's diagnostic rendering and, inside a container, a
// hint for the mistake a container makes likely: a path that was never
// mounted, or mounted from a mistyped source, which Docker Desktop replaces
// with an empty folder.
//
// The hint is keyed on the reason code rather than on an error type, so the
// two classes remain the only error types the command knows. It repeats the
// path only in the form the operator supplied, which is what the error carries
// as its offending value (ADR-0067 clauses 3 and 5); where the error names no
// value, there is nothing a hint could point at and none is printed.
func report(w io.Writer, err error, inContainer bool) {
	fmt.Fprintf(w, "Error: %v\n", err)
	if !inContainer {
		return
	}
	value := core.OffendingValue(err)
	if value == "" {
		return
	}
	switch core.ReasonOf(err) {
	case core.ReasonPathNotFound, core.ReasonNotARepository:
		fmt.Fprintf(w, "hint: nothing usable is mounted at %s. Mount it with "+
			"--mount type=bind,source=<host path>,target=%s,readonly and check that the source path "+
			"exists; Docker Desktop mounts an empty folder in place of a mistyped one.\n", value, value)
	}
}
