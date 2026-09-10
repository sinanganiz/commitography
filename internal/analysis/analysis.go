// Package analysis contains the shared, output-free analysis service used by
// the command-line and local web adapters.
package analysis

import (
	"context"

	"github.com/sinanganiz/commitography/internal/aggregate"
	"github.com/sinanganiz/commitography/internal/model"
)

// Options contains analysis behavior that is independent of the output
// adapter. Output directories and rendering flags deliberately do not belong
// here: the analysis service returns a report and does not write files.
type Options struct {
	RepoPath     string
	ConfigPath   string
	Since        string
	Until        string
	PerAuthor    bool
	Anonymize    bool
	NoBlame      bool
	AllowShallow bool
	CountMerges  bool
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
// nil when the current stage cannot provide a useful estimate.
type ProgressEvent struct {
	Sequence  uint64
	Stage     string
	Detail    string
	Fraction  *float64
	Current   int
	Total     int
	Estimated bool
}

// ProgressSink receives progress events from an analysis run.
type ProgressSink func(ProgressEvent)

// RunFunc is the callable shape of the shared analysis service. The concrete
// implementation is added by the orchestration work package.
type RunFunc func(context.Context, Options, ProgressSink) (*aggregate.Report, error)

// Result contains the report and collection metadata returned by the shared
// analysis service.
type Result struct {
	Report     *aggregate.Report
	Repository model.RepositoryInfo
	Warnings   []string
}
