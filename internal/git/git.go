// Package git is the single place commitography invokes git (ADR-0065
// clause 1). No other package may start a git process, directly or indirectly,
// and the lint rule in .golangci.yml plus TestProcessExecutionSites in
// internal/checks hold the tree to that.
//
// Everything ADR-0065 clause 2 requires is applied here, by construction,
// rather than remembered at a call site: no shell, a sanitised environment,
// the mandatory configuration flags before the subcommand, user-derived
// operands behind a "--" separator, a context and a timeout (ADR-0044), the
// NUL-delimited output formats ADR-0045 makes necessary, and a size limit on
// every read. So is the pinned output configuration of ADR-0071, in pinned.go,
// which keeps an operator's own git configuration from moving a number. A
// caller describes what it wants with a Spec; it cannot describe an invocation
// that omits one of those.
//
// Cancellation terminates the process group rather than the direct child, so a
// descendant git spawns is not orphaned (ADR-0044 clause 4). The two platform
// implementations sit behind the group interface in group_unix.go and
// group_windows.go.
//
// A failed invocation is classified here as well (ADR-0041): see classify.go.
package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

const (
	// DefaultTimeout bounds one invocation. Profiling a 171,000-commit
	// repository measured 115 seconds of git time for the whole history read,
	// which is the longest invocation the product is known to make; this is
	// over five times that, so the bound stops a hung process without
	// interrupting a legitimate read (ADR-0065 clause 2, ADR-0044 clause 3).
	DefaultTimeout = 10 * time.Minute

	// MaxOutputBytes bounds a buffered read. The largest one the product makes
	// is the commit list of a large repository: 171,000 object names at 41
	// bytes is under 8 MB, so this is eight times the known maximum.
	MaxOutputBytes = 64 << 20

	// MaxRecordBytes bounds one NUL-delimited record. A record holds one
	// header or one file entry, so the bound is reached only by a commit
	// subject or a path that is itself megabytes long. Both are
	// attacker-controlled (ADR-0045), so the read is refused rather than
	// grown.
	MaxRecordBytes = 1 << 20

	// maxStderrBytes bounds what is kept of a failed invocation's standard
	// error. It is a diagnostic, not output.
	maxStderrBytes = 8 << 10
)

// Spec describes one git invocation. The zero value is not runnable on its
// own; build one with At.
type Spec struct {
	// Repo is the repository directory, passed as -C. Empty means the
	// process's working directory.
	Repo string

	// Args are the product's own arguments: the subcommand and the flags this
	// build chose. Nothing here is derived from a repository or a request.
	Args []string

	// Settings are extra configuration flags an invocation needs, as
	// "key=value". They are placed before the pinned and the mandatory ones,
	// so that an invocation can add a setting and cannot replace one: git
	// takes the last occurrence of a key, and those two sets come after it.
	// Set them with Configured.
	Settings []string

	// Operands are user-derived revisions and pathspecs. They are placed after
	// a "--" separator, so a value beginning with "-" cannot be read as an
	// option (ADR-0065 clause 2). Set them with Pathspecs.
	Operands []string

	// Env holds extra environment entries applied after sanitisation, for the
	// one variable an invocation legitimately sets for itself.
	Env []string

	// Stdin, when set, is the invocation's standard input.
	Stdin io.Reader

	// Timeout bounds this invocation; zero means DefaultTimeout.
	Timeout time.Duration

	// Limit bounds a buffered read in bytes; zero means MaxOutputBytes.
	Limit int

	// terminate places the "--" separator even with no operands, which
	// Pathspecs sets. A subcommand that takes no pathspec list, rev-parse
	// among them, echoes the separator into its own output, so it is never
	// added unasked.
	terminate bool

	// binary is the program to run. It is empty everywhere but in this
	// package's own tests, which point it at a recording program so that the
	// argument vector and the environment of a real invocation can be read
	// back rather than inferred from the code (ADR-0065 clause 2).
	binary string
}

// At describes an invocation against repo. args are the product's own
// arguments; anything user-derived goes through Pathspecs.
func At(repo string, args ...string) Spec {
	return Spec{Repo: repo, Args: args}
}

// Pathspecs places a "--" separator after the arguments, followed by paths.
// Calling it with no paths still places the separator, which is what stops a
// later operand from being read as an option.
func (s Spec) Pathspecs(paths ...string) Spec {
	s.terminate = true
	s.Operands = append(append([]string(nil), s.Operands...), paths...)
	return s
}

// Configured adds configuration flags, as "key=value". A key the pinned or
// the mandatory set also carries is overridden by that set, not the other way
// round.
func (s Spec) Configured(settings ...string) Spec {
	s.Settings = append(append([]string(nil), s.Settings...), settings...)
	return s
}

// WithEnv adds environment entries, applied after sanitisation.
func (s Spec) WithEnv(env ...string) Spec {
	s.Env = append(append([]string(nil), s.Env...), env...)
	return s
}

