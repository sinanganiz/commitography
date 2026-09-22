package collect

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// The record stream this package reads is what
// `git log -z --raw --numstat -m` produces. header builds one record header in
// that format.
func header(hash, subject string) string {
	return mergeHeader(hash, subject, "55b21389ef6f6f535dd04b710605ba3b605f7c3a")
}

// mergeHeader builds a record header with the given parents.
func mergeHeader(hash, subject string, parents ...string) string {
	return hash +
		"\x1fRaymond Hettinger\x1fpython@rcn.com\x1fpython@rcn.com" +
		"\x1f2010-04-10T16:57:36Z\x1f2010-04-10T16:57:36Z" +
		"\x1f" + strings.Join(parents, " ") +
		"\x1f" + treeOf(hash) +
		"\x1f" + subject
}

// treeOf is the tree the test streams give a commit.
func treeOf(hash string) string { return "7" + hash[1:] }

// The object names the test streams give a file's content before and after.
const (
	oldObject = "1111111111111111111111111111111111111111"
	newObject = "2222222222222222222222222222222222222222"
	noObject  = "0000000000000000000000000000000000000000"
)

// raw builds a raw entry and its path records: one path, or the path before
// and the path after for a rename.
func raw(status string, paths ...string) string {
	return rawObjects(status, oldObject, newObject, paths...)
}

// rawObjects is raw with the object names given.
func rawObjects(status, before, after string, paths ...string) string {
	return ":100644 100644 " + before + " " + after + " " + status + "\x00" + strings.Join(paths, "\x00")
}

// parseStream feeds a recorded stream through the parser the way the git
// package hands records to it: split on NUL, nothing else.
func parseStream(opts Options, expected int, stream string) *logParser {
	parser := newLogParser(opts, expected)
	for _, record := range strings.Split(stream, "\x00") {
		_ = parser.record(record)
	}
	parser.finish()
	return parser
}

const testHash = "4e45512de2d76e0366c5e7ac5d02119419bfc9ea"

// A commit subject may contain any byte but NUL and a newline, the field
// separator included. The header is split into a fixed number of fields so
// that such a subject keeps its separators instead of producing a record with
// too many fields (ADR-0045).
func TestParseKeepsAFieldSeparatorInsideASubject(t *testing.T) {
	t.Parallel()
	subject := "Issue 8361: Remove\x1fassert\x01 from \"functools\"\\"
	stream := header(testHash, subject) + "\n" + raw("M", "Lib/functools.py") + "\x003\t2\tLib/functools.py\x00\x00"

	var warnings []string
	parsed := parseStream(Options{OnWarning: func(m string) { warnings = append(warnings, m) }}, 0, stream)

	if parsed.failed != 0 || len(warnings) != 0 {
		t.Errorf("failed = %d, warnings = %q, want none of either", parsed.failed, warnings)
	}
	if parsed.total != 1 {
		t.Errorf("total = %d, want 1", parsed.total)
	}
	if len(parsed.commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(parsed.commits))
	}
	c := parsed.commits[0]
	if c.Hash != testHash {
		t.Errorf("hash = %q, want %q", c.Hash, testHash)
	}
	if c.Subject != subject {
		t.Errorf("subject = %q, want %q", c.Subject, subject)
	}
	if len(c.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(c.Files))
	}
	if f := c.Files[0]; f.Path != "Lib/functools.py" || f.Added != 3 || f.Deleted != 2 ||
		f.OldBlob != oldObject || f.NewBlob != newObject {
		t.Errorf("file = %+v, want Lib/functools.py +3 -2 from %s to %s", f, oldObject, newObject)
	}
	if c.Tree != treeOf(testHash) {
		t.Errorf("tree = %q, want %q", c.Tree, treeOf(testHash))
	}
}

