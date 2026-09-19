// Command commitography turns a git repository's history into a self-contained
// static dashboard.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
)

// buildInfo is the link-time build metadata, read once at composition and
// passed to whatever needs it (ADR-0061 clause 4). systemEnvironment reads it.
type buildInfo struct {
	version, commit, date string
}

// String returns a single-line human-readable version string.
func (b buildInfo) String() string {
	return fmt.Sprintf("commitography %s (commit %s, built %s)", b.version, b.commit, b.date)
}

func main() {
	env := systemEnvironment()
	if err := execute(env, os.Args[1:]); err != nil {
		report(env.stderr, err, env.inContainer)
		os.Exit(core.ExitCode(err))
	}
}

// execute runs the command and classifies what it returns.
//
// Cobra rejects a malformed command line — an unknown flag, a value it cannot
// parse, too many arguments, an unknown subcommand — before the command body
// runs, and returns a plain error for every one of them. ADR-0034 clause 5
// makes all of them usage errors, and the body not having run is exactly what
// identifies them. Until this package they exited on the internal-error code.
func execute(env environment, args []string) error {
	entered := false
	cmd := newRootCommand(env, &entered)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil || entered {
		return err
	}
	return core.NewUserError(core.ReasonInvalidInvocation, "", invocationRemedy, "%s", err).Wrapping(err)
}

// newRootCommand builds the command tree. entered is set as soon as the root
// command's body runs, so execute can tell a rejected command line from a
// failure inside the analysis.
func newRootCommand(env environment, entered *bool) *cobra.Command {
	var (
		opts        Options
		showVersion bool
	)

	cmd := &cobra.Command{
		Use:   "commitography [path] [flags]",
		Short: "Turn a git repository's history into a static dashboard",
		Long: `Commitography reads a git repository's commit history and writes a
self-contained HTML dashboard describing it: when the repository is awake, which
files it keeps returning to, where knowledge is concentrated, and what its commit
messages reveal about how the team works.

Nothing is uploaded anywhere. The tool reads the repository and writes a file.

Repository-level by default; per-contributor breakdowns are opt-in behind
--per-author.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			*entered = true
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), env.build.String())
				return nil
			}

			opts.ToolVersion = env.build.version
			opts.RepoPath = "."
			if len(args) == 1 {
				opts.RepoPath = args[0]
			}
			opts.SetChanged(
				cmd.Flags().Changed("output"),
				cmd.Flags().Changed("count-merges"),
			)
			analyzer, logger := composeRun(env, opts)
			return Run(opts, analyzer, logger)
		},
	}
	cmd.SetOut(env.stdout)
	cmd.SetErr(env.stderr)

	f := cmd.Flags()
	f.StringVarP(&opts.OutputDir, "output", "o", config.DefaultOperational().OutputDir, "Output directory")
	f.StringVarP(&opts.ConfigPath, "config", "c", "", "Explicit config file path; replaces repository-local config")
	f.IntVar(&opts.Wrapped, "wrapped", 0, "Generate the year-in-review page for the given year")
	f.BoolVar(&opts.PerAuthor, "per-author", false, "Include the per-contributor section")
	f.BoolVar(&opts.Anonymize, "anonymize", false, "Replace names with pseudonyms and drop emails")
	f.StringVar(&opts.Since, "since", "", "Lower date bound, passed to git")
	f.StringVar(&opts.Until, "until", "", "Upper date bound, passed to git")
	f.BoolVar(&opts.JSONOnly, "json", false, "Write only report.json, skip HTML rendering")
	f.BoolVar(&opts.NoBlame, "no-blame", false, "Skip blame-derived metrics")
	f.BoolVar(&opts.AllowShallow, "allow-shallow", false, "Proceed despite a shallow clone")
	f.BoolVar(&opts.CountMerges, "count-merges", false, "Include merge commits in analysis")
	f.BoolVarP(&opts.Quiet, "quiet", "q", false, "Suppress progress output")
	f.BoolVarP(&opts.Verbose, "verbose", "v", false, "Emit debug logging to stderr")
	f.BoolVar(&showVersion, "version", false, "Print version and exit")
	cmd.AddCommand(newServeCommand(env))

	return cmd
}
