package checks

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
)

// The family declaration checker (ADR-0076 clause 1, ADR-0031 clause 2,
// ADR-0062 clause 3): every family of ADR-0076 clause 6 has a package that
// declares its contract, and every declaration agrees with the catalogue.
//
// It checks the declarations, not the report. The families it checks are the
// aggregate stage's registry, the one list of families in the tree, so a
// family the stage runs cannot escape it and it holds no list of its own.

// catalogueNamespaces returns the namespace docs/metrics.md gives each family:
// the backticked name after "**Namespace:**" in the family's section. A
// family section without one is absent from the result.
func catalogueNamespaces(content string) map[string]string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	heading := regexp.MustCompile("(?m)^## [0-9]+\\. `([a-z][a-z-]*)`\\s*$")
	namespace := regexp.MustCompile("\\*\\*Namespace:\\*\\* `([a-z][a-z0-9_-]*)`")
	out := map[string]string{}
	for _, m := range heading.FindAllStringSubmatchIndex(content, -1) {
		section := content[m[1]:]
		if next := strings.Index(section, "\n## "); next >= 0 {
			section = section[:next]
		}
		if n := namespace.FindStringSubmatch(section); n != nil {
			out[content[m[2]:m[3]]] = n[1]
		}
	}
	return out
}

// recordInputs returns each family's row of ADR-0076 clause 6: the family
// name and the input kinds its second column lists.
func recordInputs(content string) map[string][]core.InputKind {
	row := regexp.MustCompile("(?m)^\\s*\\| `([a-z][a-z-]*)` \\| ([a-z, -]+?) \\|\\s*$")
	out := map[string][]core.InputKind{}
	for _, m := range row.FindAllStringSubmatch(strings.ReplaceAll(content, "\r\n", "\n"), -1) {
		var inputs []core.InputKind
		for _, in := range strings.Split(m[2], ",") {
			inputs = append(inputs, core.InputKind(strings.TrimSpace(in)))
		}
		out[m[1]] = inputs
	}
	return out
}

// sameInputs reports whether two input lists hold the same kinds, in any
// order.
func sameInputs(a, b []core.InputKind) bool {
	set := func(l []core.InputKind) string {
		names := map[string]bool{}
		for _, k := range l {
			names[string(k)] = true
		}
		return strings.Join(sortedKeys(names), ",")
	}
	return len(a) == len(b) && set(a) == set(b)
}

// declarationViolation is one failure, with the record it breaks.
type declarationViolation struct {
	record int
	text   string
}