// WithStdin sets the invocation's standard input.
func (s Spec) WithStdin(r io.Reader) Spec {
	s.Stdin = r
	return s
}

// WithTimeout shortens or lengthens this invocation's bound.
func (s Spec) WithTimeout(d time.Duration) Spec {
	s.Timeout = d
	return s
}

// arguments is the full argument vector after the program name. The
// invocation's own settings come first, then the pinned output configuration
// (ADR-0071) and the mandatory hardening set last, because git takes the last
// occurrence of a configuration key: neither set can be replaced from a call
// site (ADR-0065 clause 2).
func (s Spec) arguments() []string {
	args := make([]string, 0, len(pinnedFlags())+len(configFlags())+2*len(s.Settings)+len(s.Args)+len(s.Operands)+4)
	for _, setting := range s.Settings {
		args = append(args, "-c", setting)
	}
	args = append(args, pinnedFlags()...)
	args = append(args, configFlags()...)
	if s.Repo != "" {
		args = append(args, "-C", s.Repo)
	}
	args = append(args, s.Args...)
	if s.terminate || len(s.Operands) > 0 {
		args = append(args, "--")
		args = append(args, s.Operands...)
	}
	return args
}

// program is the executable to run.
func (s Spec) program() string {
	if s.binary != "" {
		return s.binary
	}
	return "git"
}

// describe names the invocation for an error, without the repository path: an
// internal error's wrapping context is a diagnostic, and the arguments are
// enough to find the site (ADR-0067 clause 3).
func (s Spec) describe() string {
	return "git " + strings.Join(s.Args, " ")
}

func (s Spec) timeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return DefaultTimeout
}

func (s Spec) limit() int {
	if s.Limit > 0 {
		return s.Limit
	}
	return MaxOutputBytes
}

// LookPath reports where the git executable is found in PATH.
func LookPath() (string, error) {
	return exec.LookPath("git")
}

// Output runs git and returns standard output with its final line terminator
// removed. The read is bounded by the spec's limit; there is no unbounded
// read anywhere in this package.
func Output(ctx context.Context, spec Spec) (string, error) {
	p, err := Start(ctx, spec)
	if err != nil {
		return "", err
	}
	defer p.Close()

	out := &limitedBuffer{limit: spec.limit()}
	if _, err := io.Copy(out, p.stdout); err != nil && !out.exceeded {
		return "", core.Internalf(err, "reading the output of %s", spec.describe())
	}
	if err := p.Wait(); err != nil {
		return "", err
	}
	if out.exceeded {
		return "", core.Internalf(nil, "%s produced more than %d bytes of output", spec.describe(), spec.limit())
	}
	return trimFinalNewline(out.buf.String()), nil
}

