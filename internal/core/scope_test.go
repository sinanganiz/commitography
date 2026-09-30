package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// TestScopeCellIdentityIsWrittenAsTheIdentitiesSectionWritesIt is the JSON
// form of a cell identity (ADR-0078 clause 3): an id, or the aggregate flag,
// named as the identities section names them; embedded in a cell keyed by one
// identity, and under a name of its own in a cell keyed by two.
func TestScopeCellIdentityIsWrittenAsTheIdentitiesSectionWritesIt(t *testing.T) {
	t.Parallel()
	type oneKey struct {
		CellIdentity
		Year int `json:"year"`
	}
	type twoKeys struct {
		Editor CellIdentity `json:"editor"`
		Owner  CellIdentity `json:"owner"`
		Year   int          `json:"year"`
	}
	for _, c := range []struct {
		name  string
		value any
		want  string
	}{
		{"an identity", CellIdentity{ID: "b5fc85e55755f9e0"}, `{"id":"b5fc85e55755f9e0"}`},
		{"the aggregate bucket", CellIdentity{Aggregate: true}, `{"aggregate":true}`},
		{"a cell of one identity", oneKey{CellIdentity{ID: "b5fc85e55755f9e0"}, 2026}, `{"id":"b5fc85e55755f9e0","year":2026}`},
		{"a cell of the bucket", oneKey{CellIdentity{Aggregate: true}, 2025}, `{"aggregate":true,"year":2025}`},
		{"a cell of two identities", twoKeys{CellIdentity{ID: "b5fc85e55755f9e0"}, CellIdentity{Aggregate: true}, 2026},
			`{"editor":{"id":"b5fc85e55755f9e0"},"owner":{"aggregate":true},"year":2026}`},
	} {
		encoded, err := json.Marshal(c.value)
		if err != nil || string(encoded) != c.want {
			t.Errorf("%s encodes as %s (%v), want %s", c.name, encoded, err, c.want)
		}
	}
	var decoded oneKey
	if err := json.Unmarshal([]byte(`{"aggregate":true,"year":2025}`), &decoded); err != nil ||
		decoded.CellIdentity != (CellIdentity{Aggregate: true}) || decoded.Year != 2025 {
		t.Errorf("decoding a cell of the bucket gave %+v (%v)", decoded, err)
	}
}

// boundedTable returns a table with one identity more than the bound, and an
// individual and the folded identity of it.
func boundedTable() (table *IdentityTable, individual, folded string) {
	var commits []model.Commit
	for i := 0; i <= LimitIdentities; i++ {
		id := fmt.Sprintf("%016x", i)
		commits = append(commits, analysedOn(id, day(i)))
		if i > 0 {
			commits = append(commits, analysedOn(id, day(i)))
		}
	}
	return NewIdentityTable(commits), fmt.Sprintf("%016x", 1), fmt.Sprintf("%016x", 0)
}

// TestScopeTableKeysACellByTheIdentityOrTheBucket is the fold every family
// applies (ADR-0078 clause 9): an individual identity keys its own cells, and
// a folded identity, or one with no analysed commit, the aggregate bucket's.
func TestScopeTableKeysACellByTheIdentityOrTheBucket(t *testing.T) {
	t.Parallel()
	table, individual, folded := boundedTable()
	bucket := CellIdentity{Aggregate: true}
	for id, want := range map[string]CellIdentity{
		individual:         {ID: individual},
		folded:             bucket,
		"ffffffffffffffff": bucket,
	} {
		if got := table.Cell(id); got != want {
			t.Errorf("the cell of %s is keyed by %+v, want %+v", id, got, want)
		}
	}
	var missing *IdentityTable
	if got := missing.Cell(individual); got != bucket {
		t.Errorf("a missing table keys a cell by %+v", got)
	}
}

// TestScopeZeroValueIsEveryIdentityAndEveryYear keeps the repository's own
// scope the zero value (ADR-0078 clause 1).
func TestScopeZeroValueIsEveryIdentityAndEveryYear(t *testing.T) {
	t.Parallel()
	var s Scope
	for _, c := range []CellIdentity{{ID: "b5fc85e55755f9e0"}, {Aggregate: true}} {
		if !s.IncludesIdentity(c) {
			t.Errorf("the zero scope leaves out %+v", c)
		}
	}
	for _, year := range []int{1970, 2026, 10000} {
		if !s.IncludesYear(year) {
			t.Errorf("the zero scope leaves out %d", year)
		}
	}
	if got := s.String(); got != "every identity, every year" {
		t.Errorf("the zero scope reads %q", got)
	}
}

// TestScopeConstructorRefusesWhatNoSelectionNames is ADR-0078 clauses 1 and
// 9: a selection names individually represented identities, each once, and
// never the aggregate bucket. A refusal quotes no value that is not an
// identity digest, since a reader may have supplied it (ADR-0041 clause 5).
func TestScopeConstructorRefusesWhatNoSelectionNames(t *testing.T) {
	t.Parallel()
	table, individual, folded := boundedTable()
	for _, c := range []struct {
		name      string
		selection []CellIdentity
		want      string
	}{
		{"the aggregate bucket", []CellIdentity{{ID: individual}, {Aggregate: true}}, "aggregate bucket"},
		{"the bucket beside an id", []CellIdentity{{ID: individual, Aggregate: true}}, "aggregate bucket"},
		{"no id, as the aggregate entry has", []CellIdentity{{}}, "aggregate bucket"},
		{"a folded identity", []CellIdentity{{ID: folded}}, folded + ", which folds into the aggregate bucket"},
		{"an identity with no analysed commit", []CellIdentity{{ID: "ffffffffffffffff"}}, "has no analysed commit"},
		{"an address", []CellIdentity{{ID: "someone@example.com"}}, "not an identity digest"},
		{"one identity twice", []CellIdentity{{ID: individual}, {ID: individual}}, individual + " twice"},
	} {
		_, err := NewScope(table, c.selection, 0)
		switch {
		case err == nil:
			t.Errorf("a selection of %s was accepted", c.name)
		case ClassOf(err) != ClassInternal || !strings.Contains(err.Error(), c.want):
			t.Errorf("a selection of %s was refused as %v (%s), want an internal error naming %q",
				c.name, err, ClassOf(err), c.want)
		case strings.Contains(err.Error(), "@"):
			t.Errorf("refusing a selection of %s quoted the address: %v", c.name, err)
		}
	}
}

