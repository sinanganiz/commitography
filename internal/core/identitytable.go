// The identity bound (ADR-0018 clause 4, ADR-0078 clause 9): which identities
// the report represents individually, selected once, here, and folded through
// by every part of the report that lists identities. docs/metrics.md section
// 14 states the selection rule and section 12 the limit.

package core

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core/model"
)

// IdentityTable is an analysis's identity table: every identity with an
// analysed commit, with its analysed commit count and its first and last
// analysed local dates, and which of them are represented individually. Every
// other identity folds into the one aggregate bucket.
//
// The aggregate stage computes it once per build, before any family runs, and
// gives every family the same table, so the identities section and every
// family's cells fold through one selection and an identity is individual in
// every part of the report or in none (ADR-0078 clause 9). Nothing outside
// core selects the individually represented identities.
type IdentityTable struct {
	// rows is every identity, ordered by id.
	rows []IdentityRow
	// index maps an id to its position in rows.
	index map[string]int
}

// IdentityRow is one identity of the table.
type IdentityRow struct {
	// ID is the identity's digest (docs/metrics.md section 14, `id`).
	ID string
	// CommitCount is the number of the identity's analysed commits.
	CommitCount int
	// FirstCommitDate and LastCommitDate are the local dates of the
	// identity's earliest and latest analysed commits, as the collect stage
	// decided them: each commit's ActiveDate, never recomputed here.
	FirstCommitDate string
	LastCommitDate  string
	// Individual marks an identity the report represents individually.
	Individual bool
}

// NewIdentityTable computes the identity table of the analysed commits.
//
// An identity's earliest commit is the one attributed to the earliest instant,
// its LocalTime, and its first date is that commit's ActiveDate; its latest
// commit and last date likewise, the first commit met winning a tie. That is
// how the identities section has always dated an identity. Taking the least
// and greatest ActiveDate instead differs only where two of one identity's
// commits carry different offsets, and would change the section's derivation,
// which its version would then have to record (ADR-0070 clause 1).
//
// The individually represented identities are those with the most analysed
// commits, ties broken by the earlier first date and then by the smaller id,
// up to the individually represented identities limit (docs/metrics.md
// sections 12 and 14).
func NewIdentityTable(analysed []model.Commit) *IdentityTable {
	type span struct {
		row         IdentityRow
		first, last time.Time
	}
	spans := map[string]*span{}
	for _, c := range analysed {
		s, ok := spans[c.IdentityID]
		if !ok {
			s = &span{
				row:   IdentityRow{ID: c.IdentityID, FirstCommitDate: c.ActiveDate, LastCommitDate: c.ActiveDate},
				first: c.LocalTime,
				last:  c.LocalTime,
			}
			spans[c.IdentityID] = s
		}
		s.row.CommitCount++
		if c.LocalTime.Before(s.first) {
			s.first, s.row.FirstCommitDate = c.LocalTime, c.ActiveDate
		}
		if c.LocalTime.After(s.last) {
			s.last, s.row.LastCommitDate = c.LocalTime, c.ActiveDate
		}
	}

	rows := make([]IdentityRow, 0, len(spans))
	for _, s := range spans {
		rows = append(rows, s.row)
	}
	// The bound selects by activity; nothing the table hands out is ordered
	// by it (ADR-0009 clause 3).
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.CommitCount != b.CommitCount {
			return a.CommitCount > b.CommitCount
		}
		if a.FirstCommitDate != b.FirstCommitDate {
			return a.FirstCommitDate < b.FirstCommitDate
		}
		return a.ID < b.ID
	})
	for i := range rows {
		rows[i].Individual = i < LimitIdentities
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	index := make(map[string]int, len(rows))
	for i, r := range rows {
		index[r.ID] = i
	}
	return &IdentityTable{rows: rows, index: index}
}

// Rows returns every identity of the table, ordered by id. The slice is a
// copy.
func (t *IdentityTable) Rows() []IdentityRow {
	if t == nil {
		return nil
	}
	return append([]IdentityRow(nil), t.rows...)
}

// Lookup returns the row of the identity with the given id, and whether the
// table holds one: an identity with no analysed commit has none.
func (t *IdentityTable) Lookup(id string) (IdentityRow, bool) {
	if t == nil {
		return IdentityRow{}, false
	}
	i, ok := t.index[id]
	if !ok {
		return IdentityRow{}, false
	}
	return t.rows[i], true
}

// Individual reports whether the identity with the given id is represented
// individually. Every other identity, one without an analysed commit among
// them, folds into the aggregate bucket.
func (t *IdentityTable) Individual(id string) bool {
	row, ok := t.Lookup(id)
	return ok && row.Individual
}

// Cell returns the identity a cell of the identity with the given id is keyed
// by: the identity itself where the table represents it individually, and the
// aggregate bucket otherwise (ADR-0078 clause 3).
func (t *IdentityTable) Cell(id string) CellIdentity {
	if t.Individual(id) {
		return CellIdentity{ID: id}
	}
	return CellIdentity{Aggregate: true}
}
