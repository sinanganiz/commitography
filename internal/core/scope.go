// Scopes and cells (ADR-0078): the identity selection and year a scoped figure
// is computed for, and the identity and year a family keys each of its cells
// by. docs/metrics.md section 1 defines both.

package core

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// CellIdentity is the identity a cell is keyed by (ADR-0078 clause 3): an
// individually represented identity, by its id, or the aggregate bucket that
// folds every other. Its JSON form is {"id": "<id>"} or {"aggregate": true},
// the fields named as the identities section names them. A cell keyed by one
// identity embeds it, so that the cell reads {"id": …, "year": …}; a cell
// keyed by two names each under a field of its own.
type CellIdentity struct {
	ID        string `json:"id,omitzero"`
	Aggregate bool   `json:"aggregate,omitzero"`
}

// Scope is what a scoped figure is computed for (ADR-0078 clause 1): an
// identity selection and a year. The selection is every identity, or a set of
// individually represented identities named by id; the year is every year, or
// one calendar year. The zero value is every identity and every year, the
// scope of the repository's own figures. Any other scope comes from NewScope,
// which validates it.
type Scope struct {
	// selection is the ids of the selected identities, in id order, or empty
	// for every identity.
	selection []string
	// year is the one calendar year, or 0 for every year. No commit is dated
	// in year 0, since git dates commits in seconds from 1970.
	year int
}

// NewScope returns the scope of the identities selection names and of year,
// validated against the identity table of the analysis it applies to. An
// empty selection is every identity, and a year of 0 every year.
//
// It refuses a selection that names the aggregate bucket, whether by its flag
// or by naming no id, as the identities section's aggregate entry does; an
// identity the table does not represent individually; and an identity named
// twice (ADR-0078 clauses 1 and 9). A refusal is an internal error: no reason
// code describes a selection, so a caller that takes one from a reader states
// the user error its own interface defines.
func NewScope(table *IdentityTable, selection []CellIdentity, year int) (Scope, error) {
	ids := make([]string, 0, len(selection))
	named := make(map[string]bool, len(selection))
	for _, c := range selection {
		switch {
		case c.Aggregate || c.ID == "":
			return Scope{}, Internalf(nil, "a scope names the aggregate bucket; a selection names individually "+
				"represented identities alone (ADR-0078 clause 9)")
		case !table.Individual(c.ID):
			why := "has no analysed commit"
			if _, ok := table.Lookup(c.ID); ok {
				why = "folds into the aggregate bucket"
			}
			return Scope{}, Internalf(nil, "a scope names %s, which %s; a selection names individually represented "+
				"identities alone (ADR-0078 clauses 1 and 9)", nameable(c.ID), why)
		case named[c.ID]:
			return Scope{}, Internalf(nil, "a scope names the identity %s twice", c.ID)
		}
		named[c.ID] = true
		ids = append(ids, c.ID)
	}
	sort.Strings(ids)
	return Scope{selection: ids, year: year}, nil
}

// nameable returns an id as an error message may carry it. A caller may have
// taken the id from a reader, and a message may carry no address (ADR-0041
// clause 5), so only a value shaped as an identity digest is quoted.
func nameable(id string) string {
	if identity.IsReference(id) {
		return "the identity " + id
	}
	return "an identity by a value that is not an identity digest"
}

// IncludesIdentity reports whether cells keyed by the identity c enter the
// scope. A scope of every identity includes every cell identity, the
// aggregate bucket among them; a selection includes the identities it names
// and never the aggregate bucket (ADR-0078 clause 9).
func (s Scope) IncludesIdentity(c CellIdentity) bool {
	if len(s.selection) == 0 {
		return true
	}
	if c.Aggregate {
		return false
	}
	i := sort.SearchStrings(s.selection, c.ID)
	return i < len(s.selection) && s.selection[i] == c.ID
}

// IncludesYear reports whether cells of the given year enter the scope: every
// year does in a scope of every year, and its own year alone in a scope of
// one.
func (s Scope) IncludesYear(year int) bool {
	return s.year == 0 || s.year == year
}

// String describes the scope, for a diagnostic.
func (s Scope) String() string {
	who := "every identity"
	if len(s.selection) > 0 {
		who = "{" + strings.Join(s.selection, ", ") + "}"
	}
	when := "every year"
	if s.year != 0 {
		when = strconv.Itoa(s.year)
	}
	return who + ", " + when
}

// CommitYear returns the year a commit's figures belong to (ADR-0078
// clause 8): the calendar year of the record's active date, which the collect
// stage decided by the configured date source. A record without a well-formed
// active date, which the collect stage never writes, belongs to year 0, which
// no scope of one year includes.
func CommitYear(c model.Commit) int {
	// The date is written as Go writes one, the year in at least four digits,
	// so a year past 9999 has more: the year is everything before the month.
	d := c.ActiveDate
	n := len(d)
	if n < len("0000-01-01") || d[n-6] != '-' || d[n-3] != '-' ||
		!digits(d[n-5:n-3]) || !digits(d[n-2:]) {
		return 0
	}
	year, err := strconv.Atoi(d[:n-6])
	if err != nil {
		return 0
	}
	return year
}

// digits reports whether s is decimal digits only.
func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
