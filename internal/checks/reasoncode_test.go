package checks

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
)

const (
	metricsCatalogue  = "docs/metrics.md"
	reasonEnumeration = "internal/core/reason.go"
	errorClasses      = "internal/core/errors.go"
	reasonSection     = "## 13. Reason codes"
)

// TestReasonCodeCatalogue enforces ADR-0062 clause 6 in both directions
// (WP-0006 clause 2a).
//
// Forward: nothing can emit a code the catalogue does not list. The
// enumeration in internal/core is the only place a code is written down in Go,
// which TestReasonCodeConstruction below is what makes true, so comparing the
// enumeration with section 13 covers every emission site at once.
//
// Backward: the catalogue lists no code the enumeration lacks, so a code
// cannot be documented and unreachable.
func TestReasonCodeCatalogue(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	documented, _ := reasonCodes(t, repo)
	declared := declaredReasons(t, repo)

	enumerated := map[string]bool{}
	for _, r := range core.Reasons() {
		enumerated[string(r)] = true
	}

	for code := range declared {
		if !documented[code] {
			report(t, 62, "%s declares the reason code %q, which %s section 13 does not list; "+
				"a code the catalogue does not define does not exist", reasonEnumeration, code, metricsCatalogue)
		}
	}
	for code := range enumerated {
		if !documented[code] {
			report(t, 62, "core.Reasons returns the reason code %q, which %s section 13 does not list",
				code, metricsCatalogue)
		}
	}
	for code := range documented {
		if !declared[code] {
			report(t, 62, "%s section 13 lists the reason code %q, which %s does not declare, "+
				"so nothing can emit it", metricsCatalogue, code, reasonEnumeration)
		}
		if !enumerated[code] {
			report(t, 62, "%s section 13 lists the reason code %q, which core.Reasons omits, "+
				"so it is invisible to every consumer of the enumeration", metricsCatalogue, code)
		}
	}
}

// TestReasonCodeEmission is the other half of "no listed code is unreachable":
// a code must have a construction site, not merely a constant.
//
// It is narrowed to the user error codes. The family status codes are declared
// by this package and emitted by the report document, which does not exist yet;
// **WP-0008 widens this checker to them** when it puts every family in the
// report with a status (ADR-0064 clause 5). Without that narrowing the checker
// would fail for a reason the tree cannot fix, and the usual response to that
// is to delete the checker.
func TestReasonCodeEmission(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	_, userCodes := reasonCodes(t, repo)
	declared := declaredReasons(t, repo)
	emitted := emittedReasons(t, repo, declared)

	if len(userCodes) == 0 {
		fatal(t, 64, "no user error codes were found in %s section 13; the checker would pass vacuously",
			metricsCatalogue)
	}
	for code := range userCodes {
		if !emitted[code] {
			report(t, 62, "%s section 13 lists the user error code %q but no core.NewUserError call emits it; "+
				"either a condition is unclassified or the code should not be listed", metricsCatalogue, code)
		}
	}
}

// TestReasonCodeConstruction keeps the enumeration the only source of codes.
//
// A reason code reaching a user error as a string literal, or a user error
// built as a composite literal, would both bypass the catalogue comparison
// above and make it vacuous.
func TestReasonCodeConstruction(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	constants := reasonConstants(t, repo)
	call := regexp.MustCompile(`NewUserError\(\s*(?:core\.)?([A-Za-z0-9_.]+|"[^"]*")`)
	literal := regexp.MustCompile(`&(?:core\.)?(UserError|InternalError)\{`)
	// The leading character class keeps the pattern from matching the tail of
	// a longer identifier such as ValidReason.
	conversion := regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(?:core\.)?Reason\("`)

	sites := 0
	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		content := repo.read(t, 41, file)
		// Two files are skipped for the call rule. internal/core/errors.go
		// declares the constructor, so its parameter list looks like a call
		// site, and a test may table over reasons, which cannot put a code
		// into a shipped artifact. Production code has neither excuse.
		if !strings.HasSuffix(file, "_test.go") && file != errorClasses {
			for _, m := range call.FindAllStringSubmatch(content, -1) {
				sites++
				if !strings.HasPrefix(m[1], "Reason") {
					report(t, 41, "%s passes %s to NewUserError; the first argument must be a Reason constant, "+
						"so that the catalogue checker can see which codes the tree emits", file, m[1])
					continue
				}
				if _, ok := constants[m[1]]; !ok {
					report(t, 41, "%s passes the unknown constant %s to NewUserError", file, m[1])
				}
			}
		}
		// The two classes are declared where their constructors are, so that
		// one file is the only place a composite literal is legitimate.
		if file == reasonEnumeration || file == errorClasses {
			continue
		}
		if m := literal.FindStringSubmatch(content); m != nil {
			report(t, 41, "%s builds a %s as a composite literal; use the constructor, which is what "+
				"records the reason code and the remedy", file, m[1])
		}
		if conversion.MatchString(content) {
			report(t, 62, "%s converts a string to a Reason; a code exists only as a constant declared in %s",
				file, reasonEnumeration)
		}
	}
	if sites == 0 {
		fatal(t, 64, "no NewUserError call was found in the tree; the checker would pass vacuously")
	}
}

