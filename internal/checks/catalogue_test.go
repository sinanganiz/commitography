package checks

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
)

// The metric catalogue checker (ADR-0063 table 2): no metric, reason code or
// cardinality limit exists in the report that docs/metrics.md does not define
// (ADR-0062 clauses 1 and 6).
//
// It reads the catalogue's family sections, their metric tables, section 12 and
// section 13, and compares them with the report in two places: the Go types
// the report is encoded from, which cover every field the code can write, and
// the golden reports, which are what the analysis actually wrote. The reason
// code enumeration itself is compared with section 13 by
// TestReasonCodeCatalogue.

const (
	familyRecord      = "docs/decisions/0076-family-contract-three-inputs.md"
	limitsSection     = "## 12. Cardinality limits, collected"
	identitiesSection = "## 14. The identities section"
)

// identityFields returns the fields section 14 defines for an identities
// entry: the backticked names in the first column of its table headed "Field".
func identityFields(section string) map[string]bool {
	name := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	out := map[string]bool{}
	inTable := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "| Field |"):
			inTable = true
		case !strings.HasPrefix(trimmed, "|"):
			inTable = false
		case inTable && !strings.HasPrefix(trimmed, "|---"):
			for _, n := range name.FindAllStringSubmatch(strings.Split(trimmed, "|")[1], -1) {
				out[n[1]] = true
			}
		}
	}
	return out
}

// entryViolations compares an identities entry type with the fields section 14
// defines, in both directions.
func entryViolations(entry reflect.Type, fields map[string]bool) []string {
	var out []string
	inType := map[string]bool{}
	for i := 0; i < entry.NumField(); i++ {
		field := jsonName(entry.Field(i))
		inType[field] = true
		if !fields[field] {
			out = append(out, "the identities entry has a field "+field+", which "+metricsCatalogue+
				" section 14 does not define")
		}
	}
	for _, field := range sortedKeys(fields) {
		if !inType[field] {
			out = append(out, metricsCatalogue+" section 14 defines the identities field "+field+
				", which the report type does not carry")
		}
	}
	return out
}

// catalogueFamilies returns the families docs/metrics.md defines, each with the
// metric names its tables list. A family section is a level-two heading whose
// title is the backticked family name; its metrics are the backticked names in
// the first column of a table headed "Metric".
func catalogueFamilies(content string) map[string]map[string]bool {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	heading := regexp.MustCompile("(?m)^## [0-9]+\\. `([a-z][a-z-]*)`\\s*$")
	name := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	families := map[string]map[string]bool{}

	matches := heading.FindAllStringSubmatchIndex(content, -1)
	for _, m := range matches {
		end := len(content)
		if next := strings.Index(content[m[1]:], "\n## "); next >= 0 {
			end = m[1] + next
		}
		family := content[m[2]:m[3]]
		metrics := map[string]bool{}
		inTable := false
		for _, line := range strings.Split(content[m[1]:end], "\n") {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "| Metric |"):
				inTable = true
			case !strings.HasPrefix(trimmed, "|"):
				inTable = false
			case inTable && !strings.HasPrefix(trimmed, "|---"):
				cells := strings.Split(trimmed, "|")
				for _, n := range name.FindAllStringSubmatch(cells[1], -1) {
					metrics[n[1]] = true
				}
			}
		}
		families[family] = metrics
	}
	return families
}

// recordFamilies returns the family names of ADR-0076 clause 6.
func recordFamilies(content string) map[string]bool {
	row := regexp.MustCompile("(?m)^\\s*\\| `([a-z][a-z-]*)` \\|")
	out := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(content, -1) {
		out[m[1]] = true
	}
	return out
}

// catalogueLimits returns the rows of section 12 as label and value.
func catalogueLimits(section string) map[core.CardinalityLimit]bool {
	row := regexp.MustCompile(`(?m)^\| ([^|]+?) \| ([^|]+?) \|\s*$`)
	out := map[core.CardinalityLimit]bool{}
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		if m[1] == "Limit" || strings.HasPrefix(m[1], "---") {
			continue
		}
		out[core.CardinalityLimit{Label: m[1], Value: m[2]}] = true
	}
	return out
}