// TestScopeSelectionNeverIncludesTheAggregateBucket is ADR-0078 clause 9: a
// selection includes the identities it names, in whatever order it names
// them, and no other cell identity, the aggregate bucket least of all; a year
// includes its own cells alone.
func TestScopeSelectionNeverIncludesTheAggregateBucket(t *testing.T) {
	t.Parallel()
	table, individual, folded := boundedTable()
	other := fmt.Sprintf("%016x", 2)
	s, err := NewScope(table, []CellIdentity{{ID: other}, {ID: individual}}, 2026)
	if err != nil {
		t.Fatalf("a selection of two individual identities was refused: %v", err)
	}
	for c, want := range map[CellIdentity]bool{
		{ID: individual}:              true,
		{ID: other}:                   true,
		{ID: fmt.Sprintf("%016x", 3)}: false,
		{ID: folded}:                  false,
		{Aggregate: true}:             false,
	} {
		if got := s.IncludesIdentity(c); got != want {
			t.Errorf("%s includes %+v: %t, want %t", s, c, got, want)
		}
	}
	if !s.IncludesYear(2026) || s.IncludesYear(2025) {
		t.Errorf("%s includes 2026: %t and 2025: %t, want true and false", s, s.IncludesYear(2026), s.IncludesYear(2025))
	}
	if want := "{" + individual + ", " + other + "}, 2026"; s.String() != want {
		t.Errorf("the scope reads %q, want %q", s.String(), want)
	}

	every, err := NewScope(table, nil, 2025)
	if err != nil || !every.IncludesIdentity(CellIdentity{Aggregate: true}) || every.IncludesYear(2026) {
		t.Errorf("the scope of every identity in 2025 = %s (%v), want the bucket included and 2026 left out", every, err)
	}
}

// TestScopeCommitYearIsTheActiveDatesYear is ADR-0078 clause 8: a commit's
// figures belong to the year of the active date its record carries, however
// many digits the year has, and a record without one belongs to no year.
func TestScopeCommitYearIsTheActiveDatesYear(t *testing.T) {
	t.Parallel()
	for date, want := range map[string]int{
		"2026-01-01":  2026,
		"1969-12-31":  1969,
		"10000-01-01": 10000,
		"":            0,
		"2026-1-01":   0,
		"2026-01-xx":  0,
		"year-01-01":  0,
		"2026/01/01":  0,
	} {
		if got := CommitYear(model.Commit{ActiveDate: date}); got != want {
			t.Errorf("the year of %q is %d, want %d", date, got, want)
		}
	}
	// The year is the active date's, not the timestamp's: the collect stage
	// decided the date, by the configured source and the recorded offset.
	c := model.Commit{AuthorDate: time.Date(2025, time.December, 31, 23, 0, 0, 0, time.UTC), ActiveDate: "2026-01-01"}
	if got := CommitYear(c); got != 2026 {
		t.Errorf("the year of a commit dated 2026-01-01 is %d", got)
	}
}

// TestScopeRestrictToYearKeepsTheWholeAnalysissBound restricts an input to a
// year: the other years' commits are not analysed, the counts follow, the
// table is the one the input carried, and the input restricted is unchanged.
func TestScopeRestrictToYearKeepsTheWholeAnalysissBound(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		analysedOn("a", time.Date(2025, time.December, 31, 12, 0, 0, 0, time.UTC)),
		analysedOn("a", time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)),
		analysedOn("b", time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)),
		analysedOn("b", time.Date(2025, time.June, 1, 12, 0, 0, 0, time.UTC)),
	}
	commits[2].IsBulk = true
	commits[3].Excluded, commits[3].MergeExcluded = true, true
	in := Input{Filtered: filter.Summarize(commits)}
	in.Identities = NewIdentityTable(in.Analyzed())

	restricted := in.RestrictToYear(2026)
	if got := restricted.Analyzed(); len(got) != 2 || got[0].ActiveDate != "2026-01-01" || got[1].ActiveDate != "2026-06-01" {
		t.Errorf("2026's analysed commits are %+v, want those of 2026-01-01 and 2026-06-01", got)
	}
	if got := restricted.LineScoped(); len(got) != 1 || got[0].ActiveDate != "2026-01-01" {
		t.Errorf("2026's line-scoped commits are %+v, want the one that is not bulk", got)
	}
	if f := restricted.Filtered; f.AnalyzedCommits != 2 || len(f.BulkCommits) != 1 || f.ExcludedMerges != 1 || f.TotalCommits != 4 {
		t.Errorf("2026's counts are %+v, want 2 analysed of 4, one bulk and one merge left out", f)
	}
	if restricted.Identities != in.Identities {
		t.Error("restricting to a year recomputed the identity table; the bound is the whole analysis's")
	}
	if got := in.Analyzed(); len(got) != 3 || in.Filtered.AnalyzedCommits != 3 {
		t.Errorf("restricting to a year changed the input it restricted: %d analysed", len(got))
	}
	if got := in.RestrictToYear(2024).Analyzed(); len(got) != 0 {
		t.Errorf("a year with no commit has %d analysed", len(got))
	}
}
