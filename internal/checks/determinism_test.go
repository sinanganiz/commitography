package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// metadataSection is the path of the generation metadata in the report.
const metadataSection = "metadata"

// withoutGenerationMetadata removes the section ADR-0021 clause 6 designates
// for generation metadata, by its path: the top-level "metadata" object,
// whatever it holds. Anything that is not a JSON object, such as a refusal, is
// returned unchanged. Every other top-level value is kept as the exact bytes
// the report carries, so a difference in formatting or ordering inside it is
// still a difference (ADR-0021 clause 4 requires byte identity).
func withoutGenerationMetadata(report string) string {
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(report), &document); err != nil {
		return report
	}
	keys := make([]string, 0, len(document))
	for key := range document {
		if key != metadataSection {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%q: %s\n", key, document[key])
	}
	return b.String()
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
			if a == b && strings.Contains(a, `"generated_at"`) {
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
	run := func(at string, days string) string {
		return `{
  "document_version": {"major": 1, "minor": 0},
  "metadata": {"generated_at": "` + at + `", "tool_version": "dev"},
  "families": {"temporal": {"metrics": {"longest_silence_days": ` + days + `}}}
}
`
	}
	if diff := sameInputDifference(run("2026-09-11T12:00:00Z", "3"), run("2026-09-12T13:00:00Z", "3")); diff != "" {
		report(t, 21, "the determinism checker refused a difference inside the generation metadata:\n%s", diff)
	}
	if sameInputDifference(run("2026-09-11T12:00:00Z", "3"), run("2026-09-12T13:00:00Z", "4")) == "" {
		report(t, 64, "the determinism checker accepted a clock-dependent value outside the generation "+
			"metadata, so it cannot catch one")
	}
}
