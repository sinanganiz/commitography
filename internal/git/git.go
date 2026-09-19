// Package git is the single place commitography shells out to git (ADR-0065
// clause 1). Every invocation goes through here so that flags like
// core.quotePath are applied uniformly and stderr is always turned into a
// useful error. The invocation hardening of ADR-0065 clause 2 and the context
// binding of ADR-0044 belong here and nowhere else.
package git

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

// Args prefixes the arguments common to every git invocation.
// core.quotePath=false makes git emit paths raw and UTF-8 rather than escaping
// non-ASCII bytes, so a path is stored exactly as the repository holds it.
func Args(repoPath string, args ...string) []string {
	base := []string{"-c", "core.quotePath=false"}
	if repoPath != "" {
		base = append(base, "-C", repoPath)
	}
	return append(base, args...)
}

// LookPath reports where the git executable is found in PATH.
func LookPath() (string, error) {
	return exec.LookPath("git")
}

// Command builds an *exec.Cmd for a git invocation against repoPath.
func Command(repoPath string, args ...string) *exec.Cmd {
	return CommandContext(context.Background(), repoPath, args...)
}

// CommandContext builds a cancellable *exec.Cmd for a git invocation against
// repoPath.
func CommandContext(ctx context.Context, repoPath string, args ...string) *exec.Cmd {
	if ctx == nil {
		ctx = context.Background()
	}
	return exec.CommandContext(ctx, "git", Args(repoPath, args...)...)
}

// Run executes git and returns trimmed stdout, wrapping git's own stderr in the
// error so the caller can surface something actionable.
func Run(repoPath string, args ...string) (string, error) {
	return RunContext(context.Background(), repoPath, args...)
}

// RunContext executes git with cancellation support and returns trimmed stdout.
func RunContext(ctx context.Context, repoPath string, args ...string) (string, error) {
	return run(ctx, CommandContext(ctx, repoPath, args...), args)
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
	var prefix string
	switch bound {
	case "since":
		prefix = "--max-age="
	case "until":
		prefix = "--min-age="
	default:
		return time.Time{}, core.Internalf(nil, "resolving a date bound named %q, which is neither since nor until", bound)
	}
	args := []string{"rev-parse", "--" + bound + "=" + expression}
	cmd := CommandContext(ctx, repoPath, args...)
	cmd.Env = append(cmd.Environ(), "GIT_TEST_DATE_NOW="+strconv.FormatInt(now.Unix(), 10))
	out, err := run(ctx, cmd, args)
	if err != nil {
		return time.Time{}, err
	}
	seconds, err := strconv.ParseInt(strings.TrimPrefix(out, prefix), 10, 64)
	if err != nil || !strings.HasPrefix(out, prefix) {
		return time.Time{}, core.Internalf(err, "reading the instant git resolved a --%s bound to", bound)
	}
	return time.Unix(seconds, 0).UTC(), nil
}

// run executes a prepared git command and returns trimmed stdout. args are the
// command's arguments after the common prefix, for the error's context.
func run(ctx context.Context, cmd *exec.Cmd, args []string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx != nil && ctx.Err() != nil {
			return "", ctx.Err()
		}
		// A failed invocation is an internal error: a caller that wants to
		// refuse a repository decides that from what the invocation told it,
		// not from the invocation failing. Classifying it here is what keeps
		// git's stderr — repository-influenced, and free to name paths — inside
		// a diagnostic, because an internal error's artifact rendering is one
		// fixed sentence (ADR-0045, ADR-0067 clause 2).
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", core.Internalf(err, "running git %s", strings.Join(args, " "))
		}
		return "", core.Internalf(errors.New(msg), "running git %s", strings.Join(args, " "))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Lines runs git and splits stdout into non-empty lines.
func Lines(repoPath string, args ...string) ([]string, error) {
	return LinesContext(context.Background(), repoPath, args...)
}

// LinesContext runs git with cancellation support and splits stdout into
// non-empty lines.
func LinesContext(ctx context.Context, repoPath string, args ...string) ([]string, error) {
	out, err := RunContext(ctx, repoPath, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	raw := strings.Split(out, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if line = strings.TrimRight(line, "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
