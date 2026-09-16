// Process execution in this file is permitted by ADR-0065 clause 3: it runs
// the built binary with a fixed argument vector, no shell and a timeout
// (ADR-0065 clause 4).

package checks

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestReproducibleBuild enforces ADR-0049 clause 6 and ADR-0063 table 2: two
// builds of one commit are byte-identical. The make target that runs it
// produces the two builds, in the directories named by
// COMMITOGRAPHY_REPRODUCIBLE_FIRST and _SECOND; this checker compares every
// commitography binary in them. It also runs each binary with --version, so a
// linker flag naming a variable that no longer exists, which the linker
// ignores silently, shows up as a default version (ADR-0061).
func TestReproducibleBuild(t *testing.T) {
	first := os.Getenv("COMMITOGRAPHY_REPRODUCIBLE_FIRST")
	second := os.Getenv("COMMITOGRAPHY_REPRODUCIBLE_SECOND")
	if first == "" || second == "" {
		fatal(t, 64, "COMMITOGRAPHY_REPRODUCIBLE_FIRST and COMMITOGRAPHY_REPRODUCIBLE_SECOND must name two build directories; "+
			"run `make reproducible-build` or `make reproducible-release`")
	}
	a, b := binariesUnder(t, first), binariesUnder(t, second)
	if len(a) == 0 {
		fatal(t, 64, "no commitography binary was built under %s", first)
	}

	for rel, path := range a {
		other, ok := b[rel]
		if !ok {
			report(t, 49, "%s was built the first time but not the second", rel)
			continue
		}
		one, two := readBinary(t, path), readBinary(t, other)
		if !bytes.Equal(one, two) {
			report(t, 49, "two builds of one commit differ: %s is %d and %d bytes and first differs at byte %d",
				rel, len(one), len(two), firstDifference(one, two))
		}
	}
	for rel := range b {
		if _, ok := a[rel]; !ok {
			report(t, 49, "%s was built the second time but not the first", rel)
		}
	}

	for rel, path := range a {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		out, err := exec.CommandContext(ctx, path, "--version").Output()
		cancel()
		if err != nil {
			report(t, 61, "%s --version failed: %v", rel, err)
			continue
		}
		line := strings.TrimSpace(string(out))
		if strings.HasPrefix(line, "commitography dev ") || strings.Contains(line, "unknown") {
			report(t, 61, "%s reports %q: a linker flag did not reach its variable, so a default survived", rel, line)
		}
	}
}

// binariesUnder returns the commitography binaries below root, keyed by their
// slash-separated path relative to root.
func binariesUnder(t *testing.T, root string) map[string]string {
	t.Helper()
	found := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), "cache-") {
				return filepath.SkipDir
			}
			return nil
		}
		if name := d.Name(); name == "commitography" || name == "commitography.exe" {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			found[filepath.ToSlash(rel)] = path
		}
		return nil
	})
	if err != nil {
		fatal(t, 64, "cannot read build directory %s: %v", root, err)
	}
	return found
}

func readBinary(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		fatal(t, 49, "cannot read %s: %v", path, err)
	}
	return data
}

func firstDifference(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}
