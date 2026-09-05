package aggregate

import (
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/model"
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

// testInput builds the minimal Input the metric builders need. The commit
// slice is passed to the builder directly, so it is only here for readability
// at the call sites.
func testInput(_ []model.Commit) Input {
	return Input{Config: config.Default()}
}

func TestTemporalHistogramsUseAuthorLocalTime(t *testing.T) {
	commits := []model.Commit{
		// 2026-01-05 is a Monday.
		at(2026, time.January, 5, 9, 0, 0),   // Mon 09:00 UTC
		at(2026, time.January, 5, 23, 0, 3),  // Mon 23:00 +03 -> still Monday locally
		at(2026, time.January, 9, 18, 0, -5), // Fri 18:00 -05, a brave deploy
		at(2026, time.January, 10, 2, 30, 0), // Sat 02:30, night owl and weekend
	}
	m := buildTemporal(testInput(commits), commits)

	if m.HourHistogram[9] != 1 || m.HourHistogram[23] != 1 ||
		m.HourHistogram[18] != 1 || m.HourHistogram[2] != 1 {
		t.Errorf("hour histogram = %v", m.HourHistogram)
	}
	if sum(m.HourHistogram) != len(commits) {
		t.Errorf("sum(hourHistogram) = %d, want %d", sum(m.HourHistogram), len(commits))
	}
	if sum(m.WeekdayHistogram) != len(commits) {
		t.Errorf("sum(weekdayHistogram) = %d, want %d", sum(m.WeekdayHistogram), len(commits))
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

	if m.HourWeekdayGrid[0][23] != 1 {
		t.Errorf("grid[Monday][23] = %d, want 1", m.HourWeekdayGrid[0][23])
	}
	total := 0
	for _, row := range m.HourWeekdayGrid {
		total += sum(row)
	}
	if total != len(commits) {
		t.Errorf("grid total = %d, want %d", total, len(commits))
	}

	if m.BraveDeploys != 1 {
		t.Errorf("braveDeploys = %d, want 1 (Friday 18:00 local)", m.BraveDeploys)
	}
	// 23:00 and 02:30 are night-owl hours; 09:00 and 18:00 are not.
	if m.NightOwlRatio != 0.5 {
		t.Errorf("nightOwlRatio = %v, want 0.5", m.NightOwlRatio)
	}
}

func TestBusiestDayBreaksTiesOnEarliestDate(t *testing.T) {
	commits := []model.Commit{
		at(2026, time.March, 2, 10, 0, 0),
		at(2026, time.March, 2, 11, 0, 0),
		at(2026, time.March, 5, 10, 0, 0),
		at(2026, time.March, 5, 11, 0, 0),
	}
	m := buildTemporal(testInput(commits), commits)
	if m.BusiestDay == nil {
		t.Fatal("busiestDay is nil")
	}
	if m.BusiestDay.Date != "2026-03-02" || m.BusiestDay.Count != 2 {
		t.Errorf("busiestDay = %+v, want 2026-03-02 with 2", *m.BusiestDay)
	}
}

func TestLongestStreakAndSilence(t *testing.T) {
	commits := []model.Commit{
		at(2026, time.April, 1, 10, 0, 0),
		at(2026, time.April, 2, 10, 0, 0),
		at(2026, time.April, 3, 10, 0, 0), // a three-day streak
		at(2026, time.April, 20, 10, 0, 0),
		at(2026, time.April, 21, 10, 0, 0),
	}
	m := buildTemporal(testInput(commits), commits)

	if m.LongestStreak == nil || m.LongestStreak.Days != 3 {
		t.Fatalf("longestStreak = %+v, want 3 days", m.LongestStreak)
	}
	if m.LongestStreak.StartDate != "2026-04-01" || m.LongestStreak.EndDate != "2026-04-03" {
		t.Errorf("streak span = %s..%s, want 2026-04-01..2026-04-03",
			m.LongestStreak.StartDate, m.LongestStreak.EndDate)
	}
	if m.LongestSilence == nil || m.LongestSilence.Days != 17 {
		t.Fatalf("longestSilence = %+v, want 17 days", m.LongestSilence)
	}
	if m.LongestSilence.StartDate != "2026-04-03" || m.LongestSilence.EndDate != "2026-04-20" {
		t.Errorf("silence span = %s..%s, want 2026-04-03..2026-04-20",
			m.LongestSilence.StartDate, m.LongestSilence.EndDate)
	}
}

func TestSingleDayRepositoryEdgeCase(t *testing.T) {
	commits := []model.Commit{
		at(2026, time.May, 4, 9, 0, 0),
		at(2026, time.May, 4, 17, 0, 0),
	}
	m := buildTemporal(testInput(commits), commits)
	if m.LongestStreak == nil || m.LongestStreak.Days != 1 {
		t.Errorf("longestStreak = %+v, want 1 day", m.LongestStreak)
	}
	if m.LongestSilence == nil || m.LongestSilence.Days != 0 {
		t.Errorf("longestSilence = %+v, want 0 days", m.LongestSilence)
	}
}

func TestStreakUsesLocalDatesNotUTC(t *testing.T) {
	// 23:00 at +03 is 20:00 UTC the same day; 01:00 at +03 the next day is
	// 22:00 UTC on the first day. Counted in UTC these collapse onto one date
	// and the streak would be 1 rather than 2.
	commits := []model.Commit{
		at(2026, time.June, 1, 23, 0, 3),
		at(2026, time.June, 2, 1, 0, 3),
	}
	m := buildTemporal(testInput(commits), commits)
	if m.LongestStreak == nil || m.LongestStreak.Days != 2 {
		t.Errorf("longestStreak = %+v, want 2 days across local dates", m.LongestStreak)
	}
}

func TestCommitsPerMonthZeroFillsGaps(t *testing.T) {
	commits := []model.Commit{
		at(2025, time.November, 1, 10, 0, 0),
		at(2026, time.February, 1, 10, 0, 0),
		at(2026, time.February, 2, 10, 0, 0),
	}
	m := buildTemporal(testInput(commits), commits)

	want := []MonthCount{
		{"2025-11", 1}, {"2025-12", 0}, {"2026-01", 0}, {"2026-02", 2},
	}
	if len(m.CommitsPerMonth) != len(want) {
		t.Fatalf("commitsPerMonth = %v, want %v", m.CommitsPerMonth, want)
	}
	for i, w := range want {
		if m.CommitsPerMonth[i] != w {
			t.Errorf("month %d = %+v, want %+v", i, m.CommitsPerMonth[i], w)
		}
	}
}

func TestTemporalOnEmptyInput(t *testing.T) {
	m := buildTemporal(testInput(nil), nil)
	if len(m.HourHistogram) != 24 || len(m.WeekdayHistogram) != 7 || len(m.HourWeekdayGrid) != 7 {
		t.Error("histograms must keep their fixed shape even with no commits")
	}
	if m.BusiestDay != nil || m.LongestStreak != nil || m.LongestSilence != nil {
		t.Error("absent metrics must be null, not zero-valued")
	}
}

func TestFirstAndLastCommitPreserveOffsets(t *testing.T) {
	commits := []model.Commit{
		at(2026, time.July, 1, 12, 0, 3),
		at(2026, time.July, 9, 12, 0, -5),
	}
	m := buildTemporal(testInput(commits), commits)
	if _, offset := m.FirstCommit.Zone(); offset != 3*3600 {
		t.Errorf("firstCommit offset = %d, want %d", offset, 3*3600)
	}
	if _, offset := m.LastCommit.Zone(); offset != -5*3600 {
		t.Errorf("lastCommit offset = %d, want %d", offset, -5*3600)
	}
}

func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}
