package core

import "time"

// DateCount pairs a calendar date with a commit count.
type DateCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// Span is a run of days, used for both the longest streak and the longest
// silence.
type Span struct {
	Days      int    `json:"days"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// MonthCount pairs a YYYY-MM month with a commit count.
type MonthCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

// TemporalMetrics describes when the repository is awake. Every field is
// computed in the author's local time, reconstructed from the timezone offset
// git recorded, because hour-of-day analysis in UTC says nothing about people.
type TemporalMetrics struct {
	HourHistogram    []int        `json:"hourHistogram"`
	WeekdayHistogram []int        `json:"weekdayHistogram"`
	HourWeekdayGrid  [][]int      `json:"hourWeekdayGrid"`
	BraveDeploys     int          `json:"braveDeploys"`
	NightOwlRatio    float64      `json:"nightOwlRatio"`
	BusiestDay       *DateCount   `json:"busiestDay"`
	LongestStreak    *Span        `json:"longestStreak"`
	LongestSilence   *Span        `json:"longestSilence"`
	CommitsPerMonth  []MonthCount `json:"commitsPerMonth"`
	FirstCommit      *time.Time   `json:"firstCommit"`
	LastCommit       *time.Time   `json:"lastCommit"`
}