// A path is whatever remains of its record, so a name holding a newline, a
// quote or a control character arrives whole. These are the names a
// line-based reader splits into files that do not exist, and the ones git
// would C-quote were the output not NUL-delimited (ADR-0045).
func TestParseKeepsHostilePathsWhole(t *testing.T) {
	t.Parallel()
	hostile := []string{
		"src/two\nlines.go",
		"src/\"quoted\".go",
		"src/back\\slash.go",
		"src/bell\a-and-\x1f-separator.go",
		"src/ünïcödé-ファイル.txt",
		"src/trailing space .go",
	}
	stream := header(testHash, "chore: awkward names") + "\n"
	for _, path := range hostile {
		stream += raw("A", path) + "\x00"
	}
	for _, path := range hostile {
		stream += "1\t0\t" + path + "\x00"
	}
	stream += "\x00"

	parsed := parseStream(Options{}, 0, stream)
	if len(parsed.commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(parsed.commits))
	}
	files := parsed.commits[0].Files
	if len(files) != len(hostile) {
		t.Fatalf("got %d files, want %d; a record was split, merged or dropped", len(files), len(hostile))
	}
	for i, path := range hostile {
		if files[i].Path != path || files[i].NewBlob != newObject {
			t.Errorf("file %d = %q with content %s, want %q with %s", i, files[i].Path, files[i].NewBlob, path,
				newObject)
		}
	}
}

// A rename is an entry with an empty path field and its two paths in records
// of their own, whether it is the first entry, sharing the header's record,
// or a later one (WP-0012 clause 10a).
func TestParseReadsRenameEntries(t *testing.T) {
	t.Parallel()
	stream := header(testHash, "refactor: move things") + "\n" +
		raw("R100", "src/old.go", "src/new.go") + "\x00" +
		raw("M", "plain.go") + "\x00" +
		raw("R087", "docs/a.md", "docs/b.md") + "\x00" +
		raw("R100", "logo.png", "assets/logo.png") + "\x00" +
		"0\t0\t\x00src/old.go\x00src/new.go\x00" +
		"1\t0\tplain.go\x00" +
		"2\t1\t\x00docs/a.md\x00docs/b.md\x00" +
		"-\t-\t\x00logo.png\x00assets/logo.png\x00\x00"

	parsed := parseStream(Options{}, 0, stream)
	if len(parsed.commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(parsed.commits))
	}
	want := []struct {
		previous, path string
		added, deleted int
		binary         bool
	}{
		{"src/old.go", "src/new.go", 0, 0, false},
		{"", "plain.go", 1, 0, false},
		{"docs/a.md", "docs/b.md", 2, 1, false},
		{"logo.png", "assets/logo.png", 0, 0, true},
	}
	files := parsed.commits[0].Files
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %+v", len(files), len(want), files)
	}
	for i, w := range want {
		f := files[i]
		if f.PreviousPath != w.previous || f.Path != w.path || f.Added != w.added || f.Deleted != w.deleted ||
			f.IsBinary != w.binary {
			t.Errorf("file %d = %+v, want %+v", i, f, w)
		}
	}
}

// A rename's paths are taken because of where they sit, never because of
// what they look like: a source path crafted to resemble a commit header is
// still a path, and no commit starts in the middle of it (WP-0012 clause
// 10a, ADR-0045).
func TestParseTakesRenamePathsByPosition(t *testing.T) {
	t.Parallel()
	next := "0123456789abcdef0123456789abcdef01234567"
	crafted := strings.Repeat("f", 40) + "\x1fAda\x1fada@example.com\x1fada@example.com" +
		"\x1f2010-04-10T16:57:36Z\x1f2010-04-10T16:57:36Z\x1f\x1f" + strings.Repeat("f", 40) + "\x1fnot a commit"
	// A path shaped like a raw entry is taken by position too.
	rawShaped := ":100644 100644 " + oldObject + " " + newObject + " M"
	following := header(next, "feat: next") + "\n" + raw("M", "main.go") + "\x001\t0\tmain.go\x00\x00"
	for name, stream := range map[string]string{
		"as the first entry": header(testHash, "refactor: move") + "\n" +
			raw("R100", crafted, "src/new.go") + "\x00" +
			"0\t0\t\x00" + crafted + "\x00src/new.go\x00\x00" + following,
		"as a later entry": header(testHash, "refactor: move") + "\n" +
			raw("M", "keep.go") + "\x00" + raw("R100", crafted, "src/new.go") + "\x00" +
			"1\t0\tkeep.go\x00" + "0\t0\t\x00" + crafted + "\x00src/new.go\x00\x00" + following,
		"as the destination too": header(testHash, "refactor: move") + "\n" +
			raw("R100", crafted, crafted) + "\x00" +
			"0\t0\t\x00" + crafted + "\x00" + crafted + "\x00\x00" + following,
		"beside a path shaped like a raw entry": header(testHash, "refactor: move") + "\n" +
			raw("R100", crafted, rawShaped) + "\x00" +
			"0\t0\t\x00" + crafted + "\x00" + rawShaped + "\x00\x00" + following,
	} {
		parsed := parseStream(Options{}, 0, stream)
		if parsed.failed != 0 || len(parsed.commits) != 2 {
			t.Fatalf("%s: %d commits and %d failures, want 2 and none; the path started a commit",
				name, len(parsed.commits), parsed.failed)
		}
		found := false
		for _, f := range parsed.commits[0].Files {
			if f.PreviousPath == crafted {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the crafted rename source is not the previous path of any entry: %+v",
				name, parsed.commits[0].Files)
		}
		if parsed.commits[1].Hash != next || len(parsed.commits[1].Files) != 1 {
			t.Errorf("%s: the commit after the rename came back as %+v", name, parsed.commits[1])
		}
	}
}

