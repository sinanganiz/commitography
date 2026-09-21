// Package collect is the collect stage (ADR-0020): one pass over the history
// that produces the normalized commit records, read in parallel (ADR-0052)
// through the git package (ADR-0065).
package collect

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
)

const (
	// fieldSep separates the header's fields. It is a control character
	// rather than a comma or a pipe, but a commit subject may contain one
	// too, which is why the subject is the last field and the header is split
	// into a fixed number of parts rather than into as many as it happens to
	// contain.
	fieldSep = "\x1f"

	// headerFields is the number of %-placeholders in the pretty format below.
	headerFields = 8

	// prettyFormat lays out one record header per commit: hash, author name,
	// author email, the author email as the commit records it, author date,
	// committer date, parents, subject. The name and the first email have
	// .mailmap applied, which is the first step of identity resolution
	// (docs/metrics.md section 1); the second email is the source address
	// before it, which the identity layer counts (section 14).
	//
	// The subject is last because it is the one field that may contain the
	// field separator; %s is the first line of the message, so it cannot
	// contain a newline, which is what lets the header be cut from the first
	// file entry that follows it.
	prettyFormat = "format:%H%x1f%aN%x1f%aE%x1f%ae%x1f%aI%x1f%cI%x1f%P%x1f%s"

	// maxParseFailureRatio is the share of unparsable records above which the
	// history is considered untrustworthy rather than merely imperfect.
	maxParseFailureRatio = 0.01

	// ShardThreshold is the commit count from which history is read by
	// several git processes at once. Below it the extra process spawns cost
	// more than the parallelism returns, whatever degree was asked for. It is
	// exported so that the parallel determinism checker can confirm its
	// fixture reaches it.
	ShardThreshold = 5000

	// maxShards caps the number of concurrent git processes. Beyond roughly this
	// many the run becomes disk-bound rather than CPU-bound and the extra
	// processes only add contention.
	maxShards = 16

	// minCommitsPerShard keeps shards large enough to be worth a process.
	minCommitsPerShard = 500
)

// Collector is the collect stage. It holds the clock that stamps the history
// it produces and the file access its preflight uses, both injected
// (ADR-0042 clause 1).
type Collector struct {
	clock core.Clock
	files core.Filesystem
}

// New constructs the collect stage.
func New(clock core.Clock, files core.Filesystem) *Collector {
	return &Collector{clock: clock, files: files}
}

// Options controls how history is read.
type Options struct {
	RepoPath string

	// SuppliedPath is RepoPath in the form the operator gave it, before any
	// resolution. Only messages use it, and only they may (ADR-0067 clause 5).
	// It is empty where the path did not come from the operator, such as a
	// server request, in which case no message names it at all.
	SuppliedPath string

	// Analysis is the analysis plane the records are normalized under
	// (normalize.go). When it is set it is the only source of the read
	// options as well, and UseMailmap, Since and Until below must be left
	// unset; the pipeline sets it. When it is nil, the records are normalized
	// under the built-in plane and those three fields are the read options.
	Analysis *config.Analysis

	// Parallelism is how many readers a history above ShardThreshold is split
	// across (ADR-0052 clause 1). Zero or less derives it from the available
	// cores; it is never more than maxShards, and a reader is never given
	// fewer than minCommitsPerShard commits. It changes no record (clause 6).
	Parallelism int

	UseMailmap bool
	// Since and Until are passed to git log --since and --until; empty means
	// no bound. The pipeline passes the instants ResolveDateBounds returned,
	// so the commits read do not depend on the hour of the run.
	Since   string
	Until   string
	Context context.Context

	// ToolVersion is recorded in the history artifact. It is injected, never
	// read from build metadata here (ADR-0061 clause 4).
	ToolVersion string

	// OnWarning, when set, receives non-fatal diagnostics such as parse
	// failures. It may be called before Collect returns.
	OnWarning func(string)
	// OnProgress receives parsed-record progress. Total is zero when the commit
	// list could not be enumerated before streaming began.
	OnProgress func(current, total int)
}