// familyStatusCodes returns the codes section 13 lists for family status: the
// backticked codes between its "Family status codes" and "User error codes"
// paragraphs.
func familyStatusCodes(section string) map[string]bool {
	start := strings.Index(section, "Family status codes")
	end := strings.Index(section, "User error codes")
	out := map[string]bool{}
	if start < 0 || end < start {
		return out
	}
	for _, m := range regexp.MustCompile("`([a-z][a-z0-9_]*)`").FindAllStringSubmatch(section[start:end], -1) {
		out[m[1]] = true
	}
	return out
}

// namespaceOwners inverts the namespaces docs/metrics.md gives the families:
// the family whose namespace each report key is.
func namespaceOwners(namespaces map[string]string) map[string]string {
	out := map[string]string{}
	for family, namespace := range namespaces {
		out[namespace] = family
	}
	return out
}

// typeViolations compares a Families-shaped type with the catalogue: every
// field's key is the namespace of a family the catalogue defines, and every
// JSON name of that family's metric type is a metric its section defines.
// owners gives the family each namespace belongs to.
func typeViolations(families reflect.Type, catalogue map[string]map[string]bool, owners map[string]string) []string {
	var out []string
	for i := 0; i < families.NumField(); i++ {
		field := families.Field(i)
		family, known := owners[jsonName(field)]
		metrics := catalogue[family]
		if !known || metrics == nil {
			out = append(out, "the report type has a family under "+jsonName(field)+", which is the namespace of "+
				"no family "+metricsCatalogue+" defines")
			continue
		}
		mf, ok := field.Type.FieldByName("Metrics")
		if !ok || mf.Type.Kind() != reflect.Struct {
			out = append(out, "the family "+family+" has no Metrics struct")
			continue
		}
		for j := 0; j < mf.Type.NumField(); j++ {
			if metric := jsonName(mf.Type.Field(j)); !metrics[metric] {
				out = append(out, "the family "+family+" has a metric "+metric+", which "+metricsCatalogue+
					" does not define for it")
			}
		}
	}
	return out
}

func jsonName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		return field.Name
	}
	return name
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestMetricCatalogueFamilies requires the report's families, the catalogue's
// family sections and ADR-0076 clause 6 to be one set, with the report
// writing each family under the namespace the catalogue gives it (ADR-0076
// clause 1, ADR-0062 clause 3).
func TestMetricCatalogueFamilies(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	content := repo.read(t, 62, metricsCatalogue)
	catalogue := catalogueFamilies(content)
	namespaces := catalogueNamespaces(content)
	record := recordFamilies(repo.read(t, 76, familyRecord))
	if len(catalogue) == 0 || len(namespaces) == 0 || len(record) == 0 {
		fatal(t, 64, "no family was read from %s or %s; the checker would pass vacuously", metricsCatalogue, familyRecord)
	}

	inReport := map[string]bool{}
	families := reflect.TypeOf(core.Families{})
	for i := 0; i < families.NumField(); i++ {
		inReport[jsonName(families.Field(i))] = true
	}
	for _, f := range sortedKeys(record) {
		if catalogue[f] == nil {
			report(t, 62, "%s clause 6 names the family %s, which %s has no section for", familyRecord, f, metricsCatalogue)
		}
		if namespace, ok := namespaces[f]; !ok || !inReport[namespace] {
			report(t, 32, "the family %s is absent from the report type under the namespace %s gives it, so it "+
				"is absent from every report", f, metricsCatalogue)
		}
	}
	for _, f := range sortedKeys(catalogue) {
		if !record[f] {
			report(t, 76, "%s has a section for %s, which %s clause 6 does not list", metricsCatalogue, f, familyRecord)
		}
	}
	owners := namespaceOwners(namespaces)
	for _, key := range sortedKeys(inReport) {
		if family, ok := owners[key]; !ok || !record[family] {
			report(t, 76, "the report type carries a family under %s, which is the namespace of no family %s "+
				"clause 6 lists", key, familyRecord)
		}
	}
}

