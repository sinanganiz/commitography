package collect

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreflightBasicRepository(t *testing.T) {
	repo := fixture(t, "basic")
	info, err := Preflight(repo)
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
	info, err := Preflight(fixture(t, "shallow"))
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
	err := error(&ShallowError{Path: "/tmp/repo"})
	if !strings.HasPrefix(err.Error(), "this repository is a shallow clone") {
		t.Errorf("unexpected message: %q", err.Error())
	}
	for _, want := range []string{
		"git fetch --unshallow",
		"fetch-depth: 0",
		"GIT_DEPTH: 0",
		"clone: depth: full",
		"--allow-shallow",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message is missing %q", want)
		}
	}
	var se *ShallowError
	if !errors.As(err, &se) {
		t.Error("ShallowError is not recoverable with errors.As")
	}
}

func TestPreflightEmptyRepository(t *testing.T) {
	_, err := Preflight(fixture(t, "empty"))
	if err == nil {
		t.Fatal("expected an error for a repository with no commits")
	}
	if err.Error() != "repository has no commits" {
		t.Errorf("error = %q, want %q", err.Error(), "repository has no commits")
	}
}

func TestPreflightNotARepository(t *testing.T) {
	dir := t.TempDir()
	_, err := Preflight(dir)
	if err == nil {
		t.Fatal("expected an error for a non-repository path")
	}
	want := dir + " is not a git repository"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
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