func (o Options) warn(format string, args ...any) {
	if o.OnWarning != nil {
		o.OnWarning(fmt.Sprintf(format, args...))
	}
}

func (o Options) context() context.Context {
	if o.Context == nil {
		return context.Background()
	}
	return o.Context
}

// resolved returns the options with the analysis plane settled: the one the
// caller gave, whose read options replace the separate fields, or the built-in
// one. Giving both is a defect in the caller, not something to reconcile.
func (o Options) resolved() (Options, config.Analysis, error) {
	if o.Analysis == nil {
		return o, config.Default(), nil
	}
	if o.UseMailmap || o.Since != "" || o.Until != "" {
		return o, config.Analysis{}, core.Internalf(nil,
			"the collect stage was given an analysis plane and separate read options; the plane carries them")
	}
	cfg := *o.Analysis
	o.UseMailmap, o.Since, o.Until = cfg.UseMailmap, cfg.Since, cfg.Until
	return o, cfg, nil
}

func (o Options) progress(current, total int) {
	if o.OnProgress != nil {
		o.OnProgress(current, total)
	}
}

// Collect reads the full history and normalizes it: every record carries the
// per-commit definitions of docs/metrics.md section 1 (normalize.go).
//
// Reading is still a single pass over the history — one diff per commit, never
// a git invocation per commit — but on a large repository that pass is split
// across several concurrent `git log` processes. Profiling a 171,000-commit
// repository showed the Go side using about six seconds of CPU while git took
// 115 seconds to produce the numstat stream: the run is bounded entirely by
// git's diff computation, which is single-threaded per process. Sharding the
// commit list across cores is the only lever that moves it, and it is what
// makes the 100,000-commits-in-60-seconds requirement reachable.
//
// A single pass does not mean a single git invocation. What must be avoided is
// a git call per commit; the sharded read keeps the one-diff-per-commit property.
// Small repositories still take the single-invocation path.
func (c *Collector) Collect(opts Options) (*model.History, error) {
	opts, cfg, err := opts.resolved()
	if err != nil {
		return nil, err
	}
	info, err := c.Preflight(opts.context(), opts.RepoPath, opts.SuppliedPath)
	if err != nil {
		return nil, err
	}

	commits, failed, total, err := readHistory(opts)
	if err != nil {
		return nil, err
	}

	if failed > 0 {
		opts.warn("%d of %d commit records could not be parsed and were skipped", failed, total)
		if total > 0 && float64(failed)/float64(total) > maxParseFailureRatio {
			return nil, core.Internalf(nil, "%d of %d commit records failed to parse, over %.0f%%; refusing to report statistics from an unreliable read",
				failed, total, maxParseFailureRatio*100)
		}
	}

	attributes, err := analysedAttributes(opts.context(), opts.RepoPath, info.HeadCommit)
	if err != nil {
		return nil, err
	}
	commits, err = normalize(commits, cfg, attributes)
	if err != nil {
		return nil, err
	}

	return &model.History{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   c.clock.Now().UTC(),
		ToolVersion:   opts.ToolVersion,
		Repository:    info,
		Attributes:    string(attributes),
		Commits:       commits,
	}, nil
}

// readHistory picks between the sharded and single-stream readers.
func readHistory(opts Options) (commits []model.Commit, failed, total int, err error) {
	// rev-list carries no diff cost, so asking for the commit list first is
	// cheap enough to be worth it even when sharding is then declined.
	hashes, err := revList(opts)
	if err != nil {
		// A repository shape rev-list cannot enumerate is still readable by the
		// ordinary walk, so this is not fatal.
		return collectStream(opts, nil)
	}
	if shards := shardCount(len(hashes), opts.Parallelism); shards > 1 {
		return collectSharded(opts, hashes, shards)
	}
	// The rev-list pass gives the streaming parser an exact denominator even
	// though the small-history path still uses one git log process.
	local := opts
	local.OnProgress = opts.OnProgress
	localExpected := len(hashes)
	return collectStreamWithTotal(local, nil, localExpected)
}