// TestMetricCatalogueTypes requires every metric field of the report type to be
// a metric the catalogue defines for its family.
func TestMetricCatalogueTypes(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	content := repo.read(t, 62, metricsCatalogue)
	owners := namespaceOwners(catalogueNamespaces(content))
	for _, v := range typeViolations(reflect.TypeOf(core.Families{}), catalogueFamilies(content), owners) {
		report(t, 62, "%s", v)
	}
}

// TestMetricCatalogueRejectsAnInventedMetric is the failure demonstration
// ADR-0064 clause 6 requires: a family type carrying a metric the catalogue
// does not define, and a family the catalogue does not have, are both
// reported.
func TestMetricCatalogueRejectsAnInventedMetric(t *testing.T) {
	t.Parallel()
	type inventedMetrics struct {
		HourHistogram   []int   `json:"hour_histogram,omitzero"`
		HourWeekdayGrid [][]int `json:"hour_weekday_grid,omitzero"`
	}
	type invented struct {
		Temporal core.Family[inventedMetrics] `json:"temporal"`
		Mood     core.Family[struct{}]        `json:"mood"`
	}
	catalogue := map[string]map[string]bool{"temporal": {"hour_histogram": true}}
	owners := map[string]string{"temporal": "temporal"}
	got := strings.Join(typeViolations(reflect.TypeOf(invented{}), catalogue, owners), "\n")
	for _, want := range []string{"hour_weekday_grid", "mood"} {
		if !strings.Contains(got, want) {
			report(t, 64, "the catalogue checker did not report the invented %s; it reported:\n%s", want, got)
		}
	}
	if strings.Contains(got, "metric hour_histogram") {
		report(t, 64, "the catalogue checker reported the defined hour_histogram:\n%s", got)
	}
}

// TestMetricCatalogueIdentities requires the identities section to be a
// top-level section of the report and not a family, and its entry's fields to
// be exactly those section 14 defines.
func TestMetricCatalogueIdentities(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fields := identityFields(catalogueSection(t, repo, identitiesSection))
	if len(fields) == 0 {
		fatal(t, 64, "no identities field was read from %s section 14", metricsCatalogue)
	}
	for _, v := range entryViolations(reflect.TypeOf(core.IdentityEntry{}), fields) {
		report(t, 62, "%s", v)
	}

	top, ok := reflect.TypeOf(core.Report{}).FieldByName("Identities")
	if !ok || jsonName(top) != "identities" || top.Type != reflect.TypeOf([]core.IdentityEntry{}) {
		report(t, 10, "the report type has no top-level identities section of entries, so no reader can "+
			"select from the resolved contributors")
	}
	families := reflect.TypeOf(core.Families{})
	for i := 0; i < families.NumField(); i++ {
		if jsonName(families.Field(i)) == "identities" {
			report(t, 76, "the report type carries identities as a family; %s clause 6 fixes the family set "+
				"and it is not in it", familyRecord)
		}
	}
}

// TestMetricCatalogueRejectsAnInventedIdentityField is the failure
// demonstration for the identities half: an entry carrying a field section 14
// does not define, or missing one it does, is reported.
func TestMetricCatalogueRejectsAnInventedIdentityField(t *testing.T) {
	t.Parallel()
	type invented struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	got := strings.Join(entryViolations(reflect.TypeOf(invented{}),
		map[string]bool{"id": true, "display_name": true}), "\n")
	for _, want := range []string{"field email", "field display_name"} {
		if !strings.Contains(got, want) {
			report(t, 64, "the catalogue checker did not report the %s; it reported:\n%s", want, got)
		}
	}
	if strings.Contains(got, "field id") {
		report(t, 64, "the catalogue checker reported the defined id:\n%s", got)
	}
}

