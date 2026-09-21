// Package model defines the normalized git history types shared by every
// stage of the pipeline. These types are the contract between collection,
// filtering, aggregation and rendering.
//
// A Commit is the collect stage's normalized record (ADR-0020 clause 2): what
// git recorded, and beside it every per-commit definition of docs/metrics.md
// section 1, decided once, by the collect stage, so that a later stage reads
// it rather than deciding it again (WP-0012).
package model

import "time"

// SchemaVersion is the version of the history artifact format produced by this
// build. Readers reject artifacts written with a different version.
//
//	1  the first form
//	2  AuthorSourceEmail added
//	3  the per-commit definitions of docs/metrics.md section 1 added, with the
//	   reasons a commit is not analysed, and the analysed commit's attributes
const SchemaVersion = 3

// FileChange is a single file's line delta within one commit.
type FileChange struct {
	// Path is the file's path after the commit.
	Path string `json:"path"`
	// PreviousPath is the path the file had before the commit, where git
	// detected the change as a rename; empty otherwise. Added and Deleted are
	// then the lines the content changed, not the file's length.
	PreviousPath string `json:"previousPath,omitempty"`
	Added        int    `json:"added"`
	Deleted      int    `json:"deleted"`
	IsBinary     bool   `json:"isBinary"`

	// Excluded marks an excluded path (docs/metrics.md section 1): one that
	// matches an exclusion pattern or that the analysed commit's attributes
	// mark generated. The change counts toward its commit's existence but not
	// toward its size.
	Excluded bool `json:"excluded,omitempty"`
}

// Commit is one normalized commit record.
type Commit struct {
	Hash        string `json:"hash"`
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	// AuthorSourceEmail is the author's address as the commit records it,
	// before .mailmap; AuthorName and AuthorEmail are after it. The two
	// differ where .mailmap folds an address into another.
	AuthorSourceEmail        string       `json:"authorSourceEmail"`
	AuthorDate               time.Time    `json:"authorDate"`
	AuthorTZOffsetMinutes    int          `json:"authorTzOffsetMinutes"`
	CommitterDate            time.Time    `json:"committerDate"`
	CommitterTZOffsetMinutes int          `json:"committerTzOffsetMinutes"`
	Parents                  []string     `json:"parents"`
	IsMerge                  bool         `json:"isMerge"`
	Subject                  string       `json:"subject"`
	Files                    []FileChange `json:"files"`

	// The fields below are decided by the collect stage under the analysis
	// configuration, which is why the record depends on that configuration
	// and not only on the repository.

	// IdentityID is the digest of the identity the author resolves to, after
	// .mailmap and configuration (docs/metrics.md section 1, ADR-0033).
	IdentityID string `json:"identityId,omitempty"`

	// Excluded is the complement of analysed-commit membership
	// (docs/metrics.md section 1): true for a commit that is not an analysed
	// commit. A commit outside the date bounds is never read, so it is not in
	// the history at all. The two reasons below say why a commit that was
	// read is not analysed; either suffices.
	//
	// The year restriction is not part of the definition and is not applied
	// here. It is a recorded deviation that core.Input applies until WP-0017
	// removes it.
	Excluded bool `json:"excluded,omitempty"`
	// MergeExcluded marks a merge left out because merges are not counted.
	MergeExcluded bool `json:"mergeExcluded,omitempty"`
	// AuthorExcluded marks a commit whose resolved identity is excluded: by
	// the configured exclusion list, or as an automation account.
	AuthorExcluded bool `json:"authorExcluded,omitempty"`

	// EffectiveLines is lines added plus lines removed, counting only the
	// paths that are not excluded (docs/metrics.md section 1).
	EffectiveLines int `json:"effectiveLines"`

	// IsBulk marks an analysed commit whose effective lines exceed the
	// outlier threshold (docs/metrics.md section 1).
	IsBulk bool `json:"isBulk,omitempty"`

	// LocalTime is the instant the commit is attributed to, in the timezone
	// offset the commit recorded for it: author local time (docs/metrics.md
	// section 1), or committer local time where the analysis attributes
	// commits to their committer date.
	LocalTime time.Time `json:"localTime"`

	// ActiveDate is LocalTime's calendar date, as the report writes a date. A
	// date is an active date when an analysed commit carries it
	// (docs/metrics.md section 1).
	ActiveDate string `json:"activeDate"`
}

// RepositoryInfo describes the analyzed repository.
type RepositoryInfo struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	HeadCommit    string `json:"headCommit"`
	DefaultBranch string `json:"defaultBranch"`
	IsShallow     bool   `json:"isShallow"`
	HasGrafts     bool   `json:"hasGrafts"`
}

// History is the complete collect-stage artifact.
type History struct {
	SchemaVersion int            `json:"schemaVersion"`
	GeneratedAt   time.Time      `json:"generatedAt"`
	ToolVersion   string         `json:"toolVersion"`
	Repository    RepositoryInfo `json:"repository"`
	// Attributes is the analysed commit's root .gitattributes as the collect
	// stage read it, empty where it has none. The records' excluded paths were
	// decided with it, and a later stage that needs the same path filter builds
	// it from here rather than from the repository.
	Attributes string   `json:"attributes,omitempty"`
	Commits    []Commit `json:"commits"`
}
