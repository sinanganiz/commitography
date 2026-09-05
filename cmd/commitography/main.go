// Command commitography turns a git repository's history into a self-contained
// static dashboard.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sinanganiz/commitography/internal/cli"
	"github.com/sinanganiz/commitography/internal/version"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		cli.Report(err)
		os.Exit(cli.ExitCode(err))
	}
}

func newRootCommand() *cobra.Command {
	var (
		opts        cli.Options
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
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), version.String())
				return nil
			}

			opts.RepoPath = "."
			if len(args) == 1 {
				opts.RepoPath = args[0]
			}
			opts.SetChanged(
				cmd.Flags().Changed("output"),
				cmd.Flags().Changed("count-merges"),
			)
			return cli.Run(opts)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&opts.OutputDir, "output", "o", "./out", "Output directory")
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

	return cmd
}
