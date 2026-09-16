// Report types for the notables section, part of the report.json contract
// (ADR-0021, ADR-0031).

package core

import "github.com/sinanganiz/commitography/internal/core/filter"

// TimezoneShare is the commit count observed at one UTC offset.
type TimezoneShare struct {
	OffsetMinutes int `json:"offsetMinutes"`
	Commits       int `json:"commits"`
}

// Notables holds the fun facts. Every field is either populated or explicitly
// null, so the renderer never has to show a placeholder.
type Notables struct {
	BulkCommits           []filter.BulkCommit `json:"bulkCommits"`
	LatestNightCommit     *CommitRef          `json:"latestNightCommit"`
	EarliestMorningCommit *CommitRef          `json:"earliestMorningCommit"`
	WeekendRatio          float64             `json:"weekendRatio"`
	HolidayCommits        int                 `json:"holidayCommits"`
	FirstCommitSubject    *string             `json:"firstCommitSubject"`
	MergeCount            int                 `json:"mergeCount"`
	TimezoneSpread        []TimezoneShare     `json:"timezoneSpread"`
}
