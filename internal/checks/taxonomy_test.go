package checks

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	taxonomyFile = "internal/pipeline/interpret/taxonomy/taxonomy.yml"
	axesFile     = "internal/pipeline/interpret/taxonomy/axes.md"
)

type taxonomyEntry struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	When        map[string]any `yaml:"when"`
}

type taxonomyDocument struct {
	Archetypes map[string][]taxonomyEntry `yaml:"archetypes"`
	Badges     []taxonomyEntry            `yaml:"badges"`
}

// taxonomySubjects are the two archetype lists (ADR-0023).
func taxonomySubjects() []string { return []string{"person", "repository"} }

// TestTaxonomyIntegrity enforces the shape of the taxonomy definition file:
// every archetype has an identifier and a name (ADR-0030 clause 5) and a
// default description (ADR-0039 clause 5), identifiers are unique across the
// file (ADR-0030 clause 5), and every condition names an axis defined in
// axes.md (ADR-0030 clause 2).
func TestTaxonomyIntegrity(t *testing.T) {
	repo := openRepository(t)

	var doc taxonomyDocument
	if err := yaml.Unmarshal([]byte(repo.read(t, 30, taxonomyFile)), &doc); err != nil {
		fatal(t, 30, "%s does not parse: %v", taxonomyFile, err)
	}
	axes, prefixes := definedAxes(repo.read(t, 30, axesFile))
	if len(axes) == 0 {
		fatal(t, 30, "%s defines no axes", axesFile)
	}

	seen := map[string]string{}
	claim := func(id, where string) {
		if id == "" {
			return
		}
		if first, dup := seen[id]; dup {
			report(t, 30, "%s: identifier %q in %s is already used in %s", taxonomyFile, id, where, first)
			return
		}
		seen[id] = where
	}
	checkAxes := func(entry taxonomyEntry, where string) {
		names := make([]string, 0, len(entry.When))
		for axis := range entry.When {
			names = append(names, axis)
		}
		sort.Strings(names)
		for _, axis := range names {
			if !axes[axis] && !hasAxisPrefix(axis, prefixes) {
				report(t, 30, "%s: %s refers to axis %q, which %s does not define", taxonomyFile, where, axis, axesFile)
			}
		}
	}

	for _, subject := range taxonomySubjects() {
		list, ok := doc.Archetypes[subject]
		if !ok || len(list) == 0 {
			report(t, 30, "%s has no archetypes.%s list", taxonomyFile, subject)
			continue
		}
		for i, a := range list {
			where := describeEntry("archetypes."+subject, i, a.ID)
			if strings.TrimSpace(a.ID) == "" {
				report(t, 30, "%s: %s has an empty id", taxonomyFile, where)
			}
			if strings.TrimSpace(a.Name) == "" {
				report(t, 30, "%s: %s has an empty name", taxonomyFile, where)
			}
			if strings.TrimSpace(a.Description) == "" {
				report(t, 39, "%s: %s has an empty default description", taxonomyFile, where)
			}
			claim(a.ID, where)
			checkAxes(a, where)
		}
	}
	for subject := range doc.Archetypes {
		if subject != "person" && subject != "repository" {
			report(t, 23, "%s: archetypes.%s is not a Wrapped subject", taxonomyFile, subject)
		}
	}
	for i, b := range doc.Badges {
		where := describeEntry("badges", i, b.ID)
		if strings.TrimSpace(b.ID) == "" {
			report(t, 30, "%s: %s has an empty id", taxonomyFile, where)
		}
		claim(b.ID, where)
		checkAxes(b, where)
	}
}

func describeEntry(list string, index int, id string) string {
	if id == "" {
		return list + "[" + strconv.Itoa(index) + "]"
	}
	return list + "[" + strconv.Itoa(index) + "] (" + id + ")"
}

// definedAxes reads the axis names from the first column of every table in
// axes.md. A name ending in "_*" defines a family of axes by prefix.
func definedAxes(content string) (map[string]bool, []string) {
	names := map[string]bool{}
	var prefixes []string
	code := regexp.MustCompile("`([a-z0-9_]+\\*?)`")
	for _, line := range lines(content) {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		for _, m := range code.FindAllStringSubmatch(cells[1], -1) {
			if strings.HasSuffix(m[1], "*") {
				prefixes = append(prefixes, strings.TrimSuffix(m[1], "*"))
			} else {
				names[m[1]] = true
			}
		}
	}
	return names, prefixes
}

func hasAxisPrefix(axis string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(axis, p) && len(axis) > len(p) {
			return true
		}
	}
	return false
}
