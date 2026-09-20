package checks

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline"
)

// Two checkers over the report's top-level sections.
//
// The operational leak checker (WP-0010 clause 8): no operational value
// reaches a report. The planes exist so that a server can offer configuration
// flexibility without becoming a source of hidden divergence (ADR-0026
// clause 1), and a value that changes no number has no place in a document
// that records what produced the numbers.
//
// The section version checker (ADR-0070 clause 4): a section's values may not
// change meaning under a version that stayed put, or a stored report would be
// read as though the newer derivation had produced it. The golden files are
// where such a change becomes visible, so the check reads their history.

// operationalLeaks returns every place a document carries an operational key,
// at any depth.
func operationalLeaks(value any, path string, operational map[string]bool) []string {
	var out []string
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			where := path + "." + key
			if operational[key] {
				out = append(out, where)
			}
			out = append(out, operationalLeaks(typed[key], where, operational)...)
		}
	case []any:
		for _, element := range typed {
			out = append(out, operationalLeaks(element, path, operational)...)
		}
	}
	return out
}

func operationalKeys() map[string]bool {
	out := map[string]bool{}
	for _, key := range config.OperationalKeys() {
		out[key] = true
	}
	return out
}

// TestOperationalLeakInReports requires no report to carry an operational key,
// and the configuration section to carry the analysis keys and nothing else.
func TestOperationalLeakInReports(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	operational := operationalKeys()
	if len(operational) == 0 {
		fatal(t, 64, "the operational plane names no value, so the scan for one proves nothing")
	}
	analysis := map[string]bool{}
	for _, key := range config.AnalysisKeys() {
		analysis[key] = true
	}

	scanned := 0
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, goldenDir+"/") || !strings.HasSuffix(file, ".json") {
			continue
		}
		var document map[string]any
		if err := json.Unmarshal([]byte(repo.read(t, 26, file)), &document); err != nil {
			report(t, 26, "%s is not a report: %v", file, err)
			continue
		}
		scanned++
		for _, where := range operationalLeaks(document, "$", operational) {
			report(t, 26, "%s carries the operational value %s; the operational plane never reaches a report "+
				"(ADR-0026 clause 1)", file, where)
		}
		section, ok := document["configuration"].(map[string]any)
		if !ok {
			report(t, 26, "%s carries no configuration section", file)
			continue
		}
		for _, key := range sortedKeys(section) {
			if !analysis[key] {
				report(t, 26, "%s carries the configuration key %s, which the analysis plane does not have, "+
					"so it cannot be handed back to the command", file, key)
			}
		}
	}
	if scanned == 0 {
		fatal(t, 64, "no golden report is tracked under %s, so there is nothing to scan", goldenDir)
	}
}

// TestOperationalLeakChangesNoMetric is the other half of ADR-0026 clause 1:
// an operational value changes no number. The shallow-clone override is the
// one an analysis reads, so two runs that differ only in it produce one
// report.
func TestOperationalLeakChangesNoMetric(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := repo.root + "/testdata/fixtures/basic"
	clock := core.FixedClock(checkTime())
	allowed := analyseWith(t, pipeline.Options{RepoPath: dir, AllowShallow: true}, clock)
	refused := analyseWith(t, pipeline.Options{RepoPath: dir, AllowShallow: false}, clock)
	if diff := sameInputDifference(allowed, refused); diff != "" {
		report(t, 26, "an operational value changed the report (- allowed, + not allowed):\n%s", diff)
	}
}

// TestOperationalLeakRejectsALeakedValue is the failure demonstration ADR-0064
// clause 6 requires: an operational key in a report, at the top level or
// inside the configuration section, is found.
func TestOperationalLeakRejectsALeakedValue(t *testing.T) {
	t.Parallel()
	operational := operationalKeys()
	for name, document := range map[string]string{
		"a top-level operational key":       `{"configuration":{},"output_dir":"./out"}`,
		"one inside the configuration":      `{"configuration":{"output_dir":"./out"}}`,
		"one inside a list in the document": `{"identities":[{"allowed_roots":["/srv"]}]}`,
	} {
		var value any
		if err := json.Unmarshal([]byte(document), &value); err != nil {
			fatal(t, 64, "decoding a deliberately leaked report: %v", err)
		}
		if len(operationalLeaks(value, "$", operational)) == 0 {
			report(t, 64, "the operational scan accepted %s, so it cannot catch one", name)
		}
	}
	var clean any
	if err := json.Unmarshal([]byte(`{"configuration":{"exclude_paths":["out/**"],"year":0}}`), &clean); err != nil {
		fatal(t, 64, "decoding a clean report: %v", err)
	}
	if found := operationalLeaks(clean, "$", operational); len(found) != 0 {
		report(t, 26, "the operational scan refused a report that carries no operational value: %v", found)
	}
}

// sectionsOf returns a report's top-level sections and the versions it states
// for them, or ok false when the text is not a report.
func sectionsOf(document string) (sections map[string]json.RawMessage, versions map[string]string, ok bool) {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(document), &parsed); err != nil {
		return nil, nil, false
	}
	sections = map[string]json.RawMessage{}
	for _, name := range []string{"metadata", "repository", "configuration", "identities"} {
		if value, present := parsed[name]; present {
			sections[name] = value
		}
	}
	versions = map[string]string{"document": string(parsed["document_version"])}
	if stated, present := parsed["sections"]; present {
		var each map[string]json.RawMessage
		if err := json.Unmarshal(stated, &each); err == nil {
			for name, value := range each {
				versions[name] = string(value)
			}
		}
	}
	return sections, versions, true
}

