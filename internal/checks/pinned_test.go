// Process execution in this file is permitted by ADR-0065 clause 3: it runs
// the built binary with a fixed argument vector, no shell and a timeout
// (ADR-0065 clause 4).

package checks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// The pinned configuration checker (ADR-0071 clause 5): an analysis run under
// a deliberately hostile global git configuration produces the report it
// produces under none. The operator's global configuration is still read,
// because ADR-0016 delegates credentials to it, so every key in it that can
// move the output the analysis parses has to be overridden on the invocation.
//
// The analysis is run as the built binary with its home directory pointed at
// a directory holding the configuration, because that is how an operator's
// configuration reaches git: through the environment the product inherits,
// which no in-process test can change without changing it for every other
// test running beside it.

// pinnedConfigFixtures are the fixtures the comparison runs on: renames, for
// rename detection; a root commit whose files the hostile configuration would
// hide; a mailmap; and names that path quoting would escape.
func pinnedConfigFixtures() []string {
	return []string{"renames-and-copied-block", "basic", "mailmap", "binary"}
}

// hostileGlobalConfiguration returns a global configuration setting every key
// ADR-0071 clause 3 names, and every key since found to move parsed output, to
// a value other than git's default. The files it points at are written into
// home beside it.
func hostileGlobalConfiguration(t *testing.T, home string) string {
	t.Helper()
	write := func(name, content string) string {
		path := filepath.Join(home, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			fatal(t, 71, "writing the hostile %s: %v", name, err)
		}
		return filepath.ToSlash(path)
	}
	// Every file is binary to git, so every line count would vanish.
	attributes := write("attributes", "* binary\n")
	// The file entries of a commit would arrive in another order.
	order := write("orderfile", "*.txt\n*.md\n")
	// Every author of the fixtures would become somebody else.
	mailmap := write("mailmap", strings.Join([]string{
		"Someone Else <else@example.com> <ada@example.com>",
		"Someone Else <else@example.com> <grace@example.com>",
		"Someone Else <else@example.com> <alan@example.com>",
		"",
	}, "\n"))
	return strings.Join([]string{
		"[diff]",
		"\trenames = false",
		"\trenameLimit = 1",
		"\talgorithm = histogram",
		"\torderFile = " + order,
		"[core]",
		"\tquotePath = true",
		"\tattributesFile = " + attributes,
		"[log]",
		"\tshowRoot = false",
		"\tshowSignature = true",
		"[i18n]",
		"\tlogOutputEncoding = ISO-8859-1",
		"[mailmap]",
		"\tfile = " + mailmap,
		"\tblob = HEAD:README.md",
		"",
	}, "\n")
}

// homeWith returns a home directory whose global git configuration is
// content, and the environment an analysis run with it gets. git reads the
// global configuration from $HOME, or from $XDG_CONFIG_HOME, so both point
// inside the directory.
func homeWith(t *testing.T, content func(home string) string) (string, []string) {
	t.Helper()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(content(home)), 0o600); err != nil {
		fatal(t, 71, "writing a global git configuration: %v", err)
	}
	env := append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, "xdg"))
	return home, env
}

// analyseWithEnvironment runs the built binary on a fixture with env and
// returns the report it writes.
func analyseWithEnvironment(t *testing.T, repo repository, binary, dir string, env []string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), analysisTimeout)
	defer cancel()
	out := t.TempDir()
	cmd := exec.CommandContext(ctx, binary, dir, "--json", "--quiet", "--output", out)
	cmd.Env = env
	cmd.Dir = repo.root
	if combined, err := cmd.CombinedOutput(); err != nil {
		fatal(t, 71, "analysing %s: %v\n%s", filepath.Base(dir), err, combined)
	}
	data, err := os.ReadFile(filepath.Join(out, render.ReportFile))
	if err != nil {
		fatal(t, 71, "the analysis of %s wrote no report: %v", filepath.Base(dir), err)
	}
	return string(data)
}

// TestPinnedConfigSurvivesAHostileGlobalConfiguration is ADR-0071 clause 5.
func TestPinnedConfigSurvivesAHostileGlobalConfiguration(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	binary := buildBinary(t, repo, "./cmd/commitography")

	_, cleanEnv := homeWith(t, func(string) string { return "" })
	hostileHome, hostileEnv := homeWith(t, func(home string) string { return hostileGlobalConfiguration(t, home) })

	// The comparison proves nothing unless git actually reads the hostile
	// file, so that is established first (ADR-0064 clause 1).
	seen, err := git.Output(context.Background(), git.At("", "config", "--global", "--get", "diff.renames").
		WithEnv("HOME="+hostileHome, "XDG_CONFIG_HOME="+filepath.Join(hostileHome, "xdg")))
	if err != nil || strings.TrimSpace(seen) != "false" {
		fatal(t, 64, "git does not read the hostile global configuration (diff.renames = %q, %v), so the "+
			"comparison would prove nothing", seen, err)
	}

	for _, fixture := range pinnedConfigFixtures() {
		dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
		if _, err := os.Stat(dir); err != nil {
			fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
		}
		clean := analyseWithEnvironment(t, repo, binary, dir, cleanEnv)
		hostile := analyseWithEnvironment(t, repo, binary, dir, hostileEnv)
		if diff := sameInputDifference(clean, hostile); diff != "" {
			report(t, 71, "the %s fixture produced a different report under a hostile global git configuration; "+
				"a key that moves the output is not pinned (- clean, + hostile):\n%s", fixture, diff)
		}
	}
}
