package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// The determinism checker (ADR-0063 table 2), same-input half: two runs of
// one fixture produce byte-identical reports outside the generation metadata
// (ADR-0021 clause 4). The two runs read different clocks, so a value derived
// from the clock anywhere but the generation metadata makes them differ,
// which a single fixed clock would hide.
//
// The half that compares differing parallelism degrees arrives with
// parallelism in WP-0012 (ADR-0064 clause 5).

// withoutGenerationMetadata removes the fields ADR-0021 clause 6 designates as
// generation metadata: the generation time and the tool version. WP-0008
// gathers them into one section; until then they are these two top-level
// fields.
func withoutGenerationMetadata(report string) string {
	for _, field := range []string{"generatedAt", "toolVersion"} {
		report = regexp.MustCompile(`(?m)^(\s*"`+field+`": )"[^"]*"`).ReplaceAllString(report, `${1}"<generation metadata>"`)
	}
	return report
}

// analyseAt runs the command's analysis path on a fixture with the clock
// reading at, and returns the report as written, or the refusal.
func analyseAt(t *testing.T, dir string, at time.Time) string {
	t.Helper()
	result, err := newAnalyzerAt(core.FixedClock(at)).Run(context.Background(),
		pipeline.Options{RepoPath: dir, OperatorSupplied: true}, nil)
	if err != nil {
		return fmt.Sprintf("exit code %d\nError: %v\n", core.ExitCode(err), err)
	}
	out := filepath.Join(t.TempDir(), render.ReportFile)
	if err := render.WriteReportJSON(result.Report, out); err != nil {
		fatal(t, 21, "writing report.json: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		fatal(t, 21, "the analysis succeeded but wrote no report.json: %v", err)
	}
	return string(data)
}

// sameInputDifference returns a description of how two runs differ outside
// the generation metadata, or the empty string when they do not.
func sameInputDifference(first, second string) string {
	a, b := withoutGenerationMetadata(first), withoutGenerationMetadata(second)
	if a == b {
		return ""
	}
	return unifiedDiff(a, b)
}

// TestDeterminism runs every fixture twice, a day and an hour apart by the
// clock, and requires the two reports to be identical outside the generation
// metadata.
func TestDeterminism(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixtures := generatedFixtures(t, repo)
	if len(fixtures) == 0 {
		fatal(t, 64, "no fixture was generated; the gates generate them with `make fixtures`")
	}
	first := checkTime()
	second := first.Add(25 * time.Hour)
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
			if _, err := os.Stat(dir); err != nil {
				fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
			}
			a, b := analyseAt(t, dir, first), analyseAt(t, dir, second)
			if a == b && strings.Contains(a, `"generatedAt"`) {
				fatal(t, 64, "two runs at different times produced the same generation time, so the clock "+
					"did not reach the report and this check proves nothing")
			}
			if diff := sameInputDifference(a, b); diff != "" {
				report(t, 21, "two runs of fixture %s differ outside the generation metadata "+
					"(- first run, + second run):\n%s", fixture, diff)
			}
		})
	}
}

// TestDeterminismRejectsAClockDependentValue is the failure demonstration
// ADR-0064 clause 6 requires, kept as a test: a clock reading outside the
// generation metadata must be reported, and one inside it must not.
func TestDeterminismRejectsAClockDependentValue(t *testing.T) {
	t.Parallel()
	run := func(at string, age string) string {
		return "{\n  \"schemaVersion\": 1,\n  \"generatedAt\": \"" + at + "\",\n  \"toolVersion\": \"dev\",\n" +
			"  \"repository\": {\n    \"ageDays\": " + age + "\n  }\n}\n"
	}
	if diff := sameInputDifference(run("2026-09-11T12:00:00Z", "3"), run("2026-09-12T13:00:00Z", "3")); diff != "" {
		report(t, 21, "the determinism checker refused a difference inside the generation metadata:\n%s", diff)
	}
	if sameInputDifference(run("2026-09-11T12:00:00Z", "3"), run("2026-09-12T13:00:00Z", "4")) == "" {
		report(t, 64, "the determinism checker accepted a clock-dependent value outside the generation "+
			"metadata, so it cannot catch one")
	}
}
