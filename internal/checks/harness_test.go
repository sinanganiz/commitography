package checks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/git"
)

// report records a checker failure. Every failure in this package goes through
// report or fatal, so that each message begins with the governing record
// (ADR-0055 clause 3).
func report(t *testing.T, record int, format string, args ...any) {
	t.Helper()
	t.Error(message(record, format, args...))
}

// fatal records a checker failure and stops the checker.
func fatal(t *testing.T, record int, format string, args ...any) {
	t.Helper()
	t.Fatal(message(record, format, args...))
}

func message(record int, format string, args ...any) string {
	return fmt.Sprintf("ADR-%04d: ", record) + fmt.Sprintf(format, args...)
}

// TestHarnessMessageFormat keeps the message shape from drifting.
func TestHarnessMessageFormat(t *testing.T) {
	t.Parallel()
	shape := regexp.MustCompile(`^ADR-[0-9]{4}: `)
	for _, record := range []int{1, 49, 9999} {
		if got := message(record, "%s", "example"); !shape.MatchString(got) {
			report(t, 55, "failure message %q does not begin with the record", got)
		}
	}
}

// repository is the set of tracked files checkers may read.
type repository struct {
	root    string
	tracked []string // slash-separated, relative to root, in index order
}

func openRepository(t *testing.T) repository {
	t.Helper()
	ctx := context.Background()
	root, err := git.Output(ctx, git.At("", "rev-parse", "--show-toplevel"))
	if err != nil {
		fatal(t, 55, "cannot locate the repository root: %v", err)
	}
	tracked, err := git.Records(ctx, git.At(root, "ls-files", "-z").Pathspecs())
	if err != nil {
		fatal(t, 55, "cannot list tracked files: %v", err)
	}
	return repository{root: root, tracked: tracked}
}

func (r repository) isTracked(path string) bool {
	for _, p := range r.tracked {
		if p == path {
			return true
		}
	}
	return false
}

// read returns the working copy of a tracked file. A failure is attributed to
// the record the calling checker enforces.
func (r repository) read(t *testing.T, record int, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if err != nil {
		fatal(t, record, "cannot read tracked file %s: %v", path, err)
	}
	return string(data)
}

// isText reports whether content looks like text, using the same heuristic as
// git: no NUL byte in the first 8000 bytes.
func isText(content string) bool {
	head := content
	if len(head) > 8000 {
		head = head[:8000]
	}
	return !bytes.Contains([]byte(head), []byte{0})
}

// lines splits content into lines without their terminators.
func lines(content string) []string {
	return strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
}
