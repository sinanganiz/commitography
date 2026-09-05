// Package model defines the normalized git history types shared by every
// stage of the pipeline. These types are the contract between collection,
// filtering, aggregation and rendering.
package model

import "time"

// SchemaVersion is the version of the history artifact format produced by this
// build. Readers reject artifacts written with a different version.
const SchemaVersion = 1

// FileChange is a single file's line delta within one commit.
type FileChange struct {
	Path     string `json:"path"`
	Added    int    `json:"added"`
	Deleted  int    `json:"deleted"`
	IsBinary bool   `json:"isBinary"`
}

// Commit is one normalized commit record.
type Commit struct {
	Hash                     string       `json:"hash"`
	AuthorName               string       `json:"authorName"`
	AuthorEmail              string       `json:"authorEmail"`
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
