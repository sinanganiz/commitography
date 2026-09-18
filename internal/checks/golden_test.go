package checks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

const goldenDir = "testdata/golden"

// TestGoldenSmall enforces ADR-0019 clause 2 for every fixture except the
// designated large one, and checks that the golden set matches the fixture
// set. It belongs to the fast gate (ADR-0057 clause 2).
//
// With COMMITOGRAPHY_UPDATE_GOLDEN=1 the golden files are rewritten instead of
// compared. A commit that changes one states why in its body; the golden
// commit message checker enforces that.
func TestGoldenSmall(t *testing.T) {
	repo := openRepository(t)
	large := largeFixture(t, repo)
	fixtures := generatedFixtures(t, repo)
	checkGoldenSet(t, repo, fixtures)
	for _, f := range fixtures {
		if f != large {
			compareGolden(t, repo, f)
		}
	}
}

// TestGoldenLarge is the full-gate half of ADR-0019 clause 2: the designated
// large fixture (ADR-0057 clause 2).
func TestGoldenLarge(t *testing.T) {
	repo := openRepository(t)
	compareGolden(t, repo, largeFixture(t, repo))
}

func largeFixture(t *testing.T, repo repository) string {
	t.Helper()
	for f, cs := range loadFixtureConditions(t, repo) {
		for _, c := range cs {
			if c == largeCondition {
				return f
			}
		}
	}
	fatal(t, 19, "%s designates no %s", fixtureConditions, largeCondition)
	return ""
}

// goldenFile returns the tracked golden file for a fixture: a report, or the
// refusal the command produced instead.
func goldenFile(fixture string, refused bool) string {
	if refused {
		return path.Join(goldenDir, fixture+".refusal.txt")
	}
	return path.Join(goldenDir, fixture+".json")
}

// checkGoldenSet requires exactly one tracked golden file per fixture and no
// golden file for anything else.
func checkGoldenSet(t *testing.T, repo repository, fixtures []string) {
	t.Helper()
	count := map[string]int{}
	for _, f := range fixtures {
		count[f] = 0
	}
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, goldenDir+"/") {
			continue
		}
		name := strings.TrimPrefix(file, goldenDir+"/")
		fixture := strings.TrimSuffix(strings.TrimSuffix(name, ".json"), ".refusal.txt")
		if n, ok := count[fixture]; !ok || fixture == name {
			report(t, 19, "%s is not the golden file of any fixture in %s", file, fixtureHashes)
		} else {
			count[fixture] = n + 1
		}
	}
	if os.Getenv("COMMITOGRAPHY_UPDATE_GOLDEN") == "1" {
		return
	}
	for _, f := range fixtures {
		if count[f] != 1 {
			report(t, 19, "fixture %s has %d tracked golden files in %s; want exactly one", f, count[f], goldenDir)
		}
	}
}

// compareGolden analyses one fixture along the path `commitography <fixture>
// --json` takes and compares the result with its golden file.
func compareGolden(t *testing.T, repo repository, fixture string) {
	t.Helper()
	t.Run(fixture, func(t *testing.T) {
		got, refused := produce(t, repo, fixture)
		file := goldenFile(fixture, refused)
		abs := filepath.Join(repo.root, filepath.FromSlash(file))
		if os.Getenv("COMMITOGRAPHY_UPDATE_GOLDEN") == "1" {
			stale := filepath.Join(repo.root, filepath.FromSlash(goldenFile(fixture, !refused)))
			if err := os.Remove(stale); err != nil && !errors.Is(err, os.ErrNotExist) {
				fatal(t, 19, "removing %s: %v", stale, err)
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				fatal(t, 19, "creating %s: %v", goldenDir, err)
			}
			if err := os.WriteFile(abs, []byte(got), 0o644); err != nil {
				fatal(t, 19, "writing %s: %v", file, err)
			}
			return
		}
		if !repo.isTracked(file) {
			fatal(t, 19, "fixture %s produced a %s, but %s is not tracked", fixture, kind(refused), file)
		}
		want := strings.ReplaceAll(repo.read(t, 19, file), "\r\n", "\n")
		if got != want {
			report(t, 19, "the analysis output of fixture %s differs from %s (- golden, + produced):\n%s\n"+
				"If the change is intended, regenerate with COMMITOGRAPHY_UPDATE_GOLDEN=1 and state why the "+
				"output changed in the commit body.", fixture, file, unifiedDiff(want, got))
		}
	})
}

