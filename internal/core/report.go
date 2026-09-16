// Package core holds what every other package may depend on: the report
// types, privacy rewriting and the shared definitions the metric families
// compute over (ADR-0040 clause 1). The report types are the snapshot
// contract of ADR-0021 and are versioned under ADR-0031. Configuration,
// identity, the domain model and filtering live in subpackages beneath it
// (ADR-0066 clause 2).
package core

import "time"

// SchemaVersion is the version of report.json produced by this build. Within a
// version, fields are never removed or repurposed; consumers may rely on that.
const SchemaVersion = 1

// Report is the complete analysis artifact.
type Report struct {
	SchemaVersion int               `json:"schemaVersion"`
	GeneratedAt   time.Time         `json:"generatedAt"`
	ToolVersion   string            `json:"toolVersion"`
	Repository    RepositorySummary `json:"repository"`
	Temporal      TemporalMetrics   `json:"temporal"`
	Code          CodeMetrics       `json:"code"`
	Messages      MessageMetrics    `json:"messages"`
	Social        SocialMetrics     `json:"social"`
	Notables      Notables          `json:"notables"`
	PerAuthor     *PerAuthor        `json:"perAuthor,omitempty"`
	Warnings      []string          `json:"warnings"`
}

// ExclusionBreakdown explains, per reason, what was left out of the analysis,
// so no number on the dashboard is unexplained.
//
// The reasons overlap: a merge commit authored by a bot is counted under both.
// Total is computed independently and is the authoritative figure.
type ExclusionBreakdown struct {
	Merges int `json:"merges"`
	Bots   int `json:"bots"`
	Total  int `json:"total"`
}

// RepositorySummary is the headline description of what was measured.
type RepositorySummary struct {
	Name            string             `json:"name"`
	DefaultBranch   string             `json:"defaultBranch"`
	HeadCommit      string             `json:"headCommit"`
	FirstCommit     *time.Time         `json:"firstCommit"`
	LastCommit      *time.Time         `json:"lastCommit"`
	AgeDays         int                `json:"ageDays"`
	CommitsTotal    int                `json:"commitsTotal"`
	CommitsAnalyzed int                `json:"commitsAnalyzed"`
	CommitsExcluded ExclusionBreakdown `json:"commitsExcluded"`
	Contributors    int                `json:"contributors"`
	TrackedFiles    int                `json:"trackedFiles"`
	TrackedLines    int                `json:"trackedLines"`
	IsShallow       bool               `json:"isShallow"`
}
