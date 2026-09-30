package checks

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
)

// The cell catalogue checker (ADR-0078 clauses 2 and 3, ADR-0062, WP-0063).
// docs/metrics.md section 1 gives a family section with a scoped metric two
// conventions, and this holds every family section to them, in both
// directions, and the report's metric types to the tables:
//
//   - a Metric table's Scope column, where present, holds repository, person,
//     year or "person, year" on every row;
//   - a family marking any metric person or year has a table headed Cell
//     field, and a family with one marks a metric so;
//   - a family with a Cell field table lists cells in its Metric table, and
//     its metric type has a cells field whose element's JSON names are the
//     table's fields exactly, an embedded struct's fields counting under the
//     names they are promoted to; a family without one carries no cells.

// scopeValues are the values a Scope column may hold.
func scopeValues() map[string]bool {
	return map[string]bool{"repository": true, "person": true, "year": true, "person, year": true}
}

// catalogueMetricRow is one row of a family's Metric table: the metrics its
// first column names, and its Scope value where the table has the column.
type catalogueMetricRow struct {
	metrics []string
	scope   string
	scoped  bool
}

// catalogueCellSection is what one family section of the catalogue says
// about scopes and cells.
type catalogueCellSection struct {
	rows []catalogueMetricRow
	// cellFields is the fields its Cell field table defines, nil where the
	// section has no such table.
	cellFields map[string]bool
}

// tableCells splits a table row into its cells, trimmed, an escaped pipe
// staying inside its cell.
func tableCells(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	var out []string
	for _, cell := range strings.Split(strings.ReplaceAll(row, `\|`, "\x00"), "|") {
		out = append(out, strings.TrimSpace(strings.ReplaceAll(cell, "\x00", "|")))
	}
	return out
}

// catalogueCellSections returns, for every family section of the catalogue,
// its Metric tables' rows and its Cell field table. A table is a run of lines
// beginning with a pipe: its first line is the header and its second the
// separator.
func catalogueCellSections(content string) map[string]catalogueCellSection {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	heading := regexp.MustCompile("(?m)^## [0-9]+\\. `([a-z][a-z-]*)`\\s*$")
	name := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	names := func(cell string) []string {
		var out []string
		for _, m := range name.FindAllStringSubmatch(cell, -1) {
			out = append(out, m[1])
		}
		return out
	}

	out := map[string]catalogueCellSection{}
	for _, m := range heading.FindAllStringSubmatchIndex(content, -1) {
		body := content[m[1]:]
		if next := strings.Index(body, "\n## "); next >= 0 {
			body = body[:next]
		}
		var section catalogueCellSection
		var table [][]string
		flush := func() {
			if len(table) < 2 {
				table = nil
				return
			}
			header, rows := table[0], table[2:]
			switch header[0] {
			case "Metric":
				scope := -1
				for i, h := range header {
					if h == "Scope" {
						scope = i
					}
				}
				for _, cells := range rows {
					row := catalogueMetricRow{metrics: names(cells[0]), scoped: scope >= 0}
					if scope >= 0 && scope < len(cells) {
						row.scope = cells[scope]
					}
					section.rows = append(section.rows, row)
				}
			case "Cell field":
				if section.cellFields == nil {
					section.cellFields = map[string]bool{}
				}
				for _, cells := range rows {
					for _, n := range names(cells[0]) {
						section.cellFields[n] = true
					}
				}
			}
			table = nil
		}
		for _, line := range strings.Split(body, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "|") {
				flush()
				continue
			}
			table = append(table, tableCells(line))
		}
		flush()
		out[content[m[2]:m[3]]] = section
	}
	return out
}

// cellFieldNames returns the JSON names a cell type writes, an embedded
// struct's fields counting under the names they are promoted to, as
// encoding/json promotes them.
func cellFieldNames(cell reflect.Type) map[string]bool {
	out := map[string]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			typ := field.Type
			if typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
			}
			if field.Anonymous && name == "" && typ.Kind() == reflect.Struct {
				walk(typ)
				continue
			}
			if !field.IsExported() {
				continue
			}
			if name == "" {
				name = field.Name
			}
			out[name] = true
		}
	}
	walk(cell)
	return out
}

