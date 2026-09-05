package collect

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/model"
	"github.com/sinanganiz/commitography/internal/version"
)

const (
	// recordSep and fieldSep are control characters that cannot occur in a
	// commit subject, unlike commas or pipes.
	recordSep = '\x01'
	fieldSep  = "\x1f"

	// headerFields is the number of %-placeholders in the pretty format below.
	headerFields = 7

	// prettyFormat lays out one record header per commit: hash, author name,
	// author email, author date, committer date, parents, subject.
	prettyFormat = "format:%x01%H%x1f%aN%x1f%aE%x1f%aI%x1f%cI%x1f%P%x1f%s"

	// maxParseFailureRatio is the share of unparsable records above which the
	// history is considered untrustworthy rather than merely imperfect.
	maxParseFailureRatio = 0.01
)

// Options controls how history is read.
type Options struct {
	RepoPath   string
	UseMailmap bool
	Since      string // passed through to git log --since, empty means no bound
	Until      string // passed through to git log --until, empty means no bound

	// OnWarning, when set, receives non-fatal diagnostics such as parse
	// failures. It may be called before Collect returns.
	OnWarning func(string)
}

func (o Options) warn(format string, args ...any) {
	if o.OnWarning != nil {
		o.OnWarning(fmt.Sprintf(format, args...))
	}
}

// Collect reads the full history in a single git invocation.
func Collect(opts Options) (*model.History, error) {
	info, err := Preflight(opts.RepoPath)
	if err != nil {
		return nil, err
	}

	args := []string{"log", "--all", "--numstat", "--no-renames", "--date-order"}
	if opts.UseMailmap {
		args = append(args, "--use-mailmap")
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	args = append(args, "--pretty="+prettyFormat)

	cmd := gitCommand(opts.RepoPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &strings.Builder{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting git log: %w", err)
	}

	commits, failed, total, parseErr := parseLog(stdout, opts)

	// Drain anything left so git never blocks on a full pipe, then reap.
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()

	if parseErr != nil {
		return nil, parseErr
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("git log failed: %w", waitErr)
		}
		return nil, fmt.Errorf("git log failed: %s", msg)
	}

	if failed > 0 {
		opts.warn("%d of %d commit records could not be parsed and were skipped", failed, total)
		if total > 0 && float64(failed)/float64(total) > maxParseFailureRatio {
			return nil, fmt.Errorf("%d of %d commit records failed to parse (over %.0f%%); refusing to report statistics from an unreliable read",
				failed, total, maxParseFailureRatio*100)
		}
	}

	return &model.History{
		SchemaVersion: model.SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		ToolVersion:   version.Version,
		Repository:    info,
		Commits:       commits,
	}, nil
}

// parseLog streams git log output, splitting on the record separator. Output is
// never held in memory as a single string; only one record is materialized at a
// time, so a repository with a million commits costs no more than its largest
// commit.
func parseLog(r io.Reader, opts Options) (commits []model.Commit, failed, total int, err error) {
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		chunk, readErr := br.ReadString(recordSep)
		chunk = strings.TrimSuffix(chunk, string(recordSep))

		if strings.TrimSpace(chunk) != "" {
			total++
			c, parseErr := parseRecord(chunk)
			if parseErr != nil {
				failed++
				opts.warn("skipping unparsable commit record: %v", parseErr)
			} else {
				commits = append(commits, c)
			}
		}

		if readErr == io.EOF {
			return commits, failed, total, nil
		}
		if readErr != nil {
			return commits, failed, total, fmt.Errorf("reading git log output: %w", readErr)
		}
	}
}

// parseRecord turns one record (header line plus numstat block) into a Commit.
func parseRecord(chunk string) (model.Commit, error) {
	var c model.Commit

	header, rest, _ := strings.Cut(chunk, "\n")
	fields := strings.Split(header, fieldSep)
	if len(fields) != headerFields {
		return c, fmt.Errorf("expected %d header fields, got %d", headerFields, len(fields))
	}

	authorDate, authorOffset, err := parseGitTime(fields[3])
	if err != nil {
		return c, fmt.Errorf("commit %s: author date: %w", short(fields[0]), err)
	}
	committerDate, committerOffset, err := parseGitTime(fields[4])
	if err != nil {
		return c, fmt.Errorf("commit %s: committer date: %w", short(fields[0]), err)
	}

	var parents []string
	if p := strings.TrimSpace(fields[5]); p != "" {
		parents = strings.Split(p, " ")
	}

	c = model.Commit{
		Hash:                     fields[0],
		AuthorName:               fields[1],
		AuthorEmail:              fields[2],
		AuthorDate:               authorDate,
		AuthorTZOffsetMinutes:    authorOffset,
		CommitterDate:            committerDate,
		CommitterTZOffsetMinutes: committerOffset,
		Parents:                  parents,
		IsMerge:                  len(parents) > 1,
		Subject:                  fields[6],
		Files:                    parseNumstat(rest),
	}
	return c, nil
}

// parseNumstat reads the `<added>\t<deleted>\t<path>` block that follows a
// record header. Blank lines and malformed lines are skipped rather than
// failing the commit, since a missing file row is less damaging than a lost
// commit.
func parseNumstat(block string) []model.FileChange {
	var files []model.FileChange
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		fc := model.FileChange{Path: unquoteGitPath(parts[2])}
		if parts[0] == "-" && parts[1] == "-" {
			fc.IsBinary = true
		} else {
			added, err1 := strconv.Atoi(parts[0])
			deleted, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				continue
			}
			fc.Added = added
			fc.Deleted = deleted
		}
		files = append(files, fc)
	}
	return files
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

// unquoteGitPath reverses git's C-style quoting. git quotes any path containing
// a quote, a backslash or a control character even when core.quotePath is off.
func unquoteGitPath(p string) string {
	if len(p) < 2 || p[0] != '"' || p[len(p)-1] != '"' {
		return p
	}
	if unquoted, err := strconv.Unquote(p); err == nil {
		return unquoted
	}
	return p
}

func short(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