// declarationViolations compares declarations with the rows of ADR-0076
// clause 6 and the namespaces of docs/metrics.md.
func declarationViolations(declared []core.FamilyDeclaration, rows map[string][]core.InputKind,
	namespaces map[string]string) []declarationViolation {
	var out []declarationViolation
	add := func(record int, format string, args ...any) {
		out = append(out, declarationViolation{record, fmt.Sprintf(format, args...)})
	}

	byName := map[string]bool{}
	for _, d := range declared {
		byName[d.Name] = true
	}
	for _, f := range sortedKeys(rows) {
		if !byName[f] {
			add(76, "the family %s of %s clause 6 has no declaring package", f, familyRecord)
		}
	}

	// The owner of every namespace, as the catalogue gives it and as the
	// declarations claim it, so that an input naming either is recognised.
	owner := map[string]string{}
	for family, ns := range namespaces {
		owner[ns] = family
	}
	claimed := map[string][]string{}
	for _, d := range declared {
		claimed[d.Namespace] = append(claimed[d.Namespace], d.Name)
		if _, ok := owner[d.Namespace]; !ok && d.Namespace != "" {
			owner[d.Namespace] = d.Name
		}
	}
	kinds := map[core.InputKind]bool{}
	for _, k := range core.InputKinds() {
		kinds[k] = true
	}

	for _, d := range declared {
		if len(d.Inputs) == 0 {
			add(76, "the family %s declares no input", d.Name)
		}
		for _, in := range d.Inputs {
			if other, ok := owner[string(in)]; ok && other != d.Name {
				add(76, "the family %s declares the namespace %s of the family %s as an input; a family never "+
					"reads another family's output (clause 4)", d.Name, in, other)
				continue
			}
			if !kinds[in] {
				add(76, "the family %s declares the input %q, which is not one of the three kinds %v",
					d.Name, in, core.InputKinds())
			}
		}

		if row, ok := rows[d.Name]; ok && !sameInputs(d.Inputs, row) {
			add(76, "the family %s declares the inputs %v, and %s clause 6 gives it %v",
				d.Name, d.Inputs, familyRecord, row)
		}

		switch want, ok := namespaces[d.Name]; {
		case !ok:
			add(62, "the family %s declares the namespace %q, and %s gives no namespace for it",
				d.Name, d.Namespace, metricsCatalogue)
		case d.Namespace != want:
			add(62, "the family %s declares the namespace %q, and %s gives %q", d.Name, d.Namespace,
				metricsCatalogue, want)
		}

		if d.Version == (core.Version{}) && d.Status != core.StatusNotImplemented {
			add(31, "the family %s declares no version; only a family whose status source is %s carries "+
				"the zero version", d.Name, core.StatusNotImplemented)
		}
	}

	for _, ns := range sortedKeys(claimed) {
		if names := claimed[ns]; len(names) > 1 {
			add(76, "the families %s all declare the namespace %q; a namespace has one owner (clause 5)",
				strings.Join(names, ", "), ns)
		}
	}
	return out
}

// declarationsOf returns what each family declares.
func declarationsOf(families []core.MetricFamily) []core.FamilyDeclaration {
	out := make([]core.FamilyDeclaration, 0, len(families))
	for _, f := range families {
		out = append(out, f.Declaration())
	}
	return out
}

// TestFamilyDeclarations holds every family's declaration to ADR-0076 clause 6
// and to the namespaces docs/metrics.md gives.
func TestFamilyDeclarations(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	rows := recordInputs(repo.read(t, 76, familyRecord))
	namespaces := catalogueNamespaces(repo.read(t, 62, metricsCatalogue))
	if len(rows) == 0 || len(namespaces) == 0 {
		fatal(t, 64, "no family was read from %s or no namespace from %s; the checker would pass vacuously",
			familyRecord, metricsCatalogue)
	}

	declared := declarationsOf(aggregate.Registry())
	for _, d := range declared {
		t.Logf("%-15s inputs %v namespace %s version %s status %s method %t",
			d.Name, d.Inputs, d.Namespace, d.Version, d.Status, d.Method != "")
	}
	for _, v := range declarationViolations(declared, rows, namespaces) {
		report(t, v.record, "%s", v.text)
	}
}