// cellsElement returns the type of one cell of a metric type: the element of
// its cells field. found is false where the type has no cells field, and
// problem describes a cells field that is not a list of cells.
func cellsElement(metrics reflect.Type) (cell reflect.Type, found bool, problem string) {
	for i := 0; i < metrics.NumField(); i++ {
		field := metrics.Field(i)
		if jsonName(field) != "cells" {
			continue
		}
		typ := field.Type
		if typ.Kind() != reflect.Slice && typ.Kind() != reflect.Array {
			return nil, true, fmt.Sprintf("is a %s rather than a list of cells", typ)
		}
		typ = typ.Elem()
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			return nil, true, fmt.Sprintf("is a list of %s rather than of cells", typ)
		}
		return typ, true, ""
	}
	return nil, false, ""
}

// cellViolation is one failure, with the record it breaks.
type cellViolation struct {
	record int
	text   string
}

// cellViolations holds a catalogue's family sections, and the metric types of
// a Families-shaped type under the namespaces the catalogue gives them, to
// the scope and cell conventions of docs/metrics.md section 1.
func cellViolations(content string, families reflect.Type) []cellViolation {
	var out []cellViolation
	add := func(record int, format string, args ...any) {
		out = append(out, cellViolation{record, fmt.Sprintf(format, args...)})
	}
	namespaces := catalogueNamespaces(content)
	metricTypes := map[string]reflect.Type{}
	for i := 0; i < families.NumField(); i++ {
		field := families.Field(i)
		if metrics, ok := field.Type.FieldByName("Metrics"); ok && metrics.Type.Kind() == reflect.Struct {
			metricTypes[jsonName(field)] = metrics.Type
		}
	}

	sections := catalogueCellSections(content)
	for _, family := range sortedKeys(sections) {
		section := sections[family]
		marked, listsCells := false, false
		for _, row := range section.rows {
			if row.scoped && !scopeValues()[row.scope] {
				add(78, "the family %s's Metric table gives %s the scope %q, which is none of repository, person, "+
					"year and person, year (%s section 1)", family, strings.Join(row.metrics, ", "), row.scope,
					metricsCatalogue)
			}
			if row.scoped && row.scope != "repository" && scopeValues()[row.scope] {
				marked = true
			}
			for _, metric := range row.metrics {
				listsCells = listsCells || metric == "cells"
			}
		}

		metrics, typed := metricTypes[namespaces[family]]
		var cell reflect.Type
		carried, problem := false, ""
		if typed {
			cell, carried, problem = cellsElement(metrics)
		}

		if section.cellFields == nil {
			if marked {
				add(78, "the family %s marks a metric person or year and has no Cell field table defining its cells "+
					"(clause 3)", family)
			}
			if listsCells {
				add(78, "the family %s lists cells in its Metric table and has no Cell field table defining them",
					family)
			}
			if carried {
				add(62, "the report type carries cells for the family %s, whose section in %s has no Cell field table",
					family, metricsCatalogue)
			}
			continue
		}

		if !marked {
			add(78, "the family %s has a Cell field table and marks no metric person or year; only a family with a "+
				"scoped metric carries cells (clause 3)", family)
		}
		if !listsCells {
			add(78, "the family %s has a Cell field table and its Metric table does not list cells", family)
		}
		switch {
		case !typed:
			add(62, "the family %s has a Cell field table and the report type has no metric type under its "+
				"namespace %q", family, namespaces[family])
		case !carried:
			add(62, "the family %s has a Cell field table and its metric type %s has no cells field", family, metrics)
		case problem != "":
			add(62, "the cells field of the family %s's metric type %s", family, problem)
		default:
			carriedFields := cellFieldNames(cell)
			for _, field := range sortedKeys(section.cellFields) {
				if !carriedFields[field] {
					add(62, "%s defines the cell field %s for the family %s, which its cell type %s does not carry",
						metricsCatalogue, field, family, cell)
				}
			}
			for _, field := range sortedKeys(carriedFields) {
				if !section.cellFields[field] {
					add(62, "the family %s's cell type %s carries the field %s, which the Cell field table of %s does "+
						"not define", family, cell, field, metricsCatalogue)
				}
			}
		}
	}
	return out
}

