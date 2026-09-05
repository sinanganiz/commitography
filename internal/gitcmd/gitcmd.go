// Package gitcmd is the single place commitography shells out to git. Every
// invocation goes through here so that flags like core.quotePath are applied
// uniformly and stderr is always turned into a useful error.
package gitcmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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

// Command builds an *exec.Cmd for a git invocation against repoPath.
func Command(repoPath string, args ...string) *exec.Cmd {
	return exec.Command("git", Args(repoPath, args...)...)
}

// Run executes git and returns trimmed stdout, wrapping git's own stderr in the
// error so the caller can surface something actionable.
func Run(repoPath string, args ...string) (string, error) {
	cmd := Command(repoPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Lines runs git and splits stdout into non-empty lines.
func Lines(repoPath string, args ...string) ([]string, error) {
	out, err := Run(repoPath, args...)
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