// A stream that ends inside a rename leaves an entry with one of its two
// paths, which stands for nothing. The commit can no longer be read whole, and
// replay derives ownership from every entry of it, so it is counted as
// unreadable and skipped rather than kept with an entry missing.
func TestParseDropsARenameTheStreamEndsInside(t *testing.T) {
	t.Parallel()
	stream := header(testHash, "refactor: move") + "\n" + raw("M", "keep.go") + "\x00" +
		raw("R100", "src/old.go", "src/new.go") + "\x00" + "1\t0\tkeep.go\x000\t0\t\x00src/old.go"
	parsed := parseStream(Options{}, 0, stream)
	if len(parsed.commits) != 0 || parsed.failed != 1 || parsed.total != 1 {
		t.Errorf("got %d commits and %d of %d failed, want the one commit counted as failed",
			len(parsed.commits), parsed.failed, parsed.total)
	}
}

// git writes no file entries for an empty commit, so such a record is a header
// and the separator alone. It may not swallow the commit that follows it.
func TestParseReadsCommitsWithoutFileEntries(t *testing.T) {
	t.Parallel()
	second := "0123456789abcdef0123456789abcdef01234567"
	stream := header(testHash, "chore: nothing changes") + "\x00" +
		header(second, "feat: with a file") + "\n" + raw("A", "main.go") + "\x001\t0\tmain.go\x00\x00"

	parsed := parseStream(Options{}, 0, stream)
	if len(parsed.commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(parsed.commits))
	}
	if len(parsed.commits[0].Files) != 0 {
		t.Errorf("the empty commit gained %d files", len(parsed.commits[0].Files))
	}
	if len(parsed.commits[1].Files) != 1 {
		t.Errorf("the following commit lost its file entry")
	}
	if parsed.commits[1].Hash != second {
		t.Errorf("second commit = %q, want %q", parsed.commits[1].Hash, second)
	}
}

// A stream that never presents a valid header is a genuine failure, not a
// continuation, and must still be counted and reported.
func TestParseReportsHeaderlessStream(t *testing.T) {
	t.Parallel()
	var warnings []string
	opts := Options{OnWarning: func(msg string) { warnings = append(warnings, msg) }}

	parsed := parseStream(opts, 0, "not a header at all\x00")
	if len(parsed.commits) != 0 {
		t.Errorf("got %d commits, want 0", len(parsed.commits))
	}
	if parsed.failed != 1 || parsed.total != 1 {
		t.Errorf("failed/total = %d/%d, want 1/1", parsed.failed, parsed.total)
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %q, want exactly one", warnings)
	}
}

func TestParseReportsProgress(t *testing.T) {
	t.Parallel()
	var current, total int
	opts := Options{OnProgress: func(got, expected int) { current, total = got, expected }}
	parseStream(opts, 7, header(testHash, "subject")+"\x00")
	if current != 1 || total != 7 {
		t.Errorf("progress = %d/%d, want 1/7", current, total)
	}
}

func TestIsRecordStart(t *testing.T) {
	t.Parallel()
	sha1 := strings.Repeat("a", 40)
	sha256 := strings.Repeat("0", 64)

	cases := []struct {
		name  string
		chunk string
		want  bool
	}{
		{"sha1 header", sha1 + "\x1fname", true},
		{"sha256 header", sha256 + "\x1fname", true},
		{"a file entry", "1\t0\tfile.go", false},
		{"a file entry whose path holds a separator", "1\t0\t" + strings.Repeat("a", 40) + "\x1f.go", false},
		{"short hash", strings.Repeat("a", 12) + "\x1fname", false},
		{"non-hex", strings.Repeat("z", 40) + "\x1fname", false},
		{"uppercase hex is not git's form", strings.Repeat("A", 40) + "\x1fname", false},
		{"no separator", sha1, false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isRecordStart(tc.chunk); got != tc.want {
				t.Errorf("isRecordStart(%q) = %v, want %v", tc.chunk, got, tc.want)
			}
		})
	}
}

