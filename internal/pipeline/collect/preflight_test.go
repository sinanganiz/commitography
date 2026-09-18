package collect

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
)

func TestPreflightBasicRepository(t *testing.T) {
	repo := fixture(t, "basic")
	info, err := newCollector().Preflight(context.Background(), repo, repo)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if info.Name != "basic" {
		t.Errorf("Name = %q, want %q", info.Name, "basic")
	}
	if info.IsShallow {
		t.Error("basic fixture reported as shallow")
	}
	if info.HasGrafts {
		t.Error("basic fixture reported as grafted")
	}
	if len(info.HeadCommit) != 40 {
		t.Errorf("HeadCommit = %q, want a full 40-character hash", info.HeadCommit)
	}
	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %q, want %q", info.DefaultBranch, "main")
	}
	if !filepath.IsAbs(info.Path) {
		t.Errorf("Path = %q, want an absolute path", info.Path)
	}
}

func TestPreflightDetectsShallowClone(t *testing.T) {
	shallow := fixture(t, "shallow")
	info, err := newCollector().Preflight(context.Background(), shallow, shallow)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if !info.IsShallow {
		t.Error("shallow fixture not detected as shallow")
	}
	if !info.HasGrafts {
		t.Error("shallow fixture should have a grafts/shallow file")
	}
}

func TestShallowErrorMessage(t *testing.T) {
	err := ShallowError("../repo")
	if !strings.HasPrefix(err.Error(), "this repository is a shallow clone") {
		t.Errorf("unexpected message: %q", err.Error())
	}
	for _, want := range []string{
		"git fetch --unshallow",
		"fetch-depth: 0",
		"GIT_DEPTH: 0",
		"clone: depth: full",
		"--allow-shallow",
		// The supplied path, repeated exactly (ADR-0067 clause 3).
		"../repo",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message is missing %q", want)
		}
	}
	if got := core.ReasonOf(err); got != core.ReasonShallowClone {
		t.Errorf("reason = %q, want %q", got, core.ReasonShallowClone)
	}
	if core.ExitCode(err) != core.ExitUser {
		t.Errorf("exit code = %d, want %d", core.ExitCode(err), core.ExitUser)
	}
	// The artifact rendering keeps the remedy and drops the path.
	if artifact := core.Artifact(err); strings.Contains(artifact, "../repo") {
		t.Errorf("artifact rendering %q names the supplied path", artifact)
	}
}

func TestPreflightEmptyRepository(t *testing.T) {
	empty := fixture(t, "empty")
	_, err := newCollector().Preflight(context.Background(), empty, "fixtures/empty")
	if err == nil {
		t.Fatal("expected an error for a repository with no commits")
	}
	if got := core.ReasonOf(err); got != core.ReasonEmptyRepository {
		t.Errorf("reason = %q, want %q", got, core.ReasonEmptyRepository)
	}
	for _, want := range []string{"has no commits", "fixtures/empty", emptyRepositoryRemedy} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q is missing %q", err.Error(), want)
		}
	}
}

// The refusal repeats the path the caller supplied and never the resolved one,
// so an absolute path reaches the message only when the operator typed one
// (ADR-0067 clauses 3 and 5).
func TestPreflightNotARepository(t *testing.T) {
	dir := t.TempDir()
	_, err := newCollector().Preflight(context.Background(), dir, "./somewhere-else")
	if err == nil {
		t.Fatal("expected an error for a non-repository path")
	}
	if got := core.ReasonOf(err); got != core.ReasonNotARepository {
		t.Errorf("reason = %q, want %q", got, core.ReasonNotARepository)
	}
	if !strings.Contains(err.Error(), "./somewhere-else") {
		t.Errorf("message %q does not name the supplied path", err.Error())
	}
	if strings.Contains(err.Error(), dir) {
		t.Errorf("message %q names the resolved path", err.Error())
	}
	if !strings.Contains(err.Error(), notRepositoryRemedy) {
		t.Errorf("message %q does not name the remedy", err.Error())
	}
}

func TestParseGitVersion(t *testing.T) {
	cases := []struct {
		in    string
		want  [3]int
		valid bool
	}{
		{"git version 2.55.0.windows.3", [3]int{2, 55, 0}, true},
		{"git version 2.22.0", [3]int{2, 22, 0}, true},
		{"git version 2.39.5 (Apple Git-154)", [3]int{2, 39, 5}, true},
		{"git version 2.21", [3]int{2, 21, 0}, true},
		{"not a version", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := parseGitVersion(c.in)
		if ok != c.valid {
			t.Errorf("parseGitVersion(%q) ok = %v, want %v", c.in, ok, c.valid)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseGitVersion(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	min := mustParseVersion(MinGitVersion)
	if compareVersions([3]int{2, 21, 9}, min) >= 0 {
		t.Error("2.21.9 should sort below the minimum")
	}
	if compareVersions([3]int{2, 22, 0}, min) != 0 {
		t.Error("2.22.0 should equal the minimum")
	}
	if compareVersions([3]int{3, 0, 0}, min) <= 0 {
		t.Error("3.0.0 should sort above the minimum")
	}
}