// TestMetricCatalogueGolden requires every metric and reason code in every
// golden report to be one the catalogue defines, every identities field to be
// one section 14 defines, and every skipped family's metrics to be empty
// (ADR-0032 clause 3).
func TestMetricCatalogueGolden(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	content := repo.read(t, 62, metricsCatalogue)
	catalogue := catalogueFamilies(content)
	owners := namespaceOwners(catalogueNamespaces(content))
	codes := familyStatusCodes(catalogueSection(t, repo, reasonSection))
	if len(codes) == 0 {
		fatal(t, 64, "no family status code was read from %s section 13", metricsCatalogue)
	}
	fields := identityFields(catalogueSection(t, repo, identitiesSection))
	if len(fields) == 0 {
		fatal(t, 64, "no identities field was read from %s section 14", metricsCatalogue)
	}
	checked := 0
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, goldenDir+"/") || !strings.HasSuffix(file, ".json") {
			continue
		}
		var document struct {
			Families map[string]struct {
				Status  string                     `json:"status"`
				Reasons []string                   `json:"reasons"`
				Metrics map[string]json.RawMessage `json:"metrics"`
			} `json:"families"`
			Identities []map[string]json.RawMessage `json:"identities"`
		}
		if err := json.Unmarshal([]byte(repo.read(t, 62, file)), &document); err != nil {
			report(t, 62, "%s is not a report: %v", file, err)
			continue
		}
		checked++
		for i, entry := range document.Identities {
			for _, field := range sortedKeys(entry) {
				if !fields[field] {
					report(t, 62, "%s carries the identities field %s in entry %d, which %s section 14 does not "+
						"define", file, field, i, metricsCatalogue)
				}
			}
		}
		for _, name := range sortedKeys(document.Families) {
			family := document.Families[name]
			for _, metric := range sortedKeys(family.Metrics) {
				if !catalogue[owners[name]][metric] {
					report(t, 62, "%s carries the metric %s.%s, which %s does not define", file, name, metric, metricsCatalogue)
				}
			}
			for _, code := range family.Reasons {
				if !codes[code] {
					report(t, 62, "%s gives the family %s the reason %q, which %s section 13 does not list as a "+
						"family status code", file, name, code, metricsCatalogue)
				}
			}
			if family.Status == string(core.StatusSkipped) && len(family.Metrics) != 0 {
				report(t, 32, "%s marks %s skipped but gives it metrics %v; a skipped family's data is empty",
					file, name, sortedKeys(family.Metrics))
			}
		}
	}
	if checked == 0 {
		fatal(t, 64, "no golden report is tracked under %s, so there is nothing to check", goldenDir)
	}
}

