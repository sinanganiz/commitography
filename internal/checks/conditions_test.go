package checks

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

const fixtureConditions = "testdata/fixture-conditions.txt"

// requiredConditions are the conditions WP-0004 clause 4 requires the fixture
// set to cover, in the identifiers testdata/fixture-conditions.txt uses.
func requiredConditions() []string {
	return []string{
		"single-contributor",
		"one-person-two-addresses",
		"renames-and-copied-block",
		"hostile-names",
		"above-outlier-threshold",
		"generated-and-vendored-paths",
		"agent-coauthor-trailers",
		"multi-year-gap",
		"shallow-clone",
		"large-fixture",
	}
}

// largeCondition marks the fixture compared in the full gate only.
const largeCondition = "large-fixture"

// TestFixtureConditions enforces ADR-0019 through the fixture condition list:
// every required condition has a fixture, every generated fixture exercises a
// condition, the list names no fixture the generator does not produce, and
// exactly one fixture is designated large.
//
// The generated set is read from the committed hash list, which
// TestFixtureDeterminism keeps equal to what the generator produces, so this
// checker reads tracked files only (ADR-0063 clause 4).
func TestFixtureConditions(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixtures := generatedFixtures(t, repo)
	conditions := loadFixtureConditions(t, repo)

	generated := map[string]bool{}
	for _, f := range fixtures {
		generated[f] = true
		if len(conditions[f]) == 0 {
			report(t, 19, "fixture %s has no condition in %s; state what it exercises", f, fixtureConditions)
		}
	}
	covered := map[string][]string{}
	for f, cs := range conditions {
		if !generated[f] {
			report(t, 19, "%s names fixture %s, which is not in %s", fixtureConditions, f, fixtureHashes)
		}
		for _, c := range cs {
			covered[c] = append(covered[c], f)
		}
	}
	for _, c := range requiredConditions() {
		if len(covered[c]) == 0 {
			report(t, 19, "no fixture in %s covers the required condition %s (WP-0004 clause 4)", fixtureConditions, c)
		}
	}
	if large := covered[largeCondition]; len(large) != 1 {
		sort.Strings(large)
		report(t, 19, "exactly one fixture must be %s; %s names %d: %v", largeCondition, fixtureConditions, len(large), large)
	}
}

// generatedFixtures returns the fixture names in the committed hash list, in
// list order.
func generatedFixtures(t *testing.T, repo repository) []string {
	t.Helper()
	if !repo.isTracked(fixtureHashes) {
		fatal(t, 19, "%s is not tracked", fixtureHashes)
	}
	var names []string
	for _, line := range lines(repo.read(t, 19, fixtureHashes)) {
		if f := strings.Fields(line); len(f) == 3 && f[1] == "HEAD" {
			names = append(names, f[0])
		}
	}
	if len(names) == 0 {
		fatal(t, 19, "%s lists no fixture", fixtureHashes)
	}
	return names
}

// loadFixtureConditions maps each fixture to the conditions it exercises.
func loadFixtureConditions(t *testing.T, repo repository) map[string][]string {
	t.Helper()
	if !repo.isTracked(fixtureConditions) {
		fatal(t, 19, "%s is not tracked", fixtureConditions)
	}
	identifier := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	out := map[string][]string{}
	for i, line := range lines(repo.read(t, 19, fixtureConditions)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 || !identifier.MatchString(f[0]) || !identifier.MatchString(f[1]) {
			report(t, 19, "%s:%d: want \"<fixture> <condition>\" in kebab case, got %q", fixtureConditions, i+1, line)
			continue
		}
		for _, c := range out[f[0]] {
			if c == f[1] {
				report(t, 19, "%s:%d: %s %s is listed twice", fixtureConditions, i+1, f[0], f[1])
			}
		}
		out[f[0]] = append(out[f[0]], f[1])
	}
	return out
}