// TestSectionVersionsMoveWithTheirDerivation reads the history of the golden
// files: where a commit changed what a section holds, that section's version
// moved, or the document version did, because a structural change moves that
// one instead.
//
// The metadata section is left out: its two values are normalised away in the
// golden files, so a change to them is invisible here by design.
func TestSectionVersionsMoveWithTheirDerivation(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	ctx := context.Background()
	show := func(revision string) (string, bool) {
		out, err := git.Output(ctx, git.At(repo.root, "show", revision))
		return out, err == nil
	}

	commits, err := git.Records(ctx, git.At(repo.root, "log", "-z", "--no-merges",
		"--format=%H").Pathspecs(goldenDir))
	if err != nil {
		fatal(t, 70, "cannot read the history of %s: %v", goldenDir, err)
	}
	compared := 0
	for _, commit := range commits {
		// A commit that changes the fixtures changes what the reports
		// describe, not how they are derived, so its golden diff says nothing
		// about versions.
		if changed, err := git.Records(ctx, git.At(repo.root, "diff-tree", "--no-commit-id", "--name-only",
			"-r", "-z", commit).Pathspecs("testdata/build-fixtures.sh", "testdata/fixture-hashes.txt")); err != nil || len(changed) > 0 {
			continue
		}
		files, err := git.Records(ctx, git.At(repo.root, "diff-tree", "--no-commit-id", "--name-only",
			"-r", "-z", commit).Pathspecs(goldenDir))
		if err != nil {
			fatal(t, 70, "cannot read what commit %.12s changed: %v", commit, err)
		}
		for _, file := range files {
			if !strings.HasSuffix(file, ".json") {
				continue
			}
			before, hadBefore := show(commit + "^:" + file)
			after, hadAfter := show(commit + ":" + file)
			if !hadBefore || !hadAfter {
				// The file was added or removed; there is no pair to compare.
				continue
			}
			oldSections, oldVersions, okBefore := sectionsOf(before)
			newSections, newVersions, okAfter := sectionsOf(after)
			if !okBefore || !okAfter {
				continue
			}
			compared++
			if oldVersions["document"] != newVersions["document"] {
				continue
			}
			for name, oldValue := range oldSections {
				if name == "metadata" {
					continue
				}
				newValue, present := newSections[name]
				if !present || string(oldValue) == string(newValue) {
					continue
				}
				if oldVersions[name] == newVersions[name] {
					report(t, 70, "commit %.12s changes the %s section of %s while its version stays %s; a "+
						"section's values may not change meaning under a version that stayed put",
						commit, name, file, newVersions[name])
				}
			}
		}
	}
	if compared == 0 {
		fatal(t, 64, "no pair of golden versions was compared, so the check proves nothing")
	}
}

// TestSectionVersionsRejectAChangedDerivation is the failure demonstration:
// the comparison the checker performs must refuse a section whose values
// changed under a version that did not, and accept one whose version moved.
func TestSectionVersionsRejectAChangedDerivation(t *testing.T) {
	t.Parallel()
	document := func(sectionVersion, entry string) string {
		return `{"document_version":{"major":1,"minor":4},` +
			`"sections":{"identities":` + sectionVersion + `},` +
			`"identities":[` + entry + `],"configuration":{},"metadata":{},"repository":{}}`
	}
	const (
		first  = `{"major":2,"minor":0}`
		second = `{"major":2,"minor":1}`
		entryA = `{"id":"a","merge_candidates":[]}`
		entryB = `{"id":"a","merge_candidates":[{"id":"b","signal":"local_part"}]}`
	)

	changed := func(before, after string) bool {
		oldSections, oldVersions, okBefore := sectionsOf(before)
		newSections, newVersions, okAfter := sectionsOf(after)
		if !okBefore || !okAfter {
			fatal(t, 64, "the checker's own documents are not reports")
		}
		return string(oldSections["identities"]) != string(newSections["identities"]) &&
			oldVersions["identities"] == newVersions["identities"]
	}
	if !changed(document(first, entryA), document(first, entryB)) {
		report(t, 64, "the section check accepted a derivation change under an unchanged version, so it "+
			"cannot catch one")
	}
	if changed(document(first, entryA), document(second, entryB)) {
		report(t, 70, "the section check refused a derivation change whose version moved with it")
	}
	if changed(document(first, entryA), document(first, entryA)) {
		report(t, 70, "the section check refused a section that did not change")
	}
}

// TestSectionVersionsCoverEverySection keeps the checker's section list and
// the report's own sections together, so a section added later is compared too.
func TestSectionVersionsCoverEverySection(t *testing.T) {
	t.Parallel()
	sections, _, ok := sectionsOf(`{"metadata":{},"repository":{},"configuration":{},"identities":[],` +
		`"document_version":{},"sections":{},"families":{}}`)
	if !ok {
		fatal(t, 64, "the checker cannot read a report")
	}
	report := reflect.TypeOf(core.Report{})
	expected := 0
	for i := 0; i < report.NumField(); i++ {
		name, _, _ := strings.Cut(report.Field(i).Tag.Get("json"), ",")
		switch name {
		case "document_version", "sections", "families":
			continue
		}
		expected++
		if _, present := sections[name]; !present {
			fatal(t, 70, "the section %s is not among the sections this checker compares", name)
		}
	}
	if len(sections) != expected {
		fatal(t, 70, "the checker compares %d sections and the report has %d", len(sections), expected)
	}
}