// TestFamilyDeclarationRejectsEachViolation is the failure demonstration
// ADR-0064 clause 6 requires. Starting from a set of declarations that
// passes, it breaks one thing at a time and requires the checker to report
// it, under the record it breaks.
func TestFamilyDeclarationRejectsEachViolation(t *testing.T) {
	t.Parallel()
	valid := func() ([]core.FamilyDeclaration, map[string][]core.InputKind, map[string]string) {
		declared := declarationsOf(aggregate.Registry())
		rows := map[string][]core.InputKind{}
		namespaces := map[string]string{}
		for _, d := range declared {
			rows[d.Name] = append([]core.InputKind(nil), d.Inputs...)
			namespaces[d.Name] = d.Namespace
		}
		return declared, rows, namespaces
	}
	index := func(declared []core.FamilyDeclaration, name string) int {
		for i, d := range declared {
			if d.Name == name {
				return i
			}
		}
		t.Fatalf("no declaration named %s", name)
		return -1
	}

	if got := declarationViolations(valid()); len(got) != 0 {
		report(t, 64, "the unbroken declarations were reported: %v", got)
	}

	cases := []struct {
		name   string
		breaks func([]core.FamilyDeclaration) []core.FamilyDeclaration
		record int
		want   string
	}{
		{"a family has no declaring package", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			i := index(d, "coupling")
			return append(d[:i:i], d[i+1:]...)
		}, 76, "coupling of " + familyRecord + " clause 6 has no declaring package"},
		{"an input outside the three kinds", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "temporal")].Inputs = []core.InputKind{"commit-log"}
			return d
		}, 76, `temporal declares the input "commit-log", which is not one of the three kinds`},
		{"no input", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "messages")].Inputs = nil
			return d
		}, 76, "messages declares no input"},
		{"a namespace the catalogue does not give", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "commit-size")].Namespace = "commit-size"
			return d
		}, 62, `commit-size declares the namespace "commit-size", and docs/metrics.md gives "commit_size"`},
		{"two families declare one namespace", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "hotspot")].Namespace = "files"
			return d
		}, 76, `files, hotspot all declare the namespace "files"`},
		{"another family's namespace as an input", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "hotspot")].Inputs = []core.InputKind{core.InputCommitRecords, "coupling"}
			return d
		}, 76, "hotspot declares the namespace coupling of the family coupling as an input"},
		{"inputs that differ from the clause 6 row", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "files")].Inputs = []core.InputKind{core.InputCommitRecords}
			return d
		}, 76, "files declares the inputs [commit-records], and " + familyRecord + " clause 6 gives it " +
			"[commit-records replay-state]"},
		{"a missing version", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "files")].Version = core.Version{}
			return d
		}, 31, "files declares no version"},
	}
	for _, c := range cases {
		declared, rows, namespaces := valid()
		got := declarationViolations(c.breaks(declared), rows, namespaces)
		found := false
		for _, v := range got {
			if v.record == c.record && strings.Contains(v.text, c.want) {
				found = true
			}
		}
		if !found {
			report(t, 64, "%s was not reported as ADR-%04d %q; the checker reported: %v",
				c.name, c.record, c.want, got)
		}
	}
}

// TestFamilyDeclarationCatalogueParsing exercises the namespace parsing the
// checker depends on, so that it cannot pass because the parser matched
// nothing.
func TestFamilyDeclarationCatalogueParsing(t *testing.T) {
	t.Parallel()
	content := "## 1. Shared\n\n**Namespace:** `not_a_family`.\n\n" +
		"## 2. `alpha`\n\n**Input:** commit-records. **Namespace:** `alpha`.\n\n" +
		"## 3. `beta-gamma`\r\n\r\n**Input:** worktree. **Namespace:** `beta_gamma`.\r\n\r\n" +
		"## 4. `delta`\n\nNo namespace here.\n\n## 5. Other\n\n**Namespace:** `epsilon`.\n"
	got := catalogueNamespaces(content)
	want := map[string]string{"alpha": "alpha", "beta-gamma": "beta_gamma"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		report(t, 64, "namespaces = %v, want %v", got, want)
	}
}

// TestFamilyDeclarationRecordParsing exercises the row parsing the input check
// depends on.
func TestFamilyDeclarationRecordParsing(t *testing.T) {
	t.Parallel()
	content := "   | Family | Inputs |\n   |---|---|\n   | `temporal` | commit-records |\r\n" +
		"   | `ai-archaeology` | commit-records, replay-state |\n\nProse `not-a-row`.\n"
	got := recordInputs(content)
	want := map[string][]core.InputKind{
		"temporal":       {core.InputCommitRecords},
		"ai-archaeology": {core.InputCommitRecords, core.InputReplayState},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		report(t, 64, "rows = %v, want %v", got, want)
	}
	if !sameInputs(want["ai-archaeology"], []core.InputKind{core.InputReplayState, core.InputCommitRecords}) ||
		sameInputs(want["ai-archaeology"], []core.InputKind{core.InputReplayState, core.InputReplayState}) {
		report(t, 64, "sameInputs does not compare input lists as sets")
	}
}
