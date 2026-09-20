package checks

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// linterConfiguration is the file ADR-0055 clause 1 puts the configurable
// rules in.
const linterConfiguration = ".golangci.yml"

// permittedProcessSites is the ADR-0065 clause 3 set, plus the git package of
// clause 1, as the paths the lint rule excludes from its denial.
//
// The third category of clause 3 — test setup creating a platform construct
// the standard library cannot express — has no site: the one test that needs
// such a construct creates it through the Windows API and starts no process.
// A category with no site gets no path, because a path here is a permission.
//
// Adding an entry requires a superseding record (ADR-0065 clause 6), so this
// list changing is the signal that one is needed.
func permittedProcessSites() []string {
	return []string{
		"!**/internal/git/**",
		"!**/internal/checks/**",
		"!**/internal/server/browser.go",
	}
}

// processExecutionImports are the standard library packages from which a
// process can be started. The lint rule denies the first by import; the other
// two carry functions that start one and are denied by call, so this checker
// treats a file importing any of them as a site that must name the record.
func processExecutionImports() []string {
	return []string{"os/exec"}
}

// TestProcessExecutionSitesNameTheirRecord enforces the reference half of
// ADR-0065 clause 4: every tracked Go file outside the git package that
// imports process execution carries a file-level reference to ADR-0065. The
// lint rule decides where such a file may exist; this checker makes each one
// say why. Build tags are ignored, so tagged verification packages are
// covered too, which is what "test files included" in clause 7 needs from a
// checker that the linter cannot give while those packages are not loaded.
func TestProcessExecutionSitesNameTheirRecord(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	sites := 0

	for _, file := range repo.tracked {
		if filepath.Ext(file) != ".go" || strings.HasPrefix(file, "internal/git/") {
			continue
		}
		content := repo.read(t, 65, file)
		parsed, err := parser.ParseFile(fset, file, content, parser.ImportsOnly)
		if err != nil {
			fatal(t, 65, "cannot parse %s: %v", file, err)
		}
		executes := false
		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			for _, execution := range processExecutionImports() {
				if path == execution {
					executes = true
				}
			}
		}
		if !executes {
			continue
		}
		sites++
		if !strings.Contains(fileComment(content), "ADR-0065") {
			report(t, 65, "%s imports process execution but its file-level comment does not name ADR-0065", file)
		}
	}

	// The browser opener and the verification packages always exist, so
	// finding none means the checker is not reading the tree (ADR-0064).
	if sites == 0 {
		fatal(t, 64, "found no process execution site outside internal/git; the checker cannot fail")
	}
}

// TestProcessExecutionSitesAreTheClosedSet enforces ADR-0065 clause 6 on the
// mechanism that grants the permission. Clause 3 is a closed list, and the
// two ways to widen it without touching a record are to add a path to the
// lint rule and to stop the rule covering test files. This checker reads the
// configuration and refuses both.
func TestProcessExecutionSitesAreTheClosedSet(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	if !repo.isTracked(linterConfiguration) {
		fatal(t, 55, "%s is not tracked, so the configured rules cannot be checked", linterConfiguration)
	}

	var configuration struct {
		Run struct {
			Tests *bool `yaml:"tests"`
		} `yaml:"run"`
		Linters struct {
			Settings struct {
				Depguard struct {
					Rules map[string]struct {
						Files []string `yaml:"files"`
						Deny  []struct {
							Pkg string `yaml:"pkg"`
						} `yaml:"deny"`
					} `yaml:"rules"`
				} `yaml:"depguard"`
			} `yaml:"settings"`
		} `yaml:"linters"`
	}
	if err := yaml.Unmarshal([]byte(repo.read(t, 65, linterConfiguration)), &configuration); err != nil {
		fatal(t, 55, "cannot read %s: %v", linterConfiguration, err)
	}

	// Clause 7: the rule covers test files.
	if configuration.Run.Tests == nil || !*configuration.Run.Tests {
		report(t, 65, "%s does not lint test files, so clause 7 is not enforced in them", linterConfiguration)
	}

	rule, ok := configuration.Linters.Settings.Depguard.Rules["process-execution"]
	if !ok {
		fatal(t, 65, "%s carries no process-execution rule, so nothing denies process execution",
			linterConfiguration)
	}

	denied := map[string]bool{}
	for _, entry := range rule.Deny {
		denied[entry.Pkg] = true
	}
	for _, execution := range processExecutionImports() {
		if !denied[execution] {
			report(t, 65, "the process-execution rule does not deny %q", execution)
		}
	}

	// The permitted set, compared in both directions so that neither a new
	// path nor a removed one passes unseen.
	permitted := map[string]bool{}
	for _, path := range rule.Files {
		if strings.HasPrefix(path, "!") {
			permitted[path] = true
		}
	}
	expected := map[string]bool{}
	for _, path := range permittedProcessSites() {
		expected[path] = true
		if !permitted[path] {
			report(t, 65, "the process-execution rule no longer permits %q, which ADR-0065 allows", path)
		}
	}
	for path := range permitted {
		if !expected[path] {
			report(t, 65, "the process-execution rule permits %q, which is not in the ADR-0065 clause 3 set; "+
				"adding a site requires a superseding record, not a lint exception (clause 6)", path)
		}
	}
}

// TestProcessExecutionSitesRejectAWidenedRule is the failure demonstration
// ADR-0064 clause 6 requires: the comparison above catches a path added to
// the rule and a path taken out of it.
func TestProcessExecutionSitesRejectAWidenedRule(t *testing.T) {
	t.Parallel()
	expected := map[string]bool{}
	for _, path := range permittedProcessSites() {
		expected[path] = true
	}
	if len(expected) == 0 {
		fatal(t, 64, "the permitted set is empty, so the comparison cannot fail")
	}
	if expected["!**/internal/pipeline/**"] {
		report(t, 65, "the permitted set contains a path ADR-0065 clause 3 does not name")
	}
	for _, path := range permittedProcessSites() {
		if !strings.HasPrefix(path, "!") {
			report(t, 65, "the permitted set entry %q is not an exclusion, so it grants nothing", path)
		}
	}
}
