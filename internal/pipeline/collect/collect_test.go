package collect

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
)

// newCollector is the collect stage with a fixed clock, so nothing a test
// produces depends on when it ran.
func newCollector() *Collector {
	return New(core.FixedClock(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)), core.SystemFilesystem())
}

// fixture returns the path to a built fixture repository, failing the test
// when fixtures have not been generated. No test in this package touches the
// network.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ADR-0064: fixture %q is missing; the gates generate it with `make fixtures`", name)
	}
	return path
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := git.Output(context.Background(), git.At(repo, args...))
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

// gitRecords reads git's NUL-delimited output. The recomputations below are
// independent of the parser under test but not of the record format: a
// reading of git that split on newlines would be wrong about exactly the
// names this package exists to handle (ADR-0065 clause 2).
func gitRecords(t *testing.T, repo string, args ...string) []string {
	t.Helper()
	records, err := git.Records(context.Background(), git.At(repo, args...).Pathspecs())
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return records
}

func TestCollectCommitCountMatchesRevList(t *testing.T) {
	t.Parallel()
	repo := fixture(t, "basic")
	h, err := newCollector().Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	want, err := strconv.Atoi(gitOutput(t, repo, "rev-list", "--all", "--count"))
	if err != nil {
		t.Fatalf("parsing rev-list output: %v", err)
	}
	if len(h.Commits) != want {
		t.Errorf("collected %d commits, git rev-list reports %d", len(h.Commits), want)
	}
}

func TestCollectTotalAddedLinesMatchesGit(t *testing.T) {
	t.Parallel()
	repo := fixture(t, "basic")
	h, err := newCollector().Collect(Options{RepoPath: repo})
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
	// The header is an object name and the field separator, so a record that
	// ends in one is a header with no file entries, and the entry a header
	// shares a record with follows its newline.
	var want int
	for _, record := range gitRecords(t, repo, "log", "-z", "--all", "--numstat", "--no-renames",
		"--pretty=format:%H\x1f") {
		entry := record
		if head, rest, ok := strings.Cut(record, "\n"); ok && strings.HasSuffix(head, "\x1f") {
			entry = rest
		} else if strings.HasSuffix(record, "\x1f") {
			continue
		}
		parts := strings.SplitN(entry, "\t", 3)
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
	t.Parallel()
	repo := fixture(t, "basic")
	h, err := newCollector().Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// Raw %aI strings, keyed by hash, straight from git.
	raw := map[string]string{}
	for _, record := range gitRecords(t, repo, "log", "-z", "--all", "--pretty=format:%H %aI") {
		if hash, iso, ok := strings.Cut(record, " "); ok {
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
	t.Parallel()
	repo := fixture(t, "basic")
	h, err := newCollector().Collect(Options{RepoPath: repo})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	roots := map[string]bool{}
	for _, record := range gitRecords(t, repo, "log", "-z", "--all", "--max-parents=0",
		"--pretty=format:%H") {
		roots[record] = true
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
	t.Parallel()
	repo := fixture(t, "merges")
	h, err := newCollector().Collect(Options{RepoPath: repo})
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
	t.Parallel()
	h, err := newCollector().Collect(Options{RepoPath: fixture(t, "single")})
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
	t.Parallel()
	h, err := newCollector().Collect(Options{RepoPath: fixture(t, "binary")})
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
	t.Parallel()
	repo := fixture(t, "mailmap")
	h, err := newCollector().Collect(Options{RepoPath: repo, UseMailmap: true})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	got := map[string]int{}
	for _, c := range h.Commits {
		got[c.AuthorName]++
	}

	// The counts `shortlog -sn` would group by author name. shortlog has no
	// NUL-delimited form, and an author name is repository content, so the
	// names are counted from records instead.
	want := map[string]int{}
	for _, name := range gitRecords(t, repo, "log", "-z", "--all", "--pretty=format:%aN") {
		want[name]++
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
