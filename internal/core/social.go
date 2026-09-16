package core

import (
	"time"

	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// DirectoryBusFactor is the bus factor of one directory.
type DirectoryBusFactor struct {
	Path         string `json:"path"`
	BusFactor    int    `json:"busFactor"`
	Contributors int    `json:"contributors"`
	Commits      int    `json:"commits"`
}

// CoupledPair is a pair of files that keep changing together.
type CoupledPair struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Support    int     `json:"support"`
	Confidence float64 `json:"confidence"`
	// Expected marks pairs sharing a basename, such as a file and its test, so
	// the UI can de-emphasize coupling nobody needs to be told about.
	Expected bool `json:"expected"`
}

// ChurnHotspot is a file repeatedly rewritten inside a short window.
type ChurnHotspot struct {
	Path               string    `json:"path"`
	MaxCommitsInWindow int       `json:"maxCommitsInWindow"`
	WindowStart        time.Time `json:"windowStart"`
	TotalCommits       int       `json:"totalCommits"`
	Added              int       `json:"added"`
	Deleted            int       `json:"deleted"`
}

// KnowledgeShare reports how concentrated a directory's history is. The
// contributor responsible is named only when --per-author was requested.
type KnowledgeShare struct {
	Path           string  `json:"path"`
	LargestShare   float64 `json:"largestShare"`
	Contributors   int     `json:"contributors"`
	Commits        int     `json:"commits"`
	TopContributor string  `json:"topContributor,omitempty"`
}

// SocialMetrics describes where knowledge sits and what moves together.
type SocialMetrics struct {
	BusFactor              int                  `json:"busFactor"`
	DirectoryBusFactor     []DirectoryBusFactor `json:"directoryBusFactor"`
	Coupling               []CoupledPair        `json:"coupling"`
	Churn                  []ChurnHotspot       `json:"churn"`
	KnowledgeConcentration []KnowledgeShare     `json:"knowledgeConcentration"`
}

// ScopedCommit is one commit reduced to what the social metrics need: its
// identity, its timestamp, and the files that survived path exclusion. The
// ownership, coupling and hotspot families all read it, so it is shared
// derived data (ADR-0040 clause 4).
type ScopedCommit struct {
	IdentityID string
	When       time.Time
	Files      []model.FileChange
}

// ScopedCommits reduces commits to ScopedCommit, leaving out the commits that
// touch no included file.
func ScopedCommits(in Input, commits []model.Commit) []ScopedCommit {
	scoped := make([]ScopedCommit, 0, len(commits))
	for _, c := range commits {
		files := filter.IncludedFiles(c, in.PathFilter)
		if len(files) == 0 {
			continue
		}
		scoped = append(scoped, ScopedCommit{
			IdentityID: c.IdentityID,
			When:       in.Date(c),
			Files:      files,
		})
	}
	return scoped
}
