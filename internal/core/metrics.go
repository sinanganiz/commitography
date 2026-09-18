// The metric types of the report document, one per family, section by section
// of docs/metrics.md (ADR-0062). Every JSON name of a field below is a metric
// that section defines; TestMetricCatalogue in internal/checks fails on any
// other.
//
// A type lists only the metrics its family computes today. A metric the
// catalogue defines and the family does not yet compute is absent, and the
// package that implements it adds its field and increments the family
// version's minor component (ADR-0031 clause 2). Every field is a pointer or a
// slice tagged omitzero, so an unmeasured metric is absent rather than zero
// (ADR-0032 clause 3).

package core

// DateCount is a local calendar date, YYYY-MM-DD, with a commit count.
type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// TemporalMetrics is section 2, `temporal`.
type TemporalMetrics struct {
	HourHistogram      []int      `json:"hour_histogram,omitzero"`
	WeekdayHistogram   []int      `json:"weekday_histogram,omitzero"`
	NightRatio         *float64   `json:"night_ratio,omitzero"`
	WeekendRatio       *float64   `json:"weekend_ratio,omitzero"`
	FridayEveningCount *int       `json:"friday_evening_count,omitzero"`
	BusiestDay         *DateCount `json:"busiest_day,omitzero"`
	LongestStreakDays  *int       `json:"longest_streak_days,omitzero"`
	LongestSilenceDays *int       `json:"longest_silence_days,omitzero"`
	FirstCommitDate    *string    `json:"first_commit_date,omitzero"`
	LastCommitDate     *string    `json:"last_commit_date,omitzero"`
}

// LargestCommit is the commit with the most effective lines.
type LargestCommit struct {
	Hash  string `json:"hash"`
	Date  string `json:"date"`
	Lines int    `json:"lines"`
	Files int    `json:"files"`
}

// CommitSizeMetrics is section 3, `commit-size`.
type CommitSizeMetrics struct {
	MeanLines     *float64       `json:"mean_lines,omitzero"`
	MedianLines   *float64       `json:"median_lines,omitzero"`
	LargestCommit *LargestCommit `json:"largest_commit,omitzero"`
}

// MessagesMetrics is section 4, `messages`.
type MessagesMetrics struct {
	ConventionalRatio *float64 `json:"conventional_ratio,omitzero"`
	MeanSubjectLength *float64 `json:"mean_subject_length,omitzero"`
	RevertCount       *int     `json:"revert_count,omitzero"`
	FixTypoCount      *int     `json:"fix_typo_count,omitzero"`
}

// ModifiedPath is one entry of the most modified paths. Removed marks a path
// absent at the analysed commit.
type ModifiedPath struct {
	Path    string `json:"path"`
	Commits int    `json:"commits"`
	Removed bool   `json:"removed"`
}

// PathDate is a path with a local calendar date, YYYY-MM-DD.
type PathDate struct {
	Path string `json:"path"`
	Date string `json:"date"`
}

// FilesMetrics is section 5, `files`.
type FilesMetrics struct {
	MostModified         []ModifiedPath `json:"most_modified,omitzero"`
	OldestUntouched      *PathDate      `json:"oldest_untouched,omitzero"`
	HeadFileCount        *int           `json:"head_file_count,omitzero"`
	TrackedTextFileCount *int           `json:"tracked_text_file_count,omitzero"`
}

// CoupledPair is two paths changed together, with the pair's support and
// confidence. Expected marks paths sharing a base name.
type CoupledPair struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Support    int     `json:"support"`
	Confidence float64 `json:"confidence"`
	Expected   bool    `json:"expected"`
}

// CouplingMetrics is section 6, `coupling`.
type CouplingMetrics struct {
	Pairs []CoupledPair `json:"pairs,omitzero"`
}

// OwnershipMetrics is section 7, `ownership`. The family needs replay-state,
// which does not exist yet, so it computes nothing.
type OwnershipMetrics struct{}

// WorktypeMetrics is section 8, `worktype`, not yet implemented.
type WorktypeMetrics struct{}

// AIArchaeologyMetrics is section 9, `ai-archaeology`, not yet implemented.
type AIArchaeologyMetrics struct{}

// ChurnFile is a file receiving many analysed commits inside one rolling
// window, with the most commits any such window holds.
type ChurnFile struct {
	Path            string `json:"path"`
	CommitsInWindow int    `json:"commits_in_window"`
}

// HotspotMetrics is section 10, `hotspot`.
type HotspotMetrics struct {
	ChurnFiles []ChurnFile `json:"churn_files,omitzero"`
}

// StaticAnalysisMetrics is section 11, `static-analysis`, which stays skipped
// until it is implemented (ADR-0012 clause 2, ADR-0032 clause 1).
type StaticAnalysisMetrics struct{}