func kind(refused bool) string {
	if refused {
		return "refusal"
	}
	return "report"
}

// produce runs the command's analysis path on a fixture: the pipeline root
// followed by the report writer, as `commitography <fixture> --json` runs
// them. The command itself is package main and cannot be imported, so the exit
// code is taken from core.ExitCode, which is the same single mapping the
// command uses (ADR-0041 clause 4). It returns the normalised report, or the
// normalised refusal when the command refuses.
func produce(t *testing.T, repo repository, fixture string) (string, bool) {
	t.Helper()
	root := filepath.Join(repo.root, "testdata", "fixtures")
	dir := filepath.Join(root, fixture)
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}
	out := filepath.Join(t.TempDir(), render.ReportFile)
	// OperatorSupplied mirrors the command line, so a refusal names the path
	// the way the command would (ADR-0067 clause 3); normalise then replaces
	// the fixture root, as it already does for the report.
	result, err := newAnalyzer().Run(context.Background(), pipeline.Options{RepoPath: dir, OperatorSupplied: true}, nil)
	if err != nil {
		return normalise(fmt.Sprintf("exit code %d\nError: %v\n", core.ExitCode(err), err), root), true
	}
	if err := render.WriteReportJSON(result.Report, out); err != nil {
		fatal(t, 19, "fixture %s: writing report.json: %v", fixture, err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		fatal(t, 19, "fixture %s: the analysis succeeded but wrote no report.json: %v", fixture, err)
	}
	return normalise(string(data), root), false
}

// normalise removes what legitimately differs between runs of one analysis:
// the generation time, the build's version (ADR-0061 clause 6 keeps build
// metadata out of golden comparison), and the location of the fixture root.
func normalise(s, root string) string {
	generatedAt := regexp.MustCompile(`(?m)^(\s*"generatedAt": )"[^"]*"`)
	toolVersion := regexp.MustCompile(`(?m)^(\s*"toolVersion": )"[^"]*"`)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = generatedAt.ReplaceAllString(s, `${1}"<generated-at>"`)
	s = toolVersion.ReplaceAllString(s, `${1}"<tool-version>"`)
	// JSON escapes a Windows separator, so the escaped form is replaced first.
	for _, r := range []string{strings.ReplaceAll(root, `\`, `\\`), root, filepath.ToSlash(root)} {
		s = strings.ReplaceAll(s, r, "<fixtures>")
	}
	// A refusal repeats the supplied path, whose separator is the platform's,
	// so the separator directly after the root is normalised as well. Without
	// this the golden refusals would differ between platforms.
	s = strings.ReplaceAll(s, `<fixtures>\`, "<fixtures>/")
	return s
}

// unifiedDiff lists the lines that differ, with their line numbers, using a
// longest common subsequence. Output is capped so a wholesale change stays
// readable.
func unifiedDiff(want, got string) string {
	a, b := lines(want), lines(got)
	lcs := make([][]int32, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int32, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	const limit = 200
	var out []string
	emit := func(s string) {
		if len(out) == limit {
			out = append(out, "... (diff truncated)")
		}
		if len(out) < limit {
			out = append(out, s)
		}
	}
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && j < len(b) && a[i] == b[j]:
			i++
			j++
		case j < len(b) && (i == len(a) || lcs[i][j+1] >= lcs[i+1][j]):
			emit(fmt.Sprintf("+%d: %s", j+1, b[j]))
			j++
		default:
			emit(fmt.Sprintf("-%d: %s", i+1, a[i]))
			i++
		}
	}
	return strings.Join(out, "\n")
}
