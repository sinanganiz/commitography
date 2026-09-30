package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core/model"
)

// analysedOn returns an analysed commit of identity id, attributed to at, with
// the active date the collect stage would give it.
func analysedOn(id string, at time.Time) model.Commit {
	return model.Commit{IdentityID: id, LocalTime: at, ActiveDate: at.Format("2006-01-02")}
}

// day returns noon UTC on a day of 2026.
func day(n int) time.Time {
	return time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n)
}

// TestScopeTableRepresentsTheMostActiveIndividually is the bound of
// docs/metrics.md section 14: the identities with the most analysed commits,
// up to the limit, are individual, and every other folds.
func TestScopeTableRepresentsTheMostActiveIndividually(t *testing.T) {
	t.Parallel()
	const extra = 5
	var commits []model.Commit
	for i := 0; i < LimitIdentities+extra; i++ {
		id := fmt.Sprintf("%016x", i)
		// The first identities have one commit, every other identity two, so
		// the least active fold whatever their dates.
		commits = append(commits, analysedOn(id, day(i)))
		if i >= extra {
			commits = append(commits, analysedOn(id, day(400)))
		}
	}
	table := NewIdentityTable(commits)
	individual := 0
	for _, row := range table.Rows() {
		wantIndividual := row.CommitCount == 2
		if row.Individual != wantIndividual {
			t.Errorf("%s with %d commits: individual %t, want %t", row.ID, row.CommitCount, row.Individual, wantIndividual)
		}
		if row.Individual {
			individual++
		}
	}
	if individual != LimitIdentities || len(table.Rows()) != LimitIdentities+extra {
		t.Errorf("%d of %d identities are individual, want %d of %d", individual, len(table.Rows()),
			LimitIdentities, LimitIdentities+extra)
	}
}

// TestScopeTableBreaksTiesByFirstDateThenID is the tie rule of section 14:
// among identities with equal commit counts, the earlier first date is
// individual before the later, and then the smaller id before the larger.
func TestScopeTableBreaksTiesByFirstDateThenID(t *testing.T) {
	t.Parallel()
	var commits []model.Commit
	for i := 0; i < LimitIdentities-1; i++ {
		commits = append(commits, analysedOn(fmt.Sprintf("a%015x", i), day(0)))
	}
	// Three identities contend for the last place: two starting on day 1,
	// and one starting later.
	commits = append(commits,
		analysedOn("f000000000000000", day(1)),
		analysedOn("e000000000000000", day(1)),
		analysedOn("0000000000000000", day(2)))
	table := NewIdentityTable(commits)
	for id, want := range map[string]bool{
		"e000000000000000": true,  // day 1, the smaller id
		"f000000000000000": false, // day 1, the larger id
		"0000000000000000": false, // the smallest id, but the latest start
	} {
		if got := table.Individual(id); got != want {
			t.Errorf("%s individual = %t, want %t", id, got, want)
		}
	}
}

// TestScopeTableDatesAnIdentityByItsEarliestAndLatestCommits holds the table
// to the identities section's dating: the ActiveDate of the commit attributed
// to the earliest instant, and of the latest, as the collect stage wrote them.
// Two commits whose offsets put their local dates in the other order show
// that the instant chooses the commit and the record gives its date.
func TestScopeTableDatesAnIdentityByItsEarliestAndLatestCommits(t *testing.T) {
	t.Parallel()
	east, west := time.FixedZone("", 14*3600), time.FixedZone("", -10*3600)
	earlier := time.Date(2026, time.March, 1, 11, 0, 0, 0, time.UTC).In(east) // local 2026-03-02
	later := time.Date(2026, time.March, 2, 6, 0, 0, 0, time.UTC).In(west)    // local 2026-03-01
	middle := analysedOn("a", time.Date(2026, time.March, 1, 20, 0, 0, 0, time.UTC))
	// The record's ActiveDate is what the table reports, never a date it
	// recomputes from the timestamp.
	middle.ActiveDate = "2026-03-09"

	row, ok := NewIdentityTable([]model.Commit{middle, analysedOn("a", later), analysedOn("a", earlier)}).Lookup("a")
	if !ok {
		t.Fatal("the identity is not in the table")
	}
	if row.FirstCommitDate != "2026-03-02" || row.LastCommitDate != "2026-03-01" || row.CommitCount != 3 {
		t.Errorf("row = %+v, want first 2026-03-02 and last 2026-03-01, the earliest and latest instants' dates, "+
			"and 3 commits", row)
	}

	row, _ = NewIdentityTable([]model.Commit{middle}).Lookup("a")
	if row.FirstCommitDate != "2026-03-09" || row.LastCommitDate != "2026-03-09" {
		t.Errorf("row = %+v, want the record's own active date", row)
	}
}

// TestScopeTableAnswersForEveryID answers for an individual identity, a
// folded one and one with no analysed commit, and a missing table answers as
// an empty one rather than failing.
func TestScopeTableAnswersForEveryID(t *testing.T) {
	t.Parallel()
	var commits []model.Commit
	for i := 0; i <= LimitIdentities; i++ {
		id := fmt.Sprintf("%016x", i)
		commits = append(commits, analysedOn(id, day(i)))
		if i > 0 {
			commits = append(commits, analysedOn(id, day(i)))
		}
	}
	table := NewIdentityTable(commits)
	individual, folded := fmt.Sprintf("%016x", 1), fmt.Sprintf("%016x", 0)
	if !table.Individual(individual) || table.Individual(folded) || table.Individual("ffffffffffffffff") {
		t.Errorf("individual: %t, %t and %t, want true, false and false", table.Individual(individual),
			table.Individual(folded), table.Individual("ffffffffffffffff"))
	}
	if row, ok := table.Lookup(folded); !ok || row.Individual || row.CommitCount != 1 {
		t.Errorf("the folded identity's row = %+v, %t; want its one commit, folded", row, ok)
	}
	if _, ok := table.Lookup("ffffffffffffffff"); ok {
		t.Error("an identity with no analysed commit has a row")
	}

	var missing *IdentityTable
	if missing.Rows() != nil || missing.Individual(individual) {
		t.Error("a missing table answers as though it held identities")
	}
	if rows := NewIdentityTable(nil).Rows(); len(rows) != 0 {
		t.Errorf("the table of no commit holds %v", rows)
	}
}

// TestScopeTableRowsAreOrderedByID keeps the table from handing out a list
// ordered by activity (ADR-0009 clause 3), and its rows from being shared.
func TestScopeTableRowsAreOrderedByID(t *testing.T) {
	t.Parallel()
	table := NewIdentityTable([]model.Commit{
		analysedOn("c", day(0)), analysedOn("a", day(1)), analysedOn("b", day(2)), analysedOn("b", day(3)),
	})
	rows := table.Rows()
	if len(rows) != 3 || rows[0].ID != "a" || rows[1].ID != "b" || rows[2].ID != "c" {
		t.Fatalf("rows = %+v, want a, b and c in that order", rows)
	}
	rows[0].Individual = false
	if !table.Individual("a") {
		t.Error("changing a returned row changed the table")
	}
}
