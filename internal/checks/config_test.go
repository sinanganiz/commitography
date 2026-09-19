package checks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
)

// exampleConfiguration is the configuration file the repository ships as its
// own and as the example users copy.
const exampleConfiguration = config.FileName

// exampleProblems loads a configuration file's content and returns what makes
// it untrue to its own header: that as written it changes nothing, and that
// the list keys it shows add to the built-in lists rather than disabling them.
// Every key it sets must be known, so a removed key such as hash_emails or
// theme cannot linger in it.
func exampleProblems(t *testing.T, content string) []string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(content), 0o600); err != nil {
		fatal(t, 26, "writing the example configuration: %v", err)
	}
	var warnings []string
	settings, err := config.Load(core.SystemFilesystem(), "", dir, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	if err != nil {
		return []string{"it does not load: " + err.Error()}
	}
	var out []string
	if len(warnings) != 0 {
		out = append(out, "it sets a key the configuration does not know")
	}
	if !reflect.DeepEqual(settings.Analysis.ExcludeAuthors, config.DefaultExcludeAuthors()) {
		out = append(out, "it changes the built-in author exclusions")
	}
	if !reflect.DeepEqual(settings.Analysis.ExcludePaths, config.DefaultExcludePaths()) {
		out = append(out, "it changes the built-in path exclusions")
	}
	if !reflect.DeepEqual(settings.Analysis, config.Default()) {
		out = append(out, "it changes the analysis plane, while its header says it changes nothing")
	}
	return out
}

// TestConfigExampleChangesNothing holds the example configuration to its
// header (WP-0010 clause 9a): as written it resolves to the built-in values,
// both exclusion lists included, and sets no key that does not exist.
func TestConfigExampleChangesNothing(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	if !repo.isTracked(exampleConfiguration) {
		fatal(t, 26, "%s is not tracked", exampleConfiguration)
	}
	for _, problem := range exampleProblems(t, repo.read(t, 26, exampleConfiguration)) {
		report(t, 26, "%s: %s", exampleConfiguration, problem)
	}
}

// TestConfigExampleRejectsADisablingList is the failure demonstration ADR-0064
// clause 6 requires: the empty-list form the example used to carry, and a
// removed key, each fail the check above.
func TestConfigExampleRejectsADisablingList(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"an empty exclude_paths":   "exclude_paths: []\n",
		"an empty exclude_authors": "exclude_authors: []\n",
		"the removed hash_emails":  "hash_emails: true\n",
		"the removed theme":        "theme: default\n",
	} {
		if len(exampleProblems(t, content)) == 0 {
			report(t, 64, "the example check accepted %s, so it cannot catch one", name)
		}
	}
	if found := exampleProblems(t, "count_merges: false\n"); len(found) != 0 {
		report(t, 26, "the example check refused a file that sets a default: %v", found)
	}
}
