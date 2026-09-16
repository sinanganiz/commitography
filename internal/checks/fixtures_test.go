package checks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/gitcmd"
)

const fixtureHashes = "testdata/fixture-hashes.txt"

// TestFixtureDeterminism enforces ADR-0019 clause 1: two generations of the
// fixture set produce identical commit hashes, and those hashes equal the
// committed list, so every platform that runs this checker is compared with
// every other.
//
// The generator is a shell script and this package may not execute anything
// but git (ADR-0047), so `make fixture-determinism` generates the two sets
// and passes their roots in COMMITOGRAPHY_FIXTURES_FIRST and
// COMMITOGRAPHY_FIXTURES_SECOND. The fast gate skips this checker; it belongs
// to the full gate (ADR-0057 clause 5).
//
// With COMMITOGRAPHY_UPDATE_FIXTURE_HASHES=1 the committed list is rewritten
// from the first generation instead of compared.
func TestFixtureDeterminism(t *testing.T) {
	first := os.Getenv("COMMITOGRAPHY_FIXTURES_FIRST")
	second := os.Getenv("COMMITOGRAPHY_FIXTURES_SECOND")
	if first == "" || second == "" {
		fatal(t, 64, "COMMITOGRAPHY_FIXTURES_FIRST and COMMITOGRAPHY_FIXTURES_SECOND are unset; "+
			"run this checker through `make fixture-determinism`, which generates both")
	}
	repo := openRepository(t)

	a := fixtureManifest(t, first)
	b := fixtureManifest(t, second)
	if a != b {
		report(t, 19, "two generations produced different commit hashes:\n%s", lineDiff(a, b))
	}

	path := filepath.Join(repo.root, filepath.FromSlash(fixtureHashes))
	if os.Getenv("COMMITOGRAPHY_UPDATE_FIXTURE_HASHES") == "1" {
		if err := os.WriteFile(path, []byte(a), 0o644); err != nil {
			fatal(t, 19, "cannot write %s: %v", fixtureHashes, err)
		}
		return
	}
	if !repo.isTracked(fixtureHashes) {
		fatal(t, 19, "%s is not tracked; the hashes cannot be compared across platforms", fixtureHashes)
	}
	want := strings.ReplaceAll(repo.read(t, 19, fixtureHashes), "\r\n", "\n")
	if a != want {
		report(t, 19, "generated fixtures differ from %s (- committed, + generated):\n%s\n"+
			"A fixture change that is intended updates the list in the same commit and states why "+
			"(COMMITOGRAPHY_UPDATE_FIXTURE_HASHES=1 make fixture-determinism).", fixtureHashes, lineDiff(want, a))
	}
}

// fixtureManifest describes every fixture under root by its HEAD, every ref
// and its reachable commit count. Ref hashes chain the whole history, and the
// count distinguishes a shallow clone from its source.
func fixtureManifest(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		fatal(t, 19, "cannot read fixture root %s: %v", root, err)
	}
	ctx := context.Background()
	var b strings.Builder
	b.WriteString("# Commit hashes of the generated fixtures (ADR-0019 clause 1).\n")
	b.WriteString("# Written by COMMITOGRAPHY_UPDATE_FIXTURE_HASHES=1 make fixture-determinism.\n")
	fixtures := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		fixtures++
		name := e.Name()
		dir := filepath.Join(root, name)
		// As in the generator: a deep checkout otherwise exceeds the Windows
		// path length limit when git reads objects. Other platforms ignore it.
		git := func(args ...string) []string {
			return append([]string{"-c", "core.longpaths=true"}, args...)
		}
		head, err := gitcmd.RunContext(ctx, dir, git("symbolic-ref", "HEAD")...)
		if err != nil {
			fatal(t, 19, "fixture %s: %v", name, err)
		}
		b.WriteString(name + " HEAD " + head + "\n")
		refs, err := gitcmd.LinesContext(ctx, dir, git("for-each-ref", "--format=%(refname) %(objectname)")...)
		if err != nil {
			fatal(t, 19, "fixture %s: %v", name, err)
		}
		for _, ref := range refs {
			b.WriteString(name + " " + ref + "\n")
		}
		count, err := gitcmd.RunContext(ctx, dir, git("rev-list", "--count", "--all")...)
		if err != nil {
			fatal(t, 19, "fixture %s: %v", name, err)
		}
		b.WriteString(name + " commits " + count + "\n")
	}
	if fixtures == 0 {
		fatal(t, 19, "no fixtures were generated under %s", root)
	}
	return b.String()
}

// lineDiff lists the lines present in only one of want and got.
func lineDiff(want, got string) string {
	in := func(s string) map[string]bool {
		set := map[string]bool{}
		for _, l := range lines(s) {
			set[l] = true
		}
		return set
	}
	w, g := in(want), in(got)
	var out []string
	for _, l := range lines(want) {
		if !g[l] {
			out = append(out, "- "+l)
		}
	}
	for _, l := range lines(got) {
		if !w[l] {
			out = append(out, "+ "+l)
		}
	}
	if len(out) == 0 {
		// Same lines, different order or multiplicity.
		return "--- want\n" + want + "--- got\n" + got
	}
	return strings.Join(out, "\n")
}
