package temporal

import (
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// at builds a commit at a given local wall-clock time and UTC offset.
func at(year int, month time.Month, day, hour, minute, offsetHours int) model.Commit {
	zone := time.FixedZone("fixed", offsetHours*3600)
	when := time.Date(year, month, day, hour, minute, 0, 0, zone)
	return model.Commit{
		Hash:                  "h",
		AuthorDate:            when,
		CommitterDate:         when,
		AuthorTZOffsetMinutes: offsetHours * 60,
	}
}

// build runs the family over commits and requires it to be computed.
func build(t *testing.T, commits []model.Commit) core.TemporalMetrics {
	t.Helper()
	f := Build(core.Input{Config: config.Default()}, commits)
	if f.Status != core.StatusOK {
		t.Fatalf("status = %s %v, want ok", f.Status, f.Reasons)
	}
	return f.Metrics
}

func TestTemporalHistogramsUseAuthorLocalTime(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		// 2026-01-05 is a Monday.
		at(2026, time.January, 5, 9, 0, 0),   // Mon 09:00 UTC
		at(2026, time.January, 5, 23, 0, 3),  // Mon 23:00 +03 -> still Monday locally
		at(2026, time.January, 9, 18, 0, -5), // Fri 18:00 -05, a Friday evening
		at(2026, time.January, 10, 2, 30, 0), // Sat 02:30, night and weekend
	}
	m := build(t, commits)

	if m.HourHistogram[9] != 1 || m.HourHistogram[23] != 1 ||
		m.HourHistogram[18] != 1 || m.HourHistogram[2] != 1 {
		t.Errorf("hour histogram = %v", m.HourHistogram)
	}
	if sum(m.HourHistogram) != len(commits) {
		t.Errorf("sum(hour_histogram) = %d, want %d", sum(m.HourHistogram), len(commits))
	}
	if sum(m.WeekdayHistogram) != len(commits) {
		t.Errorf("sum(weekday_histogram) = %d, want %d", sum(m.WeekdayHistogram), len(commits))
	}

	// Index 0 is Monday, index 5 is Saturday.
	if m.WeekdayHistogram[0] != 2 {
		t.Errorf("Monday count = %d, want 2", m.WeekdayHistogram[0])
	}
	if m.WeekdayHistogram[4] != 1 {
		t.Errorf("Friday count = %d, want 1", m.WeekdayHistogram[4])
	}
	if m.WeekdayHistogram[5] != 1 {
		t.Errorf("Saturday count = %d, want 1", m.WeekdayHistogram[5])
	}

	if *m.FridayEveningCount != 1 {
		t.Errorf("friday_evening_count = %d, want 1 (Friday 18:00 local)", *m.FridayEveningCount)
	}
	// 23:00 and 02:30 are night hours; 09:00 and 18:00 are not.
	if *m.NightRatio != 0.5 {
		t.Errorf("night_ratio = %v, want 0.5", *m.NightRatio)
	}
	if *m.WeekendRatio != 0.25 {
		t.Errorf("weekend_ratio = %v, want 0.25", *m.WeekendRatio)
	}
}

func TestBusiestDayBreaksTiesOnEarliestDate(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		at(2026, time.March, 2, 10, 0, 0),
		at(2026, time.March, 2, 11, 0, 0),
		at(2026, time.March, 5, 10, 0, 0),
		at(2026, time.March, 5, 11, 0, 0),
	}
	m := build(t, commits)
	if m.BusiestDay.Date != "2026-03-02" || m.BusiestDay.Count != 2 {
		t.Errorf("busiest_day = %+v, want 2026-03-02 with 2", *m.BusiestDay)
	}
}

func TestLongestStreakAndSilence(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		at(2026, time.April, 1, 10, 0, 0),
		at(2026, time.April, 2, 10, 0, 0),
		at(2026, time.April, 3, 10, 0, 0), // a three-day streak
		at(2026, time.April, 20, 10, 0, 0),
		at(2026, time.April, 21, 10, 0, 0),
	}
	m := build(t, commits)
	if *m.LongestStreakDays != 3 {
		t.Errorf("longest_streak_days = %d, want 3", *m.LongestStreakDays)
	}
	if *m.LongestSilenceDays != 17 {
		t.Errorf("longest_silence_days = %d, want 17", *m.LongestSilenceDays)
	}
}

func TestSingleDayRepositoryEdgeCase(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		at(2026, time.May, 4, 9, 0, 0),
		at(2026, time.May, 4, 17, 0, 0),
	}
	m := build(t, commits)
	if *m.LongestStreakDays != 1 {
		t.Errorf("longest_streak_days = %d, want 1", *m.LongestStreakDays)
	}
	if *m.LongestSilenceDays != 0 {
		t.Errorf("longest_silence_days = %d, want 0", *m.LongestSilenceDays)
	}
}

func TestStreakUsesLocalDatesNotUTC(t *testing.T) {
	t.Parallel()
	// 23:00 at +03 is 20:00 UTC the same day; 01:00 at +03 the next day is
	// 22:00 UTC on the first day. Counted in UTC these collapse onto one date
	// and the streak would be 1 rather than 2.
	commits := []model.Commit{
		at(2026, time.June, 1, 23, 0, 3),
		at(2026, time.June, 2, 1, 0, 3),
	}
	m := build(t, commits)
	if *m.LongestStreakDays != 2 {
		t.Errorf("longest_streak_days = %d, want 2 across local dates", *m.LongestStreakDays)
	}
}

// TestTemporalOnEmptyInput is docs/metrics.md section 1: a metric over an
// empty population is absent, with the family degraded, never zero.
func TestTemporalOnEmptyInput(t *testing.T) {
	t.Parallel()
	f := Build(core.Input{Config: config.Default()}, nil)
	if f.Status != core.StatusDegraded || len(f.Reasons) != 1 || f.Reasons[0] != core.ReasonEmptyPopulation {
		t.Errorf("family = %s %v, want degraded with empty_population", f.Status, f.Reasons)
	}
	if f.Metrics.HourHistogram != nil || f.Metrics.NightRatio != nil || f.Metrics.BusiestDay != nil {
		t.Errorf("metrics over no commits = %+v, want every metric absent", f.Metrics)
	}
}

func TestFirstAndLastCommitDatesAreLocal(t *testing.T) {
	t.Parallel()
	// 01:00 at +03 on 1 July is 30 June in UTC; 22:00 at -05 on 9 July is
	// 10 July in UTC.
	commits := []model.Commit{
		at(2026, time.July, 1, 1, 0, 3),
		at(2026, time.July, 9, 22, 0, -5),
	}
	m := build(t, commits)
	if *m.FirstCommitDate != "2026-07-01" || *m.LastCommitDate != "2026-07-09" {
		t.Errorf("first/last commit date = %s/%s, want 2026-07-01/2026-07-09",
			*m.FirstCommitDate, *m.LastCommitDate)
	}
}

func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}