// TestMetricCatalogueCells holds every family section of docs/metrics.md, and
// the report's metric types, to the scope and cell conventions of section 1
// (ADR-0078 clauses 2 and 3). While no family carries cells, it holds every
// section to marking no metric person or year and every type to carrying no
// cells.
func TestMetricCatalogueCells(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	content := repo.read(t, 62, metricsCatalogue)
	sections := catalogueCellSections(content)
	withRows := 0
	for _, family := range sortedKeys(sections) {
		section := sections[family]
		if len(section.rows) > 0 {
			withRows++
		}
		if section.cellFields != nil {
			t.Logf("%s carries the cell fields %v", family, sortedKeys(section.cellFields))
		}
	}
	if withRows == 0 || len(catalogueNamespaces(content)) == 0 {
		fatal(t, 64, "no Metric table row or namespace was read from %s, so the checker would pass vacuously",
			metricsCatalogue)
	}
	for _, v := range cellViolations(content, reflect.TypeOf(core.Families{})) {
		report(t, v.record, "%s", v.text)
	}
}

// The synthetic report types TestMetricCatalogueCellsRejectsEachViolation
// feeds the checker. alpha is a family with cells, beta one without.
type (
	alphaCell struct {
		core.CellIdentity
		Year    int `json:"year"`
		Commits int `json:"commits"`
	}
	alphaMetrics struct {
		Count *int        `json:"count,omitzero"`
		Cells []alphaCell `json:"cells,omitzero"`
	}
	betaMetrics struct {
		Total *int `json:"total,omitzero"`
	}
	cellFamilies struct {
		Alpha core.Family[alphaMetrics] `json:"alpha"`
		Beta  core.Family[betaMetrics]  `json:"beta"`
	}

	uncellularAlphaMetrics struct {
		Count *int `json:"count,omitzero"`
	}
	uncellularFamilies struct {
		Alpha core.Family[uncellularAlphaMetrics] `json:"alpha"`
		Beta  core.Family[betaMetrics]            `json:"beta"`
	}

	widerAlphaCell struct {
		alphaCell
		Lines int `json:"lines"`
	}
	widerAlphaMetrics struct {
		Count *int             `json:"count,omitzero"`
		Cells []widerAlphaCell `json:"cells,omitzero"`
	}
	widerFamilies struct {
		Alpha core.Family[widerAlphaMetrics] `json:"alpha"`
		Beta  core.Family[betaMetrics]       `json:"beta"`
	}

	flatAlphaMetrics struct {
		Count *int `json:"count,omitzero"`
		Cells *int `json:"cells,omitzero"`
	}
	flatFamilies struct {
		Alpha core.Family[flatAlphaMetrics] `json:"alpha"`
		Beta  core.Family[betaMetrics]      `json:"beta"`
	}

	cellularBetaMetrics struct {
		Total *int        `json:"total,omitzero"`
		Cells []alphaCell `json:"cells,omitzero"`
	}
	cellularBetaFamilies struct {
		Alpha core.Family[alphaMetrics]        `json:"alpha"`
		Beta  core.Family[cellularBetaMetrics] `json:"beta"`
	}
)

// cellCatalogue is a synthetic catalogue the types above satisfy.
const cellCatalogue = "## 1. Shared definitions\n\n| Term | Definition |\n|---|---|\n| `person` | x |\n\n" +
	"## 2. `alpha`\n\n**Input:** commit-records. **Namespace:** `alpha`.\n\n" +
	"| Metric | Definition | Scope |\n|---|---|---|\n" +
	"| `count` | Commits, counted with `commits`. | person, year |\n" +
	"| `cells` | The cells. | repository |\n\n" +
	"| Cell field | Definition |\n|---|---|\n" +
	"| `id`, `aggregate` | The cell identity. |\n| `year` | The year. |\n| `commits` | A count. |\n\n" +
	"## 3. `beta`\n\n**Input:** commit-records. **Namespace:** `beta`.\n\n" +
	"| Metric | Definition |\n|---|---|\n| `total` | A total. |\n"

