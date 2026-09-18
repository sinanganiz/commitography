package core

import (
	"time"

	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// ScopedCommit is one commit reduced to what the path-based families need: its
// identity, its timestamp, and the files that survived path exclusion. The
// coupling and hotspot families both read it, so it is shared derived data
// (ADR-0040 clause 4).
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