// shardCount decides how many git processes read a history of the given size.
// degree is the parallelism asked for; zero or less derives it from the
// available cores, never a fixed number (ADR-0052 clause 5).
func shardCount(commits, degree int) int {
	if commits < ShardThreshold {
		return 1
	}
	shards := degree
	if shards <= 0 {
		shards = runtime.NumCPU()
	}
	if shards > maxShards {
		shards = maxShards
	}
	if limit := commits / minCommitsPerShard; shards > limit {
		shards = limit
	}
	if shards < 1 {
		return 1
	}
	return shards
}

// revList enumerates the commits that will be read, in the same order the
// ordinary walk would produce them.
//
// It is a log rather than a rev-list because rev-list has no NUL-delimited
// output form and this package parses no lines (ADR-0065 clause 2). The
// object names it returns are hexadecimal either way; the format is what the
// rule is about, not this output's contents.
func revList(opts Options) ([]string, error) {
	args := []string{"log", "-z", "--pretty=format:%H", "--all", "--date-order"}
	args = append(args, boundArgs(opts)...)
	return git.Records(opts.context(), git.At(opts.RepoPath, args...).Pathspecs())
}

// boundArgs are the date bounds, already resolved to instants by the caller.
// They are user-derived, and each is bound to its option with "=", so neither
// can be read as an option of its own.
func boundArgs(opts Options) []string {
	var args []string
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	return args
}

// logArgs builds the `git log` arguments shared by both readers. When hashes
// are supplied the commits come from stdin and ancestry is not walked, so each
// shard reads exactly the commits it was given.
//
// -z is what makes the whole read NUL-delimited: it separates commits with a
// NUL instead of a newline and stops git munging path names, so a file name
// containing a newline, a quote or a control character arrives intact
// (ADR-0065 clause 2, ADR-0045).
func logArgs(opts Options, fromStdin bool) []string {
	args := []string{"log", "-z", "--numstat", "--no-renames"}
	if fromStdin {
		args = append(args, "--no-walk", "--stdin")
	} else {
		args = append(args, "--all", "--date-order")
		args = append(args, boundArgs(opts)...)
	}
	if opts.UseMailmap {
		args = append(args, "--use-mailmap")
	}
	return append(args, "--pretty="+prettyFormat)
}

// collectStream runs one git log and parses its output. When hashes is non-nil
// they are fed on stdin and only those commits are read.
func collectStream(opts Options, hashes []string) (commits []model.Commit, failed, total int, err error) {
	expected := len(hashes)
	return collectStreamWithTotal(opts, hashes, expected)
}

func collectStreamWithTotal(opts Options, hashes []string, expected int) (commits []model.Commit, failed, total int, err error) {
	spec := git.At(opts.RepoPath, logArgs(opts, hashes != nil)...).Pathspecs()
	if hashes != nil {
		// The object names were produced by the enumeration pass above, so
		// they are the product's own values rather than user-derived ones.
		spec = spec.WithStdin(strings.NewReader(strings.Join(hashes, "\n") + "\n"))
	}
	process, err := git.Start(opts.context(), spec)
	if err != nil {
		return nil, 0, 0, err
	}
	defer process.Close()

	parser := newLogParser(opts, expected)
	if parseErr := process.Scan(parser.record); parseErr != nil {
		return nil, 0, 0, parseErr
	}
	parser.finish()

	// Wait drains what is left, so git never blocks on a full pipe, and
	// classifies a failure (ADR-0041).
	if err := process.Wait(); err != nil {
		return nil, 0, 0, err
	}
	return parser.commits, parser.failed, parser.total, nil
}

