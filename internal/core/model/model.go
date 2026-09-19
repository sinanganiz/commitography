// Package model defines the normalized git history types shared by every
// stage of the pipeline. These types are the contract between collection,
// filtering, aggregation and rendering.
package model

import "time"

// SchemaVersion is the version of the history artifact format produced by this
// build. Readers reject artifacts written with a different version.
//
//	1  the first form
//	2  AuthorSourceEmail added
const SchemaVersion = 2

// FileChange is a single file's line delta within one commit.
type FileChange struct {
	Path     string `json:"path"`
	Added    int    `json:"added"`
	Deleted  int    `json:"deleted"`
	IsBinary bool   `json:"isBinary"`
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

	// Populated during filtering, not during collection.
	IdentityID string `json:"identityId,omitempty"`
	IsBulk     bool   `json:"isBulk,omitempty"`
	Excluded   bool   `json:"excluded,omitempty"`
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
	Commits       []Commit       `json:"commits"`
}
