package collect

import (
	"strings"
	"testing"
)

// The record stream this package reads is what `git log -z --numstat`
// produces. header builds one record header in that format.
func header(hash, subject string) string {
	return hash +
		"\x1fRaymond Hettinger\x1fpython@rcn.com\x1fpython@rcn.com" +
		"\x1f2010-04-10T16:57:36Z\x1f2010-04-10T16:57:36Z" +
		"\x1f55b21389ef6f6f535dd04b710605ba3b605f7c3a" +
		"\x1f" + subject
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
	stream := header(testHash, subject) + "\n3\t2\tLib/functools.py\x00\x00"

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
	if f := c.Files[0]; f.Path != "Lib/functools.py" || f.Added != 3 || f.Deleted != 2 {
		t.Errorf("file = %+v, want Lib/functools.py +3 -2", f)
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
	for i, path := range hostile {
		if i > 0 {
			stream += "\x00"
		}
		stream += "1\t0\t" + path
	}
	stream += "\x00\x00"

	parsed := parseStream(Options{}, 0, stream)
	if len(parsed.commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(parsed.commits))
	}
	files := parsed.commits[0].Files
	if len(files) != len(hostile) {
		t.Fatalf("got %d files, want %d; a record was split, merged or dropped", len(files), len(hostile))
	}
	for i, path := range hostile {
		if files[i].Path != path {
			t.Errorf("file %d = %q, want %q", i, files[i].Path, path)
		}
	}
}

// git writes no file entries for a merge or an empty commit, so such a record
// is a header and the separator alone. Neither may swallow the commit that
// follows it.
func TestParseReadsCommitsWithoutFileEntries(t *testing.T) {
	t.Parallel()
	second := "0123456789abcdef0123456789abcdef01234567"
	stream := header(testHash, "Merge branch 'topic'") + "\x00" +
		header(second, "feat: with a file") + "\n1\t0\tmain.go\x00\x00"

	parsed := parseStream(Options{}, 0, stream)
	if len(parsed.commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(parsed.commits))
	}
	if len(parsed.commits[0].Files) != 0 {
		t.Errorf("the merge record gained %d files", len(parsed.commits[0].Files))
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
// cross shardThreshold, so the sharded path is invoked directly.
func TestShardedReadMatchesSingleStream(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"basic", "merges", "binary", "single"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := fixture(t, name)
			opts := Options{RepoPath: repo, UseMailmap: true}

			want, wantFailed, wantTotal, err := collectStream(opts, nil)
			if err != nil {
				t.Fatalf("single stream: %v", err)
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
				got, failed, total, err := collectSharded(opts, hashes, shards)
				if err != nil {
					t.Fatalf("sharded (%d shards): %v", shards, err)
				}
				if failed != wantFailed || total != wantTotal {
					t.Errorf("shards=%d: failed/total = %d/%d, want %d/%d",
						shards, failed, total, wantFailed, wantTotal)
				}
				if len(got) != len(want) {
					t.Fatalf("shards=%d: got %d commits, want %d", shards, len(got), len(want))
				}
				for i := range want {
					if got[i].Hash != want[i].Hash {
						t.Fatalf("shards=%d: commit %d = %s, want %s (order not preserved)",
							shards, i, got[i].Hash, want[i].Hash)
					}
					if len(got[i].Files) != len(want[i].Files) {
						t.Errorf("shards=%d: commit %s has %d files, want %d",
							shards, got[i].Hash, len(got[i].Files), len(want[i].Files))
					}
					if got[i].Subject != want[i].Subject {
						t.Errorf("shards=%d: commit %s subject = %q, want %q",
							shards, got[i].Hash, got[i].Subject, want[i].Subject)
					}
				}
			}
		})
	}
}

// Sharding must never change how much work is done, only how it is divided.
func TestShardCount(t *testing.T) {
	t.Parallel()
	if got := shardCount(shardThreshold - 1); got != 1 {
		t.Errorf("shardCount below threshold = %d, want 1", got)
	}
	if got := shardCount(0); got != 1 {
		t.Errorf("shardCount(0) = %d, want 1", got)
	}
	// Shards are never smaller than minCommitsPerShard.
	if got := shardCount(shardThreshold); got*minCommitsPerShard > shardThreshold && got != 1 {
		t.Errorf("shardCount(%d) = %d, which implies shards under %d commits",
			shardThreshold, got, minCommitsPerShard)
	}
	if got := shardCount(1_000_000); got > maxShards {
		t.Errorf("shardCount = %d, want at most %d", got, maxShards)
	}
}