// Records runs git and returns its NUL-delimited records, dropping the empty
// ones a terminator leaves behind. Nothing here splits on a newline: a path
// and a commit message may contain one (ADR-0045).
func Records(ctx context.Context, spec Spec) ([]string, error) {
	var records []string
	total := 0
	err := Scan(ctx, spec, func(record string) error {
		if record == "" {
			return nil
		}
		if total += len(record); total > spec.limit() {
			return core.Internalf(nil, "%s produced more than %d bytes of output", spec.describe(), spec.limit())
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// Scan runs git and hands each NUL-delimited record to yield as it is read, so
// a history larger than memory costs no more than its largest record. An empty
// record is passed on, because git's own framing uses one as a separator.
func Scan(ctx context.Context, spec Spec, yield func(record string) error) error {
	p, err := Start(ctx, spec)
	if err != nil {
		return err
	}
	defer p.Close()
	if err := p.Scan(yield); err != nil {
		return err
	}
	return p.Wait()
}

// ResolveDate returns the instant git resolves a date expression to, exactly
// as git log --since or --until would; bound is "since" or "until".
//
// Git completes a bare date with the current time of day and resolves a
// relative one against now, so one expression selects different commits at
// different hours. The resolved instant does not, which is why a report
// records it rather than the expression (ADR-0026 clause 4). Git reads "now"
// here from GIT_TEST_DATE_NOW, the variable its date parser consults in place
// of the process clock, set to now, so the injected clock governs git's
// reading of a date as it governs everything else (ADR-0042 clause 4).
func ResolveDate(ctx context.Context, repoPath, bound, expression string, now time.Time) (time.Time, error) {
	return resolveDate(ctx, At(repoPath), bound, expression, now)
}

// resolveDate is ResolveDate over a prepared Spec, so this package's own
// argument-vector checker can drive it the way every other entry point is
// driven.
func resolveDate(ctx context.Context, base Spec, bound, expression string, now time.Time) (time.Time, error) {
	var prefix string
	switch bound {
	case "since":
		prefix = "--max-age="
	case "until":
		prefix = "--min-age="
	default:
		return time.Time{}, core.Internalf(nil, "resolving a date bound named %q, which is neither since nor until", bound)
	}
	// The expression is user-derived and is bound to its option by "=", so it
	// cannot be read as an option itself. rev-parse takes no pathspec list and
	// echoes a "--" separator into its own output, so none is placed.
	base.Args = append(append([]string(nil), base.Args...), "rev-parse", "--"+bound+"="+expression)
	spec := base.WithEnv("GIT_TEST_DATE_NOW=" + strconv.FormatInt(now.Unix(), 10))
	out, err := Output(ctx, spec)
	if err != nil {
		return time.Time{}, err
	}
	seconds, err := strconv.ParseInt(strings.TrimPrefix(out, prefix), 10, 64)
	if err != nil || !strings.HasPrefix(out, prefix) {
		return time.Time{}, core.Internalf(err, "reading the instant git resolved a --%s bound to", bound)
	}
	return time.Unix(seconds, 0).UTC(), nil
}

// Process is one running invocation. Close releases it; a process that is
// still running when Close is called has its whole group terminated.
type Process struct {
	spec   Spec
	ctx    context.Context
	cmd    *exec.Cmd
	group  group
	stdout io.ReadCloser
	stderr *limitedBuffer
	cancel context.CancelFunc
	waited bool
}

// Start begins an invocation and returns it for streaming. The caller owns the
// returned Process and must Close it.
func Start(ctx context.Context, spec Spec) (*Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, spec.timeout())

	// exec.CommandContext takes an argument vector and no shell, which is the
	// only way this package starts a process (ADR-0065 clause 2).
	cmd := exec.CommandContext(ctx, spec.program(), spec.arguments()...)
	cmd.Env = environment(spec.Env)
	cmd.Stdin = spec.Stdin
	stderr := &limitedBuffer{limit: maxStderrBytes}
	cmd.Stderr = stderr

	g := newGroup()
	g.prepare(cmd)
	// exec's own cancellation kills the direct child only. Terminating the
	// group is what keeps a descendant from being orphaned (ADR-0044
	// clause 4); WaitDelay stops Wait from blocking on a pipe a survivor still
	// holds.
	cmd.Cancel = func() error { return g.terminate(cmd) }
	cmd.WaitDelay = time.Second

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, core.Internalf(err, "opening the output pipe of %s", spec.describe())
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, classify(spec, "", err)
	}
	g.adopt(cmd)
	return &Process{spec: spec, ctx: ctx, cmd: cmd, group: g, stdout: stdout, stderr: stderr, cancel: cancel}, nil
}

// Scan hands each NUL-delimited record of standard output to yield.
func (p *Process) Scan(yield func(record string) error) error {
	scanner := bufio.NewScanner(p.stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), MaxRecordBytes)
	scanner.Split(splitRecords)
	for scanner.Scan() {
		if err := yield(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		if p.ctxErr() != nil {
			return p.ctxErr()
		}
		return core.Internalf(err, "reading the output of %s, in records of at most %d bytes",
			p.spec.describe(), MaxRecordBytes)
	}
	return nil
}

// Wait drains what is left of standard output, so git never blocks on a full
// pipe, and reaps the process.
func (p *Process) Wait() error {
	if p.waited {
		return nil
	}
	p.waited = true
	_, _ = io.Copy(io.Discard, p.stdout)
	err := p.cmd.Wait()
	if ctxErr := p.ctxErr(); ctxErr != nil {
		return ctxErr
	}
	if err != nil {
		return classify(p.spec, p.stderr.buf.String(), err)
	}
	return nil
}

// Close terminates the process group if the invocation is still running and
// releases the timeout.
func (p *Process) Close() {
	if !p.waited {
		p.waited = true
		_ = p.group.terminate(p.cmd)
		_ = p.cmd.Wait()
	}
	p.cancel()
}

// ctxErr reports the context's own error, which outranks whatever git said
// about being killed: a cancelled or timed-out invocation is that, not a git
// failure.
func (p *Process) ctxErr() error {
	return p.ctx.Err()
}

// splitRecords is the NUL split used everywhere in this package. An empty
// record between two terminators is returned rather than skipped, because
// git's framing uses one to separate commits.
func splitRecords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// limitedBuffer accumulates up to limit bytes and discards the rest, recording
// that it did. It is what makes every buffered read bounded (ADR-0065
// clause 2).
type limitedBuffer struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.buf.Len(); room > 0 {
		if len(p) <= room {
			return b.buf.Write(p)
		}
		b.exceeded = true
		if _, err := b.buf.Write(p[:room]); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	b.exceeded = true
	return len(p), nil
}

// trimFinalNewline removes the one line terminator git puts after a scalar
// value. Only the last one goes: a value may legitimately end in whitespace,
// and a path may contain a newline.
func trimFinalNewline(s string) string {
	s = strings.TrimSuffix(s, "\n")
	return strings.TrimSuffix(s, "\r")
}
