// Package temporal is the temporal metric family (ADR-0024, ADR-0040): when the
// repository is awake, in author local time. Its metrics are those of
// docs/metrics.md section 2 (ADR-0062); a metric that section does not define
// is not computed here.
package temporal

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// dateLayout is the calendar-date form used everywhere in the report.
const dateLayout = "2006-01-02"

// version is the family version (ADR-0031 clause 2).
func version() core.Version { return core.Version{Major: 1, Minor: 0} }

// weekdayIndex maps Go's Sunday-first weekday onto the report's Monday-first
// convention, where index 0 is Monday and index 6 is Sunday.
func weekdayIndex(t time.Time) int {
	return (int(t.Weekday()) + 6) % 7
}

// isNight reports whether a local hour falls in the night window.
func isNight(hour int) bool { return hour >= 22 || hour < 6 }

// Build computes the temporal family over the analysed commits. With no
// analysed commit every metric is absent and the family is degraded with
// empty_population, never zero-filled.
func Build(in core.Input, commits []model.Commit) core.Family[core.TemporalMetrics] {
	if len(commits) == 0 {
		f := core.Computed(version(), core.TemporalMetrics{})
		f.Degrade(core.ReasonEmptyPopulation, core.ConfidencePartial)
		return f
	}

	m := core.TemporalMetrics{
		HourHistogram:    make([]int, 24),
		WeekdayHistogram: make([]int, 7),
	}
	dates := make([]time.Time, 0, len(commits))
	perDay := map[string]int{}
	night, weekend, fridayEvening := 0, 0, 0

	for _, c := range commits {
		t := in.Date(c)
		dates = append(dates, t)

		hour := t.Hour()
		weekday := weekdayIndex(t)

		m.HourHistogram[hour]++
		m.WeekdayHistogram[weekday]++

		if weekday == 4 && hour >= 17 { // Friday, from 17:00 local
			fridayEvening++
		}
		if weekday >= 5 {
			weekend++
		}
		if isNight(hour) {
			night++
		}
		perDay[t.Format(dateLayout)]++
	}

	nightRatio := core.Round(float64(night)/float64(len(commits)), 4)
	weekendRatio := core.Round(float64(weekend)/float64(len(commits)), 4)
	m.NightRatio = &nightRatio
	m.WeekendRatio = &weekendRatio
	m.FridayEveningCount = &fridayEvening

	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	first, last := dates[0].Format(dateLayout), dates[len(dates)-1].Format(dateLayout)
	m.FirstCommitDate = &first
	m.LastCommitDate = &last

	m.BusiestDay = busiestDay(perDay)
	streak := longestStreak(perDay)
	silence := longestSilence(dates)
	m.LongestStreakDays = &streak
	m.LongestSilenceDays = &silence

	return core.Computed(version(), m)
}

// busiestDay returns the local calendar date with the most commits. Ties break
// on the earliest date so the answer never changes between runs.
func busiestDay(perDay map[string]int) *core.DateCount {
	best := core.DateCount{}
	for date, count := range perDay {
		if count > best.Count || (count == best.Count && (best.Date == "" || date < best.Date)) {
			best = core.DateCount{Date: date, Count: count}
		}
	}
	return &best
}

// longestStreak returns the longest run, in days, of consecutive local
// calendar dates that each carry at least one commit.
func longestStreak(perDay map[string]int) int {
	days := make([]time.Time, 0, len(perDay))
	for date := range perDay {
		t, err := time.Parse(dateLayout, date)
		if err != nil {
			continue
		}
		days = append(days, t)
	}
	if len(days) == 0 {
		return 0
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	best, run := 1, 1
	for i := 1; i < len(days); i++ {
		if days[i].Equal(days[i-1].AddDate(0, 0, 1)) {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
	}
	return best
}

// longestSilence returns the widest gap, in whole days, between
// chronologically adjacent commits. A repository with a single commit, or with
// all commits on one day, has a silence of zero days.
func longestSilence(sorted []time.Time) int {
	best := 0
	for i := 1; i < len(sorted); i++ {
		if gap := int(sorted[i].Sub(sorted[i-1]).Hours() / 24); gap > best {
			best = gap
		}
	}
	return best
}