// The sharded reader exists only to use more cores; it must return exactly what
// the single-invocation reader returns. Real repositories are too small here to
// cross ShardThreshold, so the sharded path is invoked directly.
func TestShardedReadMatchesSingleStream(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"basic", "merges", "binary", "single"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := fixture(t, name)
			opts := Options{RepoPath: repo, UseMailmap: true}

			single, err := collectStream(opts, nil)
			if err != nil {
				t.Fatalf("single stream: %v", err)
			}
			want, err := resolveMerges(single.commits, single.sections)
			if err != nil {
				t.Fatalf("placing the single stream's merges: %v", err)
			}

			hashes, err := revList(opts)
			if err != nil {
				t.Fatalf("enumerating commits: %v", err)
			}
			if len(hashes) != len(want) {
				t.Fatalf("the enumeration returned %d commits, the walk returned %d", len(hashes), len(want))
			}

			// More shards than commits exercises the empty-shard boundary too.
			for _, shards := range []int{2, 3, 7, len(hashes) + 2} {
				sharded, err := collectSharded(opts, hashes, shards)
				if err != nil {
					t.Fatalf("sharded (%d shards): %v", shards, err)
				}
				if sharded.failed != single.failed || sharded.total != single.total {
					t.Errorf("shards=%d: failed/total = %d/%d, want %d/%d",
						shards, sharded.failed, sharded.total, single.failed, single.total)
				}
				// A merge's parents may sit in another shard; placing its diffs
				// needs them all, which is why it happens after reassembly.
				got, err := resolveMerges(sharded.commits, sharded.sections)
				if err != nil {
					t.Fatalf("shards=%d: placing the merges: %v", shards, err)
				}
				if len(got) != len(want) {
					t.Fatalf("shards=%d: got %d commits, want %d", shards, len(got), len(want))
				}
				for i := range want {
					if got[i].Hash != want[i].Hash {
						t.Fatalf("shards=%d: commit %d = %s, want %s (order not preserved)",
							shards, i, got[i].Hash, want[i].Hash)
					}
					if !reflect.DeepEqual(got[i], want[i]) {
						t.Errorf("shards=%d: commit %s came back as %+v, want %+v", shards, got[i].Hash, got[i], want[i])
					}
				}
			}
		})
	}
}

// Sharding must never change how much work is done, only how it is divided.
func TestShardCount(t *testing.T) {
	t.Parallel()
	for _, degree := range []int{0, 1, 2, 7, 64} {
		if got := shardCount(ShardThreshold-1, degree); got != 1 {
			t.Errorf("degree %d: shardCount below threshold = %d, want 1", degree, got)
		}
		if got := shardCount(0, degree); got != 1 {
			t.Errorf("degree %d: shardCount(0) = %d, want 1", degree, got)
		}
		// Shards are never smaller than minCommitsPerShard.
		if got := shardCount(ShardThreshold, degree); got*minCommitsPerShard > ShardThreshold && got != 1 {
			t.Errorf("degree %d: shardCount(%d) = %d, which implies shards under %d commits",
				degree, ShardThreshold, got, minCommitsPerShard)
		}
		if got := shardCount(1_000_000, degree); got > maxShards {
			t.Errorf("degree %d: shardCount = %d, want at most %d", degree, got, maxShards)
		}
	}
	// An explicit degree is the degree, within those bounds; none derives one
	// from the cores (ADR-0052 clause 5).
	for degree, want := range map[int]int{1: 1, 2: 2, 7: 7} {
		if got := shardCount(1_000_000, degree); got != want {
			t.Errorf("shardCount at degree %d = %d, want %d", degree, got, want)
		}
	}
	if got := shardCount(1_000_000, 0); got != min(runtime.NumCPU(), maxShards) {
		t.Errorf("shardCount with no degree = %d, want the cores, %d, at most %d", got, runtime.NumCPU(), maxShards)
	}
}