// reasonCodes returns every code in section 13, and the subset the section's
// table marks as a user error code.
//
// Section 13 states that every backticked token in it is a code and that
// nothing else in it is backticked, which is what makes it readable without
// parsing prose.
func reasonCodes(t *testing.T, repo repository) (all, user map[string]bool) {
	t.Helper()
	section := catalogueSection(t, repo, reasonSection)
	all, user = map[string]bool{}, map[string]bool{}

	code := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	for _, m := range code.FindAllStringSubmatch(section, -1) {
		all[m[1]] = true
	}
	row := regexp.MustCompile("(?m)^\\| `([a-z][a-z0-9_]*)` \\|")
	for _, m := range row.FindAllStringSubmatch(section, -1) {
		user[m[1]] = true
	}
	if len(all) == 0 {
		fatal(t, 62, "%s section 13 lists no reason code; the catalogue checker has nothing to compare",
			metricsCatalogue)
	}
	return all, user
}

// catalogueSection returns one section of the metric catalogue, from its
// heading to the next heading of the same level.
func catalogueSection(t *testing.T, repo repository, heading string) string {
	t.Helper()
	content := strings.ReplaceAll(repo.read(t, 62, metricsCatalogue), "\r\n", "\n")
	start := strings.Index(content, heading)
	if start < 0 {
		fatal(t, 62, "%s has no %q section", metricsCatalogue, heading)
	}
	rest := content[start+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// reasonConstants returns the Reason constants the enumeration declares, as a
// map from constant name to code.
func reasonConstants(t *testing.T, repo repository) map[string]string {
	t.Helper()
	declaration := regexp.MustCompile(`(?m)^\s*(Reason[A-Za-z0-9]*)\s+Reason\s*=\s*"([a-z][a-z0-9_]*)"\s*$`)
	out := map[string]string{}
	for _, m := range declaration.FindAllStringSubmatch(repo.read(t, 62, reasonEnumeration), -1) {
		out[m[1]] = m[2]
	}
	if len(out) == 0 {
		fatal(t, 62, "%s declares no Reason constant; the catalogue checker has nothing to compare",
			reasonEnumeration)
	}
	return out
}

// declaredReasons returns the codes the enumeration declares.
func declaredReasons(t *testing.T, repo repository) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, code := range reasonConstants(t, repo) {
		out[code] = true
	}
	return out
}

// emittedReasons returns the codes some NewUserError call in the tree passes.
func emittedReasons(t *testing.T, repo repository, declared map[string]bool) map[string]bool {
	t.Helper()
	constants := reasonConstants(t, repo)
	call := regexp.MustCompile(`NewUserError\(\s*(?:core\.)?(Reason[A-Za-z0-9]*)`)
	out := map[string]bool{}
	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, ".go") || strings.HasPrefix(file, "internal/checks/") {
			continue
		}
		for _, m := range call.FindAllStringSubmatch(repo.read(t, 62, file), -1) {
			if code, ok := constants[m[1]]; ok && declared[code] {
				out[code] = true
			}
		}
	}
	return out
}

// TestReasonCodeHelpers exercises the parsing the checkers above depend on, so
// that a checker cannot pass because its parser silently matched nothing.
func TestReasonCodeHelpers(t *testing.T) {
	t.Parallel()
	code := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	row := regexp.MustCompile("(?m)^\\| `([a-z][a-z0-9_]*)` \\|")
	section := "some prose with `first_code` and `second`.\n\n" +
		"| Code | Condition |\n|---|---|\n| `third_code` | A condition |\n"

	var found []string
	for _, m := range code.FindAllStringSubmatch(section, -1) {
		found = append(found, m[1])
	}
	sort.Strings(found)
	if want := "first_code second third_code"; strings.Join(found, " ") != want {
		t.Errorf("codes = %q, want %q", strings.Join(found, " "), want)
	}
	rows := row.FindAllStringSubmatch(section, -1)
	if len(rows) != 1 || rows[0][1] != "third_code" {
		t.Errorf("table rows = %v, want one row naming third_code", rows)
	}

	declaration := regexp.MustCompile(`(?m)^\s*(Reason[A-Za-z0-9]*)\s+Reason\s*=\s*"([a-z][a-z0-9_]*)"\s*$`)
	got := declaration.FindAllStringSubmatch("\tReasonExample Reason = \"example_code\"\n", -1)
	if len(got) != 1 || got[0][1] != "ReasonExample" || got[0][2] != "example_code" {
		t.Errorf("constant declaration = %v, want ReasonExample/example_code", got)
	}

	call := regexp.MustCompile(`NewUserError\(\s*(?:core\.)?(Reason[A-Za-z0-9]*)`)
	for _, in := range []string{
		`core.NewUserError(core.ReasonEmptyRepository, "", "r", "s")`,
		"NewUserError(\n\t\tReasonShallowClone, \"\", \"r\", \"s\")",
	} {
		m := call.FindStringSubmatch(in)
		if m == nil {
			t.Errorf("no constant found in %q", in)
		}
	}
}