// collectSharded splits the commit list into contiguous runs and reads each with
// its own git process. The runs are contiguous slices of the rev-list order and
// are reassembled in that order, so the result is identical to a single walk.
func collectSharded(opts Options, hashes []string, shards int) (commits []model.Commit, failed, total int, err error) {
	type result struct {
		commits       []model.Commit
		failed, total int
		progress      int
		warnings      []string
		err           error
	}

	results := make([]result, shards)
	size := (len(hashes) + shards - 1) / shards

	var wg sync.WaitGroup
	for i := 0; i < shards; i++ {
		start := i * size
		if start >= len(hashes) {
			break
		}
		end := start + size
		if end > len(hashes) {
			end = len(hashes)
		}

		idx, chunk := i, hashes[start:end]
		wg.Go(func() {
			// Warnings are buffered per shard rather than reported as they occur:
			// OnWarning belongs to the caller and is not required to be safe for
			// concurrent use.
			local := opts
			local.OnWarning = func(msg string) {
				results[idx].warnings = append(results[idx].warnings, msg)
			}
			local.OnProgress = func(current, _ int) {
				results[idx].progress = current
			}
			c, f, t, err := collectStreamWithTotal(local, chunk, len(chunk))
			results[idx].commits, results[idx].failed, results[idx].total, results[idx].err = c, f, t, err
		})
	}
	wg.Wait()

	commits = make([]model.Commit, 0, len(hashes))
	progress := 0
	for _, r := range results {
		if r.err != nil {
			return nil, 0, 0, r.err
		}
		for _, msg := range r.warnings {
			if opts.OnWarning != nil {
				opts.OnWarning(msg)
			}
		}
		commits = append(commits, r.commits...)
		failed += r.failed
		total += r.total
		progress += r.progress
		opts.progress(progress, len(hashes))
	}
	return commits, failed, total, nil
}

// logParser turns the NUL-delimited record stream of `git log -z --numstat`
// into commits. It holds one record at a time, so a repository with a million
// commits costs no more than its largest record.
//
// The stream's framing is git's own. Each commit is
//
//	<header>\n<added>\t<deleted>\t<path>\0<added>\t<deleted>\t<path>\0 ... \0
//
// where the last NUL is the commit separator -z adds, so a commit with file
// entries produces an empty record before the next header, and a commit with
// none — a merge, an empty commit — produces a header record and nothing
// else. Nothing is split on a newline: the header's own newline is cut once,
// which is exact because %s is the first line of the message and cannot
// contain one, and a path is whatever remains of its record, newlines and
// quotes included (ADR-0045).
type logParser struct {
	opts     Options
	expected int

	commits       []model.Commit
	failed, total int

	// pending is the commit being assembled, which stays open until a header
	// or the end of the stream closes it, because its file entries arrive as
	// separate records.
	pending *model.Commit
}

func newLogParser(opts Options, expected int) *logParser {
	return &logParser{opts: opts, expected: expected}
}

// record consumes one NUL-delimited record.
func (p *logParser) record(chunk string) error {
	switch {
	case isRecordStart(chunk):
		p.finish()
		p.total++
		p.opts.progress(p.total, p.expected)
		header, first, hasFiles := strings.Cut(chunk, "\n")
		commit, err := parseHeader(header)
		if err != nil {
			p.failed++
			p.opts.warn("skipping unparsable commit record: %v", err)
			return nil
		}
		if hasFiles {
			commit.Files = appendFileChange(commit.Files, first)
		}
		p.pending = &commit
	case chunk == "":
		// The separator git writes between commits.
	case p.pending != nil:
		p.pending.Files = appendFileChange(p.pending.Files, chunk)
	default:
		// Output before any header. The read is producing something this
		// parser does not recognise, which the caller turns into a refusal
		// once enough records fail.
		p.total++
		p.opts.progress(p.total, p.expected)
		p.failed++
		p.opts.warn("skipping unparsable commit record: no record header")
	}
	return nil
}