// TestMetricCatalogueSchemaReasons requires the reason codes the schema
// documents (ADR-0032 clause 4) to be exactly the family status codes of
// section 13.
func TestMetricCatalogueSchemaReasons(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	codes := familyStatusCodes(catalogueSection(t, repo, reasonSection))
	var schema struct {
		Defs struct {
			Reasons struct {
				Items struct {
					Enum []string `json:"enum"`
				} `json:"items"`
			} `json:"reasons"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal([]byte(repo.read(t, 62, reportSchema)), &schema); err != nil {
		fatal(t, 62, "%s is not valid JSON: %v", reportSchema, err)
	}
	documented := map[string]bool{}
	for _, code := range schema.Defs.Reasons.Items.Enum {
		documented[code] = true
		if !codes[code] {
			report(t, 62, "%s documents the reason %q, which %s section 13 does not list as a family status code",
				reportSchema, code, metricsCatalogue)
		}
	}
	for _, code := range sortedKeys(codes) {
		if !documented[code] {
			report(t, 32, "%s section 13 lists the family status code %q, which %s does not document",
				metricsCatalogue, code, reportSchema)
		}
	}
}

// TestMetricCatalogueLimits compares the limits core declares with section 12
// in both directions, and requires the metric families to declare no limit of
// their own. The second half recognises a limit by its constant's name, so a
// limit written as a bare literal is beyond it.
func TestMetricCatalogueLimits(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	documented := catalogueLimits(catalogueSection(t, repo, limitsSection))
	if len(documented) == 0 {
		fatal(t, 64, "no limit was read from %s section 12", metricsCatalogue)
	}
	declared := map[core.CardinalityLimit]bool{}
	for _, l := range core.CardinalityLimits() {
		declared[l] = true
		if !documented[l] {
			report(t, 62, "core declares the limit %q = %s, which %s section 12 does not define", l.Label, l.Value, metricsCatalogue)
		}
	}
	for l := range documented {
		if !declared[l] {
			report(t, 62, "%s section 12 defines the limit %q = %s, which core does not declare", metricsCatalogue, l.Label, l.Value)
		}
	}

	limitName := regexp.MustCompile(`(?i)limit|shown|max(imum)?(entries|items|listed)`)
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, "internal/metrics/") || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, repo.read(t, 62, file), 0)
		if err != nil {
			report(t, 62, "cannot parse %s: %v", file, err)
			continue
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if limitName.MatchString(name.Name) {
						report(t, 62, "%s declares the constant %s, which reads as a cardinality limit; a family takes "+
							"its limits from core, which mirrors %s section 12", file, name.Name, metricsCatalogue)
					}
				}
			}
		}
	}
}

// TestMetricCatalogueHelpers exercises the parsing the checkers above depend
// on, so that one cannot pass because its parser silently matched nothing.
func TestMetricCatalogueHelpers(t *testing.T) {
	t.Parallel()
	content := "## 1. Shared\n\n| Term | Definition |\n|---|---|\n| `not_a_metric` | x |\n\n" +
		"## 2. `alpha`\n\n| Rule | Value |\n|---|---|\n| `rule_name` | 5 |\n\n" +
		"| Metric | Definition |\n|---|---|\n| `one`, `two` | x |\n| `three` | uses `ref` |\n\nProse `prose`.\n\n" +
		"## 3. `beta-gamma`\n\nNo table.\n"
	got := catalogueFamilies(content)
	if len(got) != 2 || got["beta-gamma"] == nil || len(got["beta-gamma"]) != 0 {
		report(t, 64, "families = %v, want alpha and an empty beta-gamma", got)
	}
	if want := "one three two"; strings.Join(sortedKeys(got["alpha"]), " ") != want {
		report(t, 64, "alpha metrics = %v, want %s", sortedKeys(got["alpha"]), want)
	}

	limits := catalogueLimits("| Limit | Value |\n|---|---|\n| Coupling pairs | 200 |\n| Nodes / edges | 1 / 2 |\n")
	if len(limits) != 2 || !limits[core.CardinalityLimit{Label: "Nodes / edges", Value: "1 / 2"}] {
		report(t, 64, "limits = %v, want two rows", limits)
	}

	codes := familyStatusCodes("Family status codes, carried:\n\n`a_code`, `b_code`.\n\nUser error codes:\n\n| `c_code` | x |\n")
	if strings.Join(sortedKeys(codes), " ") != "a_code b_code" {
		report(t, 64, "family status codes = %v, want a_code and b_code", sortedKeys(codes))
	}

	if got := recordFamilies("   | `temporal` | commit-records |\n   | Family | Inputs |\n"); len(got) != 1 || !got["temporal"] {
		report(t, 64, "record families = %v, want temporal", got)
	}

	fields := identityFields("Prose `prose`.\n\n| Field | Definition |\n|---|---|\n| `one`, `two` | uses `ref` |\n\n" +
		"| Rule | Value |\n|---|---|\n| `rule` | 1 |\n")
	if strings.Join(sortedKeys(fields), " ") != "one two" {
		report(t, 64, "identities fields = %v, want one and two", sortedKeys(fields))
	}
}
