package collect

import (
	"strings"
	"testing"
)

// A commit subject may contain any byte, including the record separator that
// git is asked to delimit records with. Such a subject splits its own record,
// and the tail carries the numstat block for the commit before it.
func TestParseLogRejoinsSubjectContainingRecordSeparator(t *testing.T) {
	const hash = "4e45512de2d76e0366c5e7ac5d02119419bfc9ea"
	stream := "\x01" + hash +
		"\x1fRaymond Hettinger\x1fpython@rcn.com" +
		"\x1f2010-04-10T16:57:36Z\x1f2010-04-10T16:57:36Z" +
		"\x1f55b21389ef6f6f535dd04b710605ba3b605f7c3a" +
		"\x1fIssue 8361: Remove assert" +
		// The subject's own 0x01 splits the record here.
		"\x01 from functools\n3\t2\tLib/functools.py\n\n"

	var warnings []string
	opts := Options{OnWarning: func(msg string) { warnings = append(warnings, msg) }}

	commits, failed, total, err := parseLog(strings.NewReader(stream), opts, 0)
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0; the tail is a continuation, not a bad record", failed)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %q, want none", warnings)
	}
	if len(commits) != 1 {
		t.Fatalf("got %d commits, want 1", len(commits))
	}

	c := commits[0]
	if c.Hash != hash {
		t.Errorf("hash = %q, want %q", c.Hash, hash)
	}
	// The separator is part of the subject the repository actually holds, so it
	// is preserved rather than silently dropped.
	if want := "Issue 8361: Remove assert\x01 from functools"; c.Subject != want {
		t.Errorf("subject = %q, want %q", c.Subject, want)
	}
	// The numstat block belongs to this commit and must not be lost.
	if len(c.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(c.Files))
	}
	if f := c.Files[0]; f.Path != "Lib/functools.py" || f.Added != 3 || f.Deleted != 2 {
		t.Errorf("file = %+v, want Lib/functools.py +3 -2", f)
	}
}

// A stream that never presents a valid header is a genuine failure, not a
// continuation, and must still be counted and reported.
func TestParseLogReportsHeaderlessStream(t *testing.T) {
	var warnings []string
	opts := Options{OnWarning: func(msg string) { warnings = append(warnings, msg) }}

	commits, failed, total, err := parseLog(strings.NewReader("\x01not a header at all\n"), opts, 0)
	if err != nil {
		t.Fatalf("parseLog: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("got %d commits, want 0", len(commits))
	}
	if failed != 1 || total != 1 {
		t.Errorf("failed/total = %d/%d, want 1/1", failed, total)
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %q, want exactly one", warnings)
	}
}

func TestParseLogReportsProgress(t *testing.T) {
	const hash = "4e45512de2d76e0366c5e7ac5d02119419bfc9ea"
	stream := "\x01" + hash +
		"\x1fName\x1fname@example.com" +
		"\x1f2020-01-01T00:00:00Z\x1f2020-01-01T00:00:00Z" +
		"\x1f\x1fsubject\n"

	var current, total int
	opts := Options{OnProgress: func(got, expected int) {
		current, total = got, expected
	}}
	if _, _, _, err := parseLog(strings.NewReader(stream), opts, 7); err != nil {
		t.Fatalf("parseLog: %v", err)
	}
	if current != 1 || total != 7 {
		t.Errorf("progress = %d/%d, want 1/7", current, total)
	}
}

func TestIsRecordStart(t *testing.T) {
	sha1 := strings.Repeat("a", 40)
	sha256 := strings.Repeat("0", 64)

	cases := []struct {
		name  string
		chunk string
		want  bool
	}{
		{"sha1 header", sha1 + "\x1fname", true},
		{"sha256 header", sha256 + "\x1fname", true},
		{"subject tail", " from functools\n1\t0\tfile.go", false},
		{"short hash", strings.Repeat("a", 12) + "\x1fname", false},
		{"non-hex", strings.Repeat("z", 40) + "\x1fname", false},
		{"uppercase hex is not git's form", strings.Repeat("A", 40) + "\x1fname", false},
		{"no separator", sha1, false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
	for _, name := range []string{"basic", "merges", "binary", "single"} {
		t.Run(name, func(t *testing.T) {
			repo := fixture(t, name)
			opts := Options{RepoPath: repo, UseMailmap: true}

			want, wantFailed, wantTotal, err := collectStream(opts, nil)
			if err != nil {
				t.Fatalf("single stream: %v", err)
			}

			hashes, err := revList(opts)
			if err != nil {
				t.Fatalf("rev-list: %v", err)
			}
			if len(hashes) != len(want) {
				t.Fatalf("rev-list returned %d commits, walk returned %d", len(hashes), len(want))
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
