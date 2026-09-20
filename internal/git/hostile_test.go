package git

import (
	"context"
	"strings"
	"testing"
)

// hostileNames are path names a repository may hold and a reader may not
// assume away: a newline, a quote, a backslash, control characters, the field
// separator the history reader uses, and a name that is itself forty
// hexadecimal characters long. None of them can be created as files on every
// supported platform, so they are put in the index directly, which needs no
// filesystem and gives the same objects everywhere (ADR-0019 clause 1,
// ADR-0045).
func hostileNames() []string {
	return []string{
		"src/two\nlines.go",
		"src/\"quoted\".go",
		"src/back\\slash.go",
		"src/bell\a-and-\x1b-escape.go",
		"src/separator\x1fin-the-name.go",
		"src/tab\tseparated.go",
		"src/" + strings.Repeat("a", 40) + ".go",
		"src/ünïcödé-ファイル.txt",
		"src/trailing space .go",
	}
}

// hostileSubject carries every byte a commit subject may hold that would
// disturb a reader: the field separator, the control character the previous
// record format used as its delimiter, quotes and a backslash. A subject
// cannot contain a newline, because it is the first line of the message.
const hostileSubject = "feat: separator\x1fhere, \"quotes\", a \\ and \x01 as well"

// TestHostileNamesSurviveTheRead enforces ADR-0065 clause 2's output rule and
// ADR-0045 on a real invocation: with NUL-delimited output, a name arrives
// exactly as the repository holds it, and no record is split, merged or
// dropped.
//
// It is the property the whole format change exists for. A reader that split
// on newlines would invent two files out of the first name; one that reversed
// git's C-style quoting would strip the quotes from the second; one that
// counted header fields would lose the subject at its separator.
func TestHostileNamesSurviveTheRead(t *testing.T) {
	t.Parallel()
	repo := hostileRepository(t)
	ctx := context.Background()

	// The listing every tracked-file read goes through.
	tracked, err := Records(ctx, At(repo, "ls-files", "-z").Pathspecs())
	if err != nil {
		t.Fatalf("listing tracked files: %v", err)
	}
	assertSameNames(t, "the tracked file listing", tracked)

	// The listing the aggregate stage reads the tree with.
	tree, err := Records(ctx, At(repo, "ls-tree", "-r", "--name-only", "-z", "HEAD").Pathspecs())
	if err != nil {
		t.Fatalf("listing the tree: %v", err)
	}
	assertSameNames(t, "the tree listing", tree)

	// The history read itself: one header record carrying the subject, then
	// one record per file entry.
	var subject string
	var paths []string
	err = Scan(ctx, At(repo, "log", "-z", "--numstat", "--no-renames",
		"--pretty=format:%H\x1f%s").Pathspecs(), func(record string) error {
		switch {
		case record == "":
			// The separator git writes between commits.
		case isHeaderRecord(record):
			// The header and the first file entry share a record, separated
			// by the newline the subject cannot contain.
			head, first, hasFiles := strings.Cut(record, "\n")
			_, subject, _ = strings.Cut(head, "\x1f")
			if hasFiles {
				paths = append(paths, pathOfEntry(first))
			}
		default:
			paths = append(paths, pathOfEntry(record))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading the history: %v", err)
	}
	if subject != hostileSubject {
		t.Errorf("the subject came back %q, want %q", subject, hostileSubject)
	}
	assertSameNames(t, "the history read", paths)
}

// isHeaderRecord reports whether a record begins with an object name
// followed by the field separator, which a file entry cannot: its first
// field is a count or a dash, and a tab is not a hexadecimal digit. One of
// the hostile names is forty hexadecimal characters long precisely so that
// this distinction is exercised.
func isHeaderRecord(record string) bool {
	i := strings.Index(record, "\x1f")
	if i != 40 && i != 64 {
		return false
	}
	return strings.Trim(record[:i], "0123456789abcdef") == ""
}

// pathOfEntry is the path of one `<added>\t<deleted>\t<path>` entry: whatever
// follows the second tab, however many tabs and newlines it contains.
func pathOfEntry(entry string) string {
	parts := strings.SplitN(entry, "\t", 3)
	if len(parts) != 3 {
		return entry
	}
	return parts[2]
}

// assertSameNames compares a listing with the names the repository holds.
func assertSameNames(t *testing.T, what string, got []string) {
	t.Helper()
	want := map[string]bool{}
	for _, name := range hostileNames() {
		want[name] = true
	}
	seen := map[string]bool{}
	for _, name := range got {
		if !want[name] {
			t.Errorf("%s produced %q, which the repository does not hold; a record was split or merged",
				what, name)
			continue
		}
		seen[name] = true
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("%s dropped %q", what, name)
		}
	}
}

// hostileRepository builds a repository holding hostileNames, through the
// index rather than the working tree so that every supported platform can
// hold it, and commits it with fixed identities and dates so that nothing
// here reads a clock (ADR-0042 clause 4).
func hostileRepository(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()

	run := func(spec Spec) string {
		t.Helper()
		out, err := Output(ctx, spec)
		if err != nil {
			t.Fatalf("preparing the hostile repository: %v", err)
		}
		return out
	}

	run(At("", "init", "-q", dir))
	object := run(At(dir, "hash-object", "-w", "--stdin").WithStdin(strings.NewReader("content\n")))

	var index strings.Builder
	for _, name := range hostileNames() {
		index.WriteString("100644 " + object + "\t" + name + "\x00")
	}
	// Git refuses to put a name Windows cannot hold into an index on
	// Windows, so building the repository needs the guard off. Reading one
	// does not, which is the point: a repository cloned elsewhere reaches
	// this reader on every platform, and the reader must return what it
	// holds (ADR-0045).
	run(At(dir, "update-index", "-z", "--add", "--index-info").
		Configured("core.protectNTFS=false").
		WithStdin(strings.NewReader(index.String())))

	tree := run(At(dir, "write-tree"))
	// commit-tree rather than commit: it needs no working tree, and the two
	// identity variables and the two dates make the commit reproducible.
	commit := run(At(dir, "commit-tree", tree, "-m", hostileSubject).WithEnv(
		"GIT_AUTHOR_NAME=Fixture Builder",
		"GIT_AUTHOR_EMAIL=fixtures@example.com",
		"GIT_AUTHOR_DATE=1700000000 +0000",
		"GIT_COMMITTER_NAME=Fixture Builder",
		"GIT_COMMITTER_EMAIL=fixtures@example.com",
		"GIT_COMMITTER_DATE=1700000000 +0000",
	))
	run(At(dir, "update-ref", "HEAD", commit))
	return dir
}
