// Package analysis contains the shared, output-free analysis service used by
// the command-line and local web adapters.
package analysis

import (
	"context"
	"fmt"

	"github.com/sinanganiz/commitography/internal/aggregate"
	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/model"
)

// Options contains analysis behavior that is independent of the output
// adapter. Output directories and rendering flags deliberately do not belong
// here: the analysis service returns a report and does not write files.
type Options struct {
	RepoPath         string
	ConfigPath       string
	Since            string
	Until            string
	Year             int
	CheckConsistency bool
	PerAuthor        bool
	Anonymize        bool
	NoBlame          bool
	AllowShallow     bool
	CountMerges      bool
	CountMergesSet   bool
	OnWarning        func(string)
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
// are the Phase 1.5 job status contract consumed by the web application.
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

// RunFunc is the callable shape of the shared analysis service. The concrete
// implementation is added by the orchestration work package.
type RunFunc func(context.Context, Options, ProgressSink) (*Result, error)

// UsageError marks an input, configuration or repository validation failure.
// CLI adapters map it to their documented usage exit code.
type UsageError struct{ Err error }

// Error returns the underlying validation message.
func (e *UsageError) Error() string { return e.Err.Error() }

// Unwrap exposes the underlying error to errors.Is and errors.As.
func (e *UsageError) Unwrap() error { return e.Err }

// YearError reports that a requested Wrapped year has too little activity.
type YearError struct {
	Year  int
	Found int
	Need  int
}

// Error returns the stable user-facing Wrapped validation message.
func (e *YearError) Error() string {
	return fmt.Sprintf("not enough commits in %d to generate a wrapped report (found %d, need at least %d)", e.Year, e.Found, e.Need)
}

// Result contains the report and collection metadata returned by the shared
// analysis service.
type Result struct {
	Report              *aggregate.Report
	Repository          model.RepositoryInfo
	Config              config.Config
	Warnings            []string
	PreviousYearCommits *int
	EndRepository       *model.RepositoryInfo
	Stale               bool
	StaleReason         string
}
