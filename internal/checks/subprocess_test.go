// Process execution in this file is permitted by ADR-0065 clause 3: it runs
// the toolchain and the built binary, each with a fixed argument vector, no
// shell and a timeout (ADR-0065 clause 4).

package checks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/git"
)

// The bounds of the subprocess count check (ADR-0063 table 2, ADR-0050
// clause 3 and clause 48's acceptance criterion: "Introducing a git
// invocation per commit fails the budget on any runner").
const (
	// maxGitProcesses bounds the git processes one analysis may start. A
	// correct implementation starts a fixed set — the preflight checks, the
	// commit enumeration, the tracked-file listing — plus one per read shard,
	// and the collector caps shards at sixteen. This is twice that total, so
	// no machine's core count can reach it while a per-commit implementation
	// passes it on the first hundred commits.
	maxGitProcesses = 64

	// maxProcessesPerCommit is the other side of the same statement, and the
	// one that scales: a per-commit implementation sits at one or above, and
	// a fixed set over twenty thousand commits sits three orders of magnitude
	// below this.
	maxProcessesPerCommit = 0.01

	// maxGrowth bounds how much the count may grow between a small history
	// and a large one. The difference is the extra read shards and nothing
	// else, so it is a constant rather than a share of the commits.
	maxGrowth = 32

	// buildTimeout and analysisTimeout bound the processes this checker
	// starts (ADR-0065 clause 4). The large fixture's analysis is the longer
	// of the two and takes seconds, not minutes.
	buildTimeout    = 5 * time.Minute
	analysisTimeout = 10 * time.Minute
)

// TestSubprocessCount enforces ADR-0063 table 2: the number of git processes
// an analysis starts does not grow proportionally to the number of commits.
// It is the check that catches an implementation that is correct and
// pathological — one git invocation per commit produces the same report and
// misses the performance requirement by two orders of magnitude.
//
// The count is measured from outside the analysis: a recording program named
// git goes first on the built binary's path, and every invocation it forwards
// leaves a record. Nothing inside the product is asked how many processes it
// started.
func TestSubprocessCount(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	binary := buildBinary(t, repo, "./cmd/commitography")

	// The smallest and the largest fixture. The commit counts differ by more
	// than two orders of magnitude, which is what makes "not proportional" a
	// statement a measurement can refute.
	small := measureGitProcesses(t, repo, binary, "single")
	large := measureGitProcesses(t, repo, binary, "large-history")

	// ADR-0050 clause 3 requires the git subprocess count to be recorded on
	// every enforcing run.
	t.Logf("git processes: %s %d for %d commits, %s %d for %d commits",
		small.fixture, small.processes, small.commits,
		large.fixture, large.processes, large.commits)

	if small.commits == 0 || large.commits == 0 {
		fatal(t, 64, "a fixture reported no commits, so the counts cannot be compared")
	}
	if large.commits/small.commits < 100 {
		fatal(t, 64, "the two fixtures differ by a factor of %d in commits, which is too little to tell "+
			"a fixed process count from a proportional one", large.commits/small.commits)
	}

	if large.processes > maxGitProcesses {
		report(t, 19, "analysing %d commits started %d git processes, over the bound of %d; the count "+
			"must follow the machine's shard count, not the commit count",
			large.commits, large.processes, maxGitProcesses)
	}
	if perCommit := float64(large.processes) / float64(large.commits); perCommit > maxProcessesPerCommit {
		report(t, 19, "analysing %d commits started %d git processes, %.4f per commit, over %.4f",
			large.commits, large.processes, perCommit, maxProcessesPerCommit)
	}
	if growth := large.processes - small.processes; growth > maxGrowth {
		report(t, 19, "going from %d to %d commits added %d git processes, over the bound of %d; the "+
			"count grows with the commit count", small.commits, large.commits, growth, maxGrowth)
	}
}

// measurement is one fixture's analysis.
type measurement struct {
	fixture   string
	commits   int
	processes int
}

// measureGitProcesses analyses a fixture with a recording program in front of
// git and returns how many invocations it saw.
func measureGitProcesses(t *testing.T, repo repository, binary, fixture string) measurement {
	t.Helper()
	dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}

	recorder := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	buildInto(t, repo, "./internal/checks/gitcount", filepath.Join(recorder, name))
	actual, err := exec.LookPath("git")
	if err != nil {
		fatal(t, 64, "git is not on the path, so its invocations cannot be counted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(recorder, "target"), []byte(actual), 0o600); err != nil {
		fatal(t, 19, "writing the recording program's target: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()
	out := t.TempDir()
	cmd := exec.CommandContext(ctx, binary, dir, "--json", "--quiet", "--output", out)
	// The recording program goes first, so every git the analysis starts is
	// this one. Nothing else about the environment changes.
	cmd.Env = append(os.Environ(), "PATH="+recorder+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Dir = repo.root
	if combined, err := cmd.CombinedOutput(); err != nil {
		fatal(t, 19, "analysing the %s fixture: %v\n%s", fixture, err, combined)
	}

	return measurement{
		fixture:   fixture,
		commits:   fixtureCommitCount(t, dir),
		processes: len(recordedInvocations(t, filepath.Join(recorder, "invocations"))),
	}
}

// fixtureCommitCount is how many commits the fixture holds, read from git
// rather than assumed, so a change to the generator cannot silently weaken
// the comparison.
func fixtureCommitCount(t *testing.T, dir string) int {
	t.Helper()
	out, err := git.Output(context.Background(), git.At(dir, "rev-list", "--count", "--all"))
	if err != nil {
		fatal(t, 19, "counting the fixture's commits: %v", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		fatal(t, 19, "reading the fixture's commit count: %v", err)
	}
	return count
}

// recordedInvocations returns what the recording program captured.
func recordedInvocations(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(t, 64, "the recording program captured nothing, so the count is not being measured: %v", err)
	}
	var out []string
	for _, record := range strings.Split(string(data), "\x00") {
		if strings.TrimSpace(record) != "" {
			out = append(out, record)
		}
	}
	if len(out) == 0 {
		fatal(t, 64, "the recording program captured no invocation, so the count is not being measured")
	}
	return out
}

// buildBinary builds a package into a temporary directory and returns the
// executable's path.
func buildBinary(t *testing.T, repo repository, pkg string) string {
	t.Helper()
	name := filepath.Base(pkg)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	buildInto(t, repo, pkg, path)
	return path
}

// buildInto builds a package to an exact path.
func buildInto(t *testing.T, repo repository, pkg, path string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", path, pkg)
	cmd.Dir = repo.root
	if out, err := cmd.CombinedOutput(); err != nil {
		fatal(t, 19, "building %s: %v\n%s", pkg, err, out)
	}
}
