package collect

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fixture returns the path to a built fixture repository, skipping the test
// when fixtures have not been generated. No test in this package touches the
// network.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture %q not built; run `make fixtures`", name)
	}
	return path
}

func git(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func TestCollectCommitCountMatchesRevList(t *testing.T) {
	repo := fixture(t, "basic")
	h, err := Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want, err := strconv.Atoi(git(t, repo, "rev-list", "--all", "--count"))
	if err != nil {
		t.Fatalf("parsing rev-list output: %v", err)
	}
	if len(h.Commits) != want {
		t.Errorf("collected %d commits, git rev-list reports %d", len(h.Commits), want)
	}
}

func TestCollectTotalAddedLinesMatchesGit(t *testing.T) {
	repo := fixture(t, "basic")
	h, err := Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var got int
	for _, c := range h.Commits {
		for _, f := range c.Files {
			got += f.Added
		}
	}

	// Independent recomputation straight from git, not reusing the parser.
	raw := git(t, repo, "log", "--all", "--numstat", "--no-renames", "--pretty=format:")
	var want int
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 3)
		if len(parts) != 3 || parts[0] == "-" {
			continue
		}
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		want += n
	}

	if got != want {
		t.Errorf("total added lines = %d, git reports %d", got, want)
	}
}

func TestCollectPreservesAuthorTimezoneOffsets(t *testing.T) {
	repo := fixture(t, "basic")
	h, err := Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// Raw %aI strings, keyed by hash, straight from git.
	raw := map[string]string{}
	for _, line := range strings.Split(git(t, repo, "log", "--all", "--pretty=format:%H %aI"), "\n") {
		hash, iso, ok := strings.Cut(strings.TrimSpace(line), " ")
		if ok {
			raw[hash] = iso
		}
	}

	seenOffsets := map[int]bool{}
	for _, c := range h.Commits {
		iso, ok := raw[c.Hash]
		if !ok {
			t.Fatalf("commit %s missing from git output", c.Hash)
		}
		want, err := offsetMinutesFromISO(iso)
		if err != nil {
			t.Fatalf("commit %s: %v", c.Hash, err)
		}
		if c.AuthorTZOffsetMinutes != want {
			t.Errorf("commit %s offset = %d minutes, want %d (from %q)",
				c.Hash, c.AuthorTZOffsetMinutes, want, iso)
		}
		seenOffsets[c.AuthorTZOffsetMinutes] = true
	}

	if len(seenOffsets) < 3 {
		t.Errorf("fixture should span at least 3 timezone offsets, saw %d", len(seenOffsets))
	}
}

// offsetMinutesFromISO reads the trailing offset of a strict ISO 8601 stamp
// without going through the production parser.
func offsetMinutesFromISO(iso string) (int, error) {
	if strings.HasSuffix(iso, "Z") {
		return 0, nil
	}
	if len(iso) < 6 {
		return 0, strconv.ErrSyntax
	}
	tail := iso[len(iso)-6:] // ±HH:MM
	sign := 1
	switch tail[0] {
	case '+':
	case '-':
		sign = -1
	default:
		return 0, strconv.ErrSyntax
	}
	hh, err := strconv.Atoi(tail[1:3])
	if err != nil {
		return 0, err
	}
	mm, err := strconv.Atoi(tail[4:6])
	if err != nil {
		return 0, err
	}
	return sign * (hh*60 + mm), nil
}

func TestCollectRootCommitHasNoParents(t *testing.T) {
	repo := fixture(t, "basic")
	h, err := Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	roots := map[string]bool{}
	for _, line := range strings.Split(git(t, repo, "rev-list", "--all", "--max-parents=0"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			roots[line] = true
		}
	}
	if len(roots) == 0 {
		t.Fatal("fixture has no root commit")
	}

	found := 0
	for _, c := range h.Commits {
		if roots[c.Hash] {
			found++
			if len(c.Parents) != 0 {
				t.Errorf("root commit %s has %d parents, want 0", c.Hash, len(c.Parents))
			}
			if c.IsMerge {
				t.Errorf("root commit %s flagged as a merge", c.Hash)
			}
		}
	}
	if found != len(roots) {
		t.Errorf("found %d of %d root commits", found, len(roots))
	}
}

func TestCollectMergeCommitsCarryNoFiles(t *testing.T) {
	repo := fixture(t, "merges")
	h, err := Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	merges := 0
	for _, c := range h.Commits {
		if len(c.Parents) <= 1 {
			continue
		}
		merges++
		if !c.IsMerge {
			t.Errorf("commit %s has %d parents but IsMerge is false", c.Hash, len(c.Parents))
		}
		if len(c.Files) != 0 {
			t.Errorf("merge commit %s carries %d file changes, want 0", c.Hash, len(c.Files))
		}
	}
	if merges != 5 {
		t.Errorf("merges fixture has %d merge commits, want 5", merges)
	}
}

func TestCollectSingleCommitRepository(t *testing.T) {
	h, err := Collect(Options{RepoPath: fixture(t, "single")})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(h.Commits) != 1 {
		t.Fatalf("collected %d commits, want 1", len(h.Commits))
	}
	if len(h.Commits[0].Parents) != 0 {
		t.Errorf("the only commit should have no parents")
	}
}

func TestCollectUnusualPaths(t *testing.T) {
	h, err := Collect(Options{RepoPath: fixture(t, "binary")})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	paths := map[string]bool{}
	binaryFiles := 0
	for _, c := range h.Commits {
		for _, f := range c.Files {
			paths[f.Path] = true
			if f.IsBinary {
				binaryFiles++
				if f.Added != 0 || f.Deleted != 0 {
					t.Errorf("binary file %s has non-zero line counts (%d/%d)", f.Path, f.Added, f.Deleted)
				}
			}
		}
	}

	for _, want := range []string{
		"assets/a file with spaces.txt",
		"assets/ünïcödé-ファイル.txt",
	} {
		if !paths[want] {
			t.Errorf("path %q not found; collected paths: %v", want, keys(paths))
		}
	}
	if binaryFiles == 0 {
		t.Error("no binary file change recorded")
	}

	// A quote in a filename is not representable on NTFS, so the fixture
	// creates it only where the filesystem allows.
	if _, err := os.Stat(filepath.Join(fixture(t, "binary"), `quoted".txt`)); err == nil {
		if !paths[`quoted".txt`] {
			t.Errorf(`path 'quoted".txt' not found; collected paths: %v`, keys(paths))
		}
	}
}

func TestCollectMailmapReconcilesWithShortlog(t *testing.T) {
	repo := fixture(t, "mailmap")
	h, err := Collect(Options{RepoPath: repo, UseMailmap: true})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	got := map[string]int{}
	for _, c := range h.Commits {
		got[c.AuthorName]++
	}

	want := map[string]int{}
	for _, line := range strings.Split(git(t, repo, "shortlog", "-sn", "--all"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		count, name, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("unexpected shortlog line %q", line)
		}
		n, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil {
			t.Fatalf("parsing shortlog count %q: %v", count, err)
		}
		want[name] = n
	}

	if len(got) != len(want) {
		t.Errorf("collected %d distinct author names, shortlog reports %d: %v vs %v",
			len(got), len(want), got, want)
	}
	for name, n := range want {
		if got[name] != n {
			t.Errorf("author %q: collected %d commits, shortlog reports %d", name, got[name], n)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
