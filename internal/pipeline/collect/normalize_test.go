package collect

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/git"
)

// newRepository creates a repository with one commit holding files, and
// returns its directory.
func newRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if _, err := git.Output(context.Background(), git.At(dir, args...).
			Configured("user.name=Ada Lovelace", "user.email=ada@example.com", "commit.gpgSign=false")); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run("init", "-q")
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "feat: initial")
	return dir
}

// WP-0012 clause 10c: attributes are repository content as of the analysed
// commit. A working-tree copy that differs, uncommitted, changes nothing.
func TestAttributesComeFromTheAnalysedCommit(t *testing.T) {
	t.Parallel()
	committed := "gen/*.go linguist-generated=true\n"
	repo := newRepository(t, map[string]string{
		".gitattributes": committed,
		"gen/api.go":     "package gen\n",
		"main.go":        "package main\n",
	})
	// An uncommitted edit that would re-include the generated file.
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("gen/*.go -linguist-generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := newCollector().Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if h.Attributes != "gen/*.go linguist-generated=true" {
		t.Errorf("the artifact carries the attributes %q, want the analysed commit's", h.Attributes)
	}
	excluded := map[string]bool{}
	for _, f := range h.Commits[0].Files {
		excluded[f.Path] = f.Excluded
	}
	if !excluded["gen/api.go"] {
		t.Error("gen/api.go is not excluded, so the attributes came from the working tree")
	}
	if excluded["main.go"] {
		t.Error("main.go is excluded, and nothing marks it generated")
	}
}

// A commit with no .gitattributes at its root has no attributes, and a path
// named .gitattributes that is not a regular file is not read as one.
func TestAttributesAbsentOrNotAFile(t *testing.T) {
	t.Parallel()
	for name, files := range map[string]map[string]string{
		"no attributes":              {"main.go": "package main\n"},
		"a directory of that name":   {".gitattributes/inside": "x\n", "main.go": "package main\n"},
		"a nested attributes file":   {"sub/.gitattributes": "*.go linguist-generated\n", "sub/a.go": "package a\n"},
		"an attributes file present": {".gitattributes": "*.md linguist-generated\n", "a.md": "x\n"},
	} {
		repo := newRepository(t, files)
		h, err := newCollector().Collect(Options{RepoPath: repo})
		if err != nil {
			t.Fatalf("%s: Collect: %v", name, err)
		}
		want := ""
		if content, ok := files[".gitattributes"]; ok {
			want = content[:len(content)-1]
		}
		if h.Attributes != want {
			t.Errorf("%s: the artifact carries the attributes %q, want %q", name, h.Attributes, want)
		}
	}
}

// The records carry every per-commit definition of docs/metrics.md section 1,
// as the section defines it.
func TestRecordsCarryTheSectionOneDefinitions(t *testing.T) {
	t.Parallel()
	repo := fixture(t, "noise")
	cfg := config.Default()
	h, err := newCollector().Collect(Options{RepoPath: repo, Analysis: &cfg})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(h.Commits) == 0 {
		t.Fatal("ADR-0064: the noise fixture has no commits")
	}
	excludedFiles, bulk := 0, 0
	for _, c := range h.Commits {
		lines := 0
		for _, f := range c.Files {
			if f.Excluded {
				excludedFiles++
				continue
			}
			lines += f.Added + f.Deleted
		}
		if c.EffectiveLines != lines {
			t.Errorf("commit %s has %d effective lines, and its non-excluded files add up to %d",
				short(c.Hash), c.EffectiveLines, lines)
		}
		if want := !c.Excluded && lines > cfg.OutlierThresholdLines; c.IsBulk != want {
			t.Errorf("commit %s is bulk %v with %d effective lines", short(c.Hash), c.IsBulk, lines)
		}
		if c.IsBulk {
			bulk++
		}
		if !c.LocalTime.Equal(c.AuthorDate) || c.LocalTime.Format("-07:00") != c.AuthorDate.Format("-07:00") {
			t.Errorf("commit %s is attributed to %v, want its author local time %v",
				short(c.Hash), c.LocalTime, c.AuthorDate)
		}
		if c.ActiveDate != c.AuthorDate.Format("2006-01-02") {
			t.Errorf("commit %s carries the active date %s, want its author local date", short(c.Hash), c.ActiveDate)
		}
		if c.IdentityID == "" {
			t.Errorf("commit %s carries no identity", short(c.Hash))
		}
	}
	// Without these the checks above would hold trivially.
	if excludedFiles == 0 {
		t.Error("ADR-0064: no file of the noise fixture is excluded, so exclusion was not exercised")
	}
	if bulk == 0 {
		t.Error("ADR-0064: the noise fixture has no bulk commit, so the bulk flag was not exercised")
	}
}

// Analysed-commit membership records why a commit is not analysed.
func TestRecordsCarryMembershipAndItsReasons(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	merges, err := newCollector().Collect(Options{RepoPath: fixture(t, "merges"), Analysis: &cfg})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	mergesExcluded := 0
	for _, c := range merges.Commits {
		if c.IsMerge != c.MergeExcluded || c.Excluded != (c.MergeExcluded || c.AuthorExcluded) {
			t.Errorf("commit %s: merge %v, merge-excluded %v, author-excluded %v, excluded %v",
				short(c.Hash), c.IsMerge, c.MergeExcluded, c.AuthorExcluded, c.Excluded)
		}
		if c.MergeExcluded {
			mergesExcluded++
		}
	}
	if mergesExcluded != 5 {
		t.Errorf("%d merges were left out, want the fixture's 5", mergesExcluded)
	}

	counted := config.Default()
	counted.CountMerges = true
	withMerges, err := newCollector().Collect(Options{RepoPath: fixture(t, "merges"), Analysis: &counted})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	for _, c := range withMerges.Commits {
		if c.MergeExcluded {
			t.Errorf("commit %s is left out as a merge, and merges are counted", short(c.Hash))
		}
	}

	bots, err := newCollector().Collect(Options{RepoPath: fixture(t, "bots"), Analysis: &cfg})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	authorExcluded := 0
	for _, c := range bots.Commits {
		if c.AuthorExcluded {
			authorExcluded++
			if !c.Excluded {
				t.Errorf("commit %s is author-excluded but analysed", short(c.Hash))
			}
		}
	}
	if authorExcluded == 0 {
		t.Error("no commit of the bots fixture is author-excluded")
	}
}

// The analysis plane carries the read options, so a caller that gives both has
// made a mistake the stage refuses rather than resolves.
func TestAnAnalysisPlaneAndSeparateReadOptionsAreRefused(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	if _, err := newCollector().Collect(Options{RepoPath: fixture(t, "single"), Analysis: &cfg, UseMailmap: true}); err == nil {
		t.Error("the stage accepted an analysis plane together with a separate read option")
	}
}
