package checks

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/metrics/aiarchaeology"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/coupling"
	"github.com/sinanganiz/commitography/internal/metrics/files"
	"github.com/sinanganiz/commitography/internal/metrics/hotspot"
	"github.com/sinanganiz/commitography/internal/metrics/messages"
	"github.com/sinanganiz/commitography/internal/metrics/ownership"
	"github.com/sinanganiz/commitography/internal/metrics/staticanalysis"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
	"github.com/sinanganiz/commitography/internal/metrics/worktype"
)

// The family declaration checker (ADR-0024 clause 1, ADR-0031 clause 2,
// ADR-0062 clause 3): every family of ADR-0024 clause 5 has a package that
// declares its contract, and every declaration agrees with the catalogue.
//
// It checks the declarations, not the report. Nothing routes a family's output
// through its declaration yet, so the report's keys are not compared here.

// familyDeclarations is the checker's own list of the ten family packages.
// WP-0061 replaces it with the aggregate stage's registry, so that one list
// exists in the end.
func familyDeclarations() []core.MetricFamily {
	return []core.MetricFamily{
		temporal.Family{},
		commitsize.Family{},
		messages.Family{},
		files.Family{},
		coupling.Family{},
		ownership.Family{},
		worktype.Family{},
		aiarchaeology.Family{},
		hotspot.Family{},
		staticanalysis.Family{},
	}
}

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

// declarationViolation is one failure, with the record it breaks.
type declarationViolation struct {
	record int
	text   string
}

// declarationViolations compares declarations with the family set of
// ADR-0024 clause 5 and the namespaces of docs/metrics.md.
func declarationViolations(declared []core.FamilyDeclaration, families map[string]bool,
	namespaces map[string]string) []declarationViolation {
	var out []declarationViolation
	add := func(record int, format string, args ...any) {
		out = append(out, declarationViolation{record, fmt.Sprintf(format, args...)})
	}

	byName := map[string]bool{}
	for _, d := range declared {
		byName[d.Name] = true
	}
	for _, f := range sortedKeys(families) {
		if !byName[f] {
			add(24, "the family %s of %s clause 5 has no declaring package", f, familyRecord)
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
			add(24, "the family %s declares no input", d.Name)
		}
		for _, in := range d.Inputs {
			if other, ok := owner[string(in)]; ok && other != d.Name {
				add(24, "the family %s declares the namespace %s of the family %s as an input; a family never "+
					"reads another family's output (clause 3)", d.Name, in, other)
				continue
			}
			if !kinds[in] {
				add(24, "the family %s declares the input %q, which is not one of the four kinds %v",
					d.Name, in, core.InputKinds())
			}
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
			add(24, "the families %s all declare the namespace %q; a namespace has one owner (clause 4)",
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

// TestFamilyDeclarations holds every family's declaration to ADR-0024 clause 5
// and to the namespaces docs/metrics.md gives.
func TestFamilyDeclarations(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	families := recordFamilies(repo.read(t, 24, familyRecord))
	namespaces := catalogueNamespaces(repo.read(t, 62, metricsCatalogue))
	if len(families) == 0 || len(namespaces) == 0 {
		fatal(t, 64, "no family was read from %s or no namespace from %s; the checker would pass vacuously",
			familyRecord, metricsCatalogue)
	}

	declared := declarationsOf(familyDeclarations())
	for _, d := range declared {
		t.Logf("%-15s inputs %v namespace %s version %s status %s method %t",
			d.Name, d.Inputs, d.Namespace, d.Version, d.Status, d.Method != "")
	}
	for _, v := range declarationViolations(declared, families, namespaces) {
		report(t, v.record, "%s", v.text)
	}
}

// TestFamilyDeclarationRejectsEachViolation is the failure demonstration
// ADR-0064 clause 6 requires. Starting from a set of declarations that
// passes, it breaks one thing at a time and requires the checker to report
// it, under the record it breaks.
func TestFamilyDeclarationRejectsEachViolation(t *testing.T) {
	t.Parallel()
	valid := func() ([]core.FamilyDeclaration, map[string]bool, map[string]string) {
		declared := declarationsOf(familyDeclarations())
		families := map[string]bool{}
		namespaces := map[string]string{}
		for _, d := range declared {
			families[d.Name] = true
			namespaces[d.Name] = d.Namespace
		}
		return declared, families, namespaces
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
		}, 24, "coupling of " + familyRecord + " clause 5 has no declaring package"},
		{"an input outside the four kinds", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "temporal")].Inputs = []core.InputKind{"commit-log"}
			return d
		}, 24, `temporal declares the input "commit-log", which is not one of the four kinds`},
		{"no input", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "messages")].Inputs = nil
			return d
		}, 24, "messages declares no input"},
		{"a namespace the catalogue does not give", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "commit-size")].Namespace = "commit-size"
			return d
		}, 62, `commit-size declares the namespace "commit-size", and docs/metrics.md gives "commit_size"`},
		{"two families declare one namespace", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "hotspot")].Namespace = "files"
			return d
		}, 24, `files, hotspot all declare the namespace "files"`},
		{"another family's namespace as an input", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "hotspot")].Inputs = []core.InputKind{core.InputCommitRecords, "coupling"}
			return d
		}, 24, "hotspot declares the namespace coupling of the family coupling as an input"},
		{"a missing version", func(d []core.FamilyDeclaration) []core.FamilyDeclaration {
			d[index(d, "files")].Version = core.Version{}
			return d
		}, 31, "files declares no version"},
	}
	for _, c := range cases {
		declared, families, namespaces := valid()
		got := declarationViolations(c.breaks(declared), families, namespaces)
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
