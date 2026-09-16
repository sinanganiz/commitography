// Report types for the per-contributor section, part of the report.json
// contract (ADR-0021, ADR-0031).

package core

import "time"

// AuthorSummary is one contributor's activity.
//
// These figures describe participation, not performance. Nothing here is
// intended to rank people, and the ordering of the containing list deliberately
// avoids implying one.
type AuthorSummary struct {
	IdentityID    string    `json:"identityId"`
	DisplayName   string    `json:"displayName"`
	Emails        []string  `json:"emails"`
	Commits       int       `json:"commits"`
	Added         int       `json:"added"`
	Deleted       int       `json:"deleted"`
	FilesTouched  int       `json:"filesTouched"`
	HourHistogram []int     `json:"hourHistogram"`
	FirstCommit   time.Time `json:"firstCommit"`
	LastCommit    time.Time `json:"lastCommit"`
	ActiveDays    int       `json:"activeDays"`
}

// PerAuthor is the opt-in per-contributor section, present only when
// --per-author was passed.
type PerAuthor struct {
	Authors []AuthorSummary `json:"authors"`
}