// TestMetricCatalogueCellsRejectsEachViolation is the failure demonstration
// ADR-0064 clause 6 requires. A synthetic catalogue and report type that pass
// are broken one thing at a time, and the checker must report each break,
// under the record it breaks.
func TestMetricCatalogueCellsRejectsEachViolation(t *testing.T) {
	t.Parallel()
	valid := reflect.TypeOf(cellFamilies{})
	if got := cellViolations(cellCatalogue, valid); len(got) != 0 {
		report(t, 64, "the unbroken catalogue and type were reported, so an embedded cell identity's promoted "+
			"fields or the Scope column are misread: %v", got)
	}

	edit := func(old, replacement string) string {
		if !strings.Contains(cellCatalogue, old) {
			t.Fatalf("the synthetic catalogue has no %q to replace", old)
		}
		return strings.Replace(cellCatalogue, old, replacement, 1)
	}
	for _, c := range []struct {
		name     string
		content  string
		families reflect.Type
		record   int
		want     string
	}{
		{"a Scope value that is none of the four", edit("| person, year |", "| team |"), valid, 78,
			`gives count the scope "team"`},
		{"a row with no Scope value", edit("| repository |", "| |"), valid, 78, `gives cells the scope ""`},
		{"a scoped metric and no Cell field table",
			edit("| Cell field | Definition |", "| Other | Definition |"), valid, 78,
			"alpha marks a metric person or year and has no Cell field table"},
		{"a Cell field table and no scoped metric", edit("| person, year |", "| repository |"), valid, 78,
			"alpha has a Cell field table and marks no metric person or year"},
		{"a Cell field table and no cells row", edit("| `cells` | The cells. | repository |\n", ""), valid, 78,
			"alpha has a Cell field table and its Metric table does not list cells"},
		{"cells listed and no Cell field table", edit("| `total` | A total. |", "| `total`, `cells` | A total. |"),
			valid, 78, "beta lists cells in its Metric table and has no Cell field table"},
		{"a Cell field table and no cells field", cellCatalogue, reflect.TypeOf(uncellularFamilies{}), 62,
			"alpha has a Cell field table and its metric type"},
		{"a cells field that is no list of cells", cellCatalogue, reflect.TypeOf(flatFamilies{}), 62,
			"cells field of the family alpha's metric type"},
		{"a cell field the type does not carry", edit("| `commits` | A count. |", "| `commits`, `authors` | A count. |"),
			valid, 62, "defines the cell field authors for the family alpha"},
		{"a promoted field the table does not define", edit("| `id`, `aggregate` |", "| `id` |"), valid, 62,
			"carries the field aggregate"},
		{"a field the table does not define", cellCatalogue, reflect.TypeOf(widerFamilies{}), 62,
			"carries the field lines"},
		{"cells carried and no Cell field table", cellCatalogue, reflect.TypeOf(cellularBetaFamilies{}), 62,
			"carries cells for the family beta"},
	} {
		got := cellViolations(c.content, c.families)
		found := false
		for _, v := range got {
			if v.record == c.record && strings.Contains(v.text, c.want) {
				found = true
			}
		}
		if !found {
			report(t, 64, "%s was not reported as ADR-%04d %q; the checker reported: %v", c.name, c.record, c.want, got)
		}
	}
}

// TestMetricCatalogueCellsParsing exercises the table reading the checker
// depends on, so that it cannot pass because it read nothing: every Metric
// table of a section is read, with its Scope column where it has one, an
// escaped pipe stays inside its cell, and only a table headed Cell field
// defines cell fields.
func TestMetricCatalogueCellsParsing(t *testing.T) {
	t.Parallel()
	content := "## 2. `alpha`\r\n\r\n| Metric | Definition | Scope |\r\n|---|---|---|\r\n" +
		"| `one`, `two` | a \\| b | person, year |\r\n\r\nProse.\r\n\r\n" +
		"| Metric | Definition |\r\n|---|---|\r\n| `three` | c |\r\n\r\n" +
		"| Cell field | Definition |\r\n|---|---|\r\n| `id`, `year` | d |\r\n\r\n" +
		"| Rule | Value |\r\n|---|---|\r\n| `rule` | 1 |\r\n"
	got := catalogueCellSections(content)["alpha"]
	want := []catalogueMetricRow{
		{metrics: []string{"one", "two"}, scope: "person, year", scoped: true},
		{metrics: []string{"three"}},
	}
	if !reflect.DeepEqual(got.rows, want) {
		report(t, 64, "rows = %+v, want %+v", got.rows, want)
	}
	if strings.Join(sortedKeys(got.cellFields), " ") != "id year" {
		report(t, 64, "cell fields = %v, want id and year", sortedKeys(got.cellFields))
	}
	if cells := tableCells(`| a \| b | c |`); len(cells) != 2 || cells[0] != "a | b" {
		report(t, 64, "an escaped pipe split its cell: %q", cells)
	}
	if names := cellFieldNames(reflect.TypeOf(widerAlphaCell{})); strings.Join(sortedKeys(names), " ") !=
		"aggregate commits id lines year" {
		report(t, 64, "the promoted cell fields = %v", sortedKeys(names))
	}
}
