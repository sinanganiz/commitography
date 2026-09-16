// Package temporal is the temporal metric family (ADR-0024, ADR-0040): when the
// repository is awake, in author local time. It also computes the notable
// events and the per-contributor section, whose figures are the same local
// time measurements taken per commit and per identity.
package temporal

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// dateLayout is the calendar-date form used everywhere in the report.
const dateLayout = "2006-01-02"

// weekdayIndex maps Go's Sunday-first weekday onto the report's Monday-first
// convention, where index 0 is Monday and index 6 is Sunday.
func weekdayIndex(t time.Time) int {
	return (int(t.Weekday()) + 6) % 7
}

// isNightOwl reports whether a local hour falls in the late-night window.
func isNightOwl(hour int) bool { return hour >= 22 || hour < 6 }

// BuildTemporal computes the temporal metrics over the analyzed commits.
func BuildTemporal(in core.Input, commits []model.Commit) core.TemporalMetrics {
	m := core.TemporalMetrics{
		HourHistogram:    make([]int, 24),
		WeekdayHistogram: make([]int, 7),
		HourWeekdayGrid:  make([][]int, 7),
		CommitsPerMonth:  []core.MonthCount{},
	}
	for i := range m.HourWeekdayGrid {
		m.HourWeekdayGrid[i] = make([]int, 24)
	}
	if len(commits) == 0 {
		return m
	}

	dates := make([]time.Time, 0, len(commits))
	perDay := map[string]int{}
	perMonth := map[string]int{}
	nightOwls := 0

	for _, c := range commits {
		t := in.Date(c)
		dates = append(dates, t)

		hour := t.Hour()
		weekday := weekdayIndex(t)

		m.HourHistogram[hour]++
		m.WeekdayHistogram[weekday]++
		m.HourWeekdayGrid[weekday][hour]++

		if weekday == 4 && hour >= 17 { // Friday, after 17:00 local
			m.BraveDeploys++
		}
		if isNightOwl(hour) {
			nightOwls++
		}

		perDay[t.Format(dateLayout)]++
		perMonth[t.Format("2006-01")]++
	}

	m.NightOwlRatio = core.Round(float64(nightOwls)/float64(len(commits)), 4)

	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	first, last := dates[0], dates[len(dates)-1]
	m.FirstCommit = &first
	m.LastCommit = &last

	m.BusiestDay = busiestDay(perDay)
	m.LongestStreak = longestStreak(perDay)
	m.LongestSilence = longestSilence(dates)
	m.CommitsPerMonth = commitsPerMonth(perMonth, first, last)

	return m
}

// busiestDay returns the local calendar date with the most commits. Ties break
// on the earliest date so the answer never changes between runs.
func busiestDay(perDay map[string]int) *core.DateCount {
	if len(perDay) == 0 {
		return nil
	}
	best := core.DateCount{}
	for date, count := range perDay {
		if count > best.Count || (count == best.Count && (best.Date == "" || date < best.Date)) {
			best = core.DateCount{Date: date, Count: count}
		}
	}
	return &best
}

// longestStreak finds the longest run of consecutive local calendar dates that
// each carry at least one commit.
func longestStreak(perDay map[string]int) *core.Span {
	if len(perDay) == 0 {
		return nil
	}
	days := make([]time.Time, 0, len(perDay))
	for date := range perDay {
		t, err := time.Parse(dateLayout, date)
		if err != nil {
			continue
		}
		days = append(days, t)
	}
	if len(days) == 0 {
		return nil
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	best := core.Span{Days: 1, StartDate: days[0].Format(dateLayout), EndDate: days[0].Format(dateLayout)}
	runStart, runLen := days[0], 1
	for i := 1; i < len(days); i++ {
		if days[i].Equal(days[i-1].AddDate(0, 0, 1)) {
			runLen++
		} else {
			runStart, runLen = days[i], 1
		}
		if runLen > best.Days {
			best = core.Span{
				Days:      runLen,
				StartDate: runStart.Format(dateLayout),
				EndDate:   days[i].Format(dateLayout),
			}
		}
	}
	return &best
}

// longestSilence finds the widest gap, in whole days, between chronologically
// adjacent commits. A repository with a single commit, or with all commits on
// one day, has a silence of zero days.
func longestSilence(sorted []time.Time) *core.Span {
	if len(sorted) == 0 {
		return nil
	}
	best := core.Span{
		Days:      0,
		StartDate: sorted[0].Format(dateLayout),
		EndDate:   sorted[0].Format(dateLayout),
	}
	for i := 1; i < len(sorted); i++ {
		gap := int(sorted[i].Sub(sorted[i-1]).Hours() / 24)
		if gap > best.Days {
			best = core.Span{
				Days:      gap,
				StartDate: sorted[i-1].Format(dateLayout),
				EndDate:   sorted[i].Format(dateLayout),
			}
		}
	}
	return &best
}

// commitsPerMonth produces a contiguous series from the first to the last month
// of activity, zero-filling the quiet months so the chart shows the gaps rather
// than hiding them.
func commitsPerMonth(perMonth map[string]int, first, last time.Time) []core.MonthCount {
	out := []core.MonthCount{}
	cursor := time.Date(first.Year(), first.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(last.Year(), last.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cursor.After(end) {
		key := cursor.Format("2006-01")
		out = append(out, core.MonthCount{Month: key, Count: perMonth[key]})
		cursor = cursor.AddDate(0, 1, 0)
	}
	return out
}
