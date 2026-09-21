// Package pipeline is the pipeline root. It contains the shared, output-free
// analysis service used by the command-line and local web adapters, and
// composes the stages beneath it (ADR-0060 clause 1).
package pipeline

import (
	"context"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// Options contains analysis behavior that is independent of the output
// adapter. Output directories and rendering flags deliberately do not belong
// here: the analysis service returns a report and does not write files.
type Options struct {
	RepoPath   string
	ConfigPath string

	// OperatorSupplied records that RepoPath and ConfigPath are what the
	// operator typed in this invocation, so a diagnostic may name them
	// (ADR-0067 clause 3). The server leaves it false: a path that arrived in
	// a request is echoed nowhere, neither in a response nor on stderr.
	OperatorSupplied bool

	Since            string
	Until            string
	Year             int
	CheckConsistency bool
	// PerAuthor and NoBlame are accepted and read by nothing. Both are
	// deviations this package records rather than removes (WP-0010 clause 9b),
	// because removing either changes what a caller sees; WP-0017 removes
	// them. ADR-0020 forbids a flag to skip blame from existing at all, and
	// ADR-0009 clause 1 makes per-contributor output something the operator
	// always has rather than something a run turns on. Neither is a
	// configuration value, so neither has a plane (ADR-0026 clause 1).
	PerAuthor      bool
	Anonymize      bool
	NoBlame        bool
	AllowShallow   bool
	CountMerges    bool
	CountMergesSet bool
	OnWarning      func(string)

	// Parallelism is how many readers the collect stage splits a history
	// above its threshold across (ADR-0052 clauses 1 and 5). Zero derives it
	// from the available cores. It changes no value in the report (clause 6),
	// so it belongs to the operational plane; the command and the server both
	// set it here, and an operator-facing flag for it is WP-0017's.
	Parallelism int

	// ToolVersion is the build's version, recorded in the report's generation
	// metadata. The command reads it at composition (ADR-0061 clause 4).
	ToolVersion string
}

// Progress stages are stable identifiers for CLI and web progress adapters.
const (
	StagePreflight  = "preflight"
	StageCollecting = "collecting"
	StageIdentity   = "identity"
	StageFiltering  = "filtering"
	StageTemporal   = "temporal"
	StageCode       = "code"
	StageMessages   = "messages"
	StageSocial     = "social"
	StageNotables   = "notables"
	StageFinalizing = "finalizing"
)

// ProgressEvent describes one observable point in an analysis. Fraction is
// nil when the current stage cannot provide a useful estimate. The JSON names
// are the job status contract consumed by the web application.
type ProgressEvent struct {
	Sequence  uint64   `json:"sequence"`
	Stage     string   `json:"stage"`
	Detail    string   `json:"detail"`
	Fraction  *float64 `json:"fraction"`
	Current   int      `json:"current"`
	Total     int      `json:"total"`
	Estimated bool     `json:"estimated"`
}

// ProgressSink receives progress events from an analysis run.
type ProgressSink func(ProgressEvent)

// RunFunc is the callable shape of the shared analysis service, which
// Analyzer.Run implements.
type RunFunc func(context.Context, Options, ProgressSink) (*Result, error)

// parameters returns the run's explicit parameters, the last layer of the
// analysis plane's resolution (ADR-0026 clause 6).
func (o Options) parameters() config.Parameters {
	p := config.Parameters{Anonymize: o.Anonymize, Since: o.Since, Until: o.Until, Year: o.Year}
	if o.CountMergesSet {
		countMerges := o.CountMerges
		p.CountMerges = &countMerges
	}
	return p
}

// SuppliedPath returns the repository path in the form a message may name, or
// the empty string when no message may name it at all.
func (o Options) SuppliedPath() string {
	if o.OperatorSupplied {
		return o.RepoPath
	}
	return ""
}

// suppliedConfigPath returns the configuration file in the form a message may
// name. Where the operator gave no explicit path, the file is the repository's
// own, and its bare name is the accurate answer as well as the safe one.
func (o Options) suppliedConfigPath() string {
	if !o.OperatorSupplied {
		return ""
	}
	if o.ConfigPath != "" {
		return o.ConfigPath
	}
	return config.FileName
}

// Result contains the report and collection metadata returned by the shared
// analysis service.
type Result struct {
	Report     *core.Report
	Repository model.RepositoryInfo
	// Analysis is the resolved analysis plane the report embeds (ADR-0026
	// clause 2). Operational is the resolved operational plane, which the
	// report never carries; the adapter reads what it needs from it.
	Analysis            config.Analysis
	Operational         config.Operational
	Warnings            []string
	PreviousYearCommits *int
	EndRepository       *model.RepositoryInfo
	Stale               bool
	StaleReason         string
}

// Stale reasons produced by the end-of-run consistency check. A revalidation
// failure appends the failing error's artifact rendering, which carries no
// path, address or git output (ADR-0067 clause 2), so a reason is safe to show
// wherever it surfaces.
const (
	StaleHeadChanged        = "repository HEAD changed during analysis"
	StaleCheckoutChanged    = "repository checkout changed during analysis"
	StaleHistoryChanged     = "repository history metadata changed during analysis"
	StaleRevalidationFailed = "repository could not be revalidated after analysis"
)