// finish closes the commit being assembled.
func (p *logParser) finish() {
	if p.pending != nil {
		p.commits = append(p.commits, *p.pending)
		p.pending = nil
	}
}

// isRecordStart reports whether a record begins with a header: an object name
// followed by the field separator. A file entry cannot be mistaken for one,
// because its first field is a decimal count or a dash followed by a tab, and
// a tab is not a hexadecimal digit.
func isRecordStart(chunk string) bool {
	i := strings.Index(chunk, fieldSep)
	// 40 hex digits for SHA-1, 64 for SHA-256.
	if i != 40 && i != 64 {
		return false
	}
	for j := 0; j < i; j++ {
		c := chunk[j]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// parseHeader turns one record header into a Commit without its files.
//
// Its errors are unclassified on purpose: they never leave this package. The
// caller counts them, warns, and refuses the read only once too many records
// fail, which is the error that does cross the boundary and is classified
// there (ADR-0041 clause 1).
func parseHeader(header string) (model.Commit, error) {
	var c model.Commit

	// SplitN, not Split: the subject is the last field and may contain the
	// field separator itself, in which case it keeps it rather than producing
	// a record with too many fields.
	fields := strings.SplitN(header, fieldSep, headerFields)
	if len(fields) != headerFields {
		return c, fmt.Errorf("expected %d header fields, got %d", headerFields, len(fields))
	}

	authorDate, authorOffset, err := parseGitTime(fields[4])
	if err != nil {
		return c, fmt.Errorf("commit %s: author date: %w", short(fields[0]), err)
	}
	committerDate, committerOffset, err := parseGitTime(fields[5])
	if err != nil {
		return c, fmt.Errorf("commit %s: committer date: %w", short(fields[0]), err)
	}

	var parents []string
	if p := strings.TrimSpace(fields[6]); p != "" {
		parents = strings.Split(p, " ")
	}

	c = model.Commit{
		Hash:                     fields[0],
		AuthorName:               fields[1],
		AuthorEmail:              fields[2],
		AuthorSourceEmail:        fields[3],
		AuthorDate:               authorDate,
		AuthorTZOffsetMinutes:    authorOffset,
		CommitterDate:            committerDate,
		CommitterTZOffsetMinutes: committerOffset,
		Parents:                  parents,
		IsMerge:                  len(parents) > 1,
		Subject:                  fields[7],
	}
	return c, nil
}

// appendFileChange reads one `<added>\t<deleted>\t<path>` entry and appends
// it. A malformed entry is skipped rather than failing the commit, since a
// missing file row is less damaging than a lost commit.
//
// The path is everything after the second tab, whatever it contains: with -z
// git neither quotes nor escapes it, so a name holding a newline, a quote or
// a control character is taken verbatim. Reversing C-style quoting here would
// corrupt a name that genuinely begins and ends with a quote.
func appendFileChange(files []model.FileChange, entry string) []model.FileChange {
	if strings.TrimSpace(entry) == "" {
		return files
	}
	parts := strings.SplitN(entry, "\t", 3)
	if len(parts) != 3 {
		return files
	}
	fc := model.FileChange{Path: parts[2]}
	if parts[0] == "-" && parts[1] == "-" {
		fc.IsBinary = true
	} else {
		added, err1 := strconv.Atoi(parts[0])
		deleted, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return files
		}
		fc.Added = added
		fc.Deleted = deleted
	}
	return append(files, fc)
}

// parseGitTime parses a strict ISO 8601 timestamp and returns it together with
// its UTC offset in minutes. The timestamp keeps its original location so that
// hour-of-day analysis stays in the author's local time.
func parseGitTime(s string) (time.Time, int, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, 0, err
	}
	_, offsetSeconds := t.Zone()
	return t, offsetSeconds / 60, nil
}

func short(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
