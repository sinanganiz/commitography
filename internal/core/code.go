package core

import "time"

// TouchedFile is one entry in the most-modified-files table.
//
// Note on field naming: `deleted` is the deleted-line count, matching `added`
// beside it. Whether the file is gone from HEAD is reported separately as
// `deletedFromHead`, because one key cannot carry both a count and a flag.
type TouchedFile struct {
	Path            string `json:"path"`
	Commits         int    `json:"commits"`
	Added           int    `json:"added"`
	Deleted         int    `json:"deleted"`
	DeletedFromHead bool   `json:"deletedFromHead"`
}

// CommitRef identifies one commit in the report without embedding the record.
type CommitRef struct {
	Hash         string    `json:"hash"`
	Subject      string    `json:"subject"`
	Date         time.Time `json:"date"`
	LinesChanged int       `json:"linesChanged"`
	Added        int       `json:"added"`
	Deleted      int       `json:"deleted"`
	Files        int       `json:"files"`
}

// FileAge names a file and when it was last modified.
type FileAge struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"lastModified"`
}

// FileTypeShare is one row of the extension distribution.
type FileTypeShare struct {
	Extension string `json:"extension"`
	Changes   int    `json:"changes"`
	Added     int    `json:"added"`
	Deleted   int    `json:"deleted"`
}

// YearLines is one stratum of the code-age chart.
type YearLines struct {
	Year  int `json:"year"`
	Lines int `json:"lines"`
}

// CodeMetrics describes what the code looks like and how it is aging.
type CodeMetrics struct {
	MostTouchedFiles     []TouchedFile   `json:"mostTouchedFiles"`
	LargestCommit        *CommitRef      `json:"largestCommit"`
	AverageCommitSize    float64         `json:"averageCommitSize"`
	MedianCommitSize     float64         `json:"medianCommitSize"`
	TotalAdded           int             `json:"totalAdded"`
	TotalDeleted         int             `json:"totalDeleted"`
	OldestUntouchedFile  *FileAge        `json:"oldestUntouchedFile"`
	FileTypeDistribution []FileTypeShare `json:"fileTypeDistribution"`
	CodeAge              []YearLines     `json:"codeAge"`

	// SurvivingFromFirstYear is null when blame was skipped, so the dashboard
	// hides the section instead of showing a misleading zero.
	SurvivingFromFirstYear *float64 `json:"survivingFromFirstYear"`

	CodeAgeSampledFiles int `json:"codeAgeSampledFiles"`
	CodeAgeTotalFiles   int `json:"codeAgeTotalFiles"`

	TrackedFiles int `json:"trackedFiles"`
	TrackedLines int `json:"trackedLines"`
}
