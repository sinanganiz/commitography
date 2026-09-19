package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// osFiles is the operating system's filesystem. The configuration package may
// import nothing from the tree, its tests included, so it cannot use
// core.SystemFilesystem.
type osFiles struct{}

func (osFiles) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }
func (osFiles) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }

// load reads configuration with warnings discarded.
func load(explicitPath, repoPath string) (Settings, error) {
	return Load(osFiles{}, explicitPath, repoPath, func(string, ...any) {})
}

func TestLoadWithoutFileYieldsDefaults(t *testing.T) {
	t.Parallel()
	s, err := load("", t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(s.Analysis, Default()) {
		t.Errorf("Load without a file did not return Default()")
	}
	if !reflect.DeepEqual(s.Operational, DefaultOperational()) {
		t.Errorf("Load without a file did not return DefaultOperational()")
	}
	if len(s.Analysis.ExcludePaths) != len(DefaultExcludePaths()) {
		t.Errorf("ExcludePaths has %d entries, want %d", len(s.Analysis.ExcludePaths), len(DefaultExcludePaths()))
	}
	if len(s.Analysis.ExcludeAuthors) != len(DefaultExcludeAuthors()) {
		t.Errorf("ExcludeAuthors has %d entries, want %d", len(s.Analysis.ExcludeAuthors), len(DefaultExcludeAuthors()))
	}
}

func TestLoadAppendsToDefaultLists(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, `
exclude_paths:
  - "generated/**"
exclude_authors:
  - "release-robot"
`)
	s, err := load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg := s.Analysis
	if len(cfg.ExcludePaths) != len(DefaultExcludePaths())+1 {
		t.Errorf("ExcludePaths has %d entries, want %d", len(cfg.ExcludePaths), len(DefaultExcludePaths())+1)
	}
	if cfg.ExcludePaths[len(cfg.ExcludePaths)-1] != "generated/**" {
		t.Error("user pattern was not appended to the defaults")
	}
	if cfg.ExcludeAuthors[len(cfg.ExcludeAuthors)-1] != "release-robot" {
		t.Error("user author was not appended to the defaults")
	}
}

func TestExplicitEmptyListClearsDefaults(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, "exclude_paths: []\n")
	s, err := load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Analysis.ExcludePaths) != 0 {
		t.Errorf("exclude_paths: [] left %d exclusions in place", len(s.Analysis.ExcludePaths))
	}
	// The other list must be untouched by the empty one.
	if len(s.Analysis.ExcludeAuthors) != len(DefaultExcludeAuthors()) {
		t.Error("clearing exclude_paths also affected exclude_authors")
	}
}

func TestScalarOverrides(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, `
outlier_threshold_lines: 500
count_merges: true
date_source: committer
use_mailmap: false
anonymize: true
output_dir: ./report
identities:
  - name: Ada Lovelace
    emails:
      - ada@example.com
      - ada.lovelace@corp.example.com
`)
	s, err := load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg := s.Analysis
	if cfg.OutlierThresholdLines != 500 || !cfg.CountMerges ||
		cfg.DateSource != DateSourceCommitter || cfg.UseMailmap || !cfg.Anonymize {
		t.Errorf("analysis overrides not applied: %+v", cfg)
	}
	if s.Operational.OutputDir != "./report" {
		t.Errorf("output_dir = %q, want ./report on the operational plane", s.Operational.OutputDir)
	}
	if len(cfg.Identities) != 1 || len(cfg.Identities[0].Emails) != 2 {
		t.Errorf("identities not parsed: %+v", cfg.Identities)
	}
}

func TestExplicitPathReplacesRepositoryConfig(t *testing.T) {
	t.Parallel()
	repoDir := writeConfig(t, "outlier_threshold_lines: 111\ncount_merges: true\n")
	otherDir := writeConfig(t, "outlier_threshold_lines: 222\n")
	explicit := filepath.Join(otherDir, FileName)

	s, err := load(explicit, repoDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Analysis.OutlierThresholdLines != 222 {
		t.Errorf("threshold = %d, want 222 from the explicit file", s.Analysis.OutlierThresholdLines)
	}
	// Replacing is not merging: a key only the repository file sets is not
	// carried over.
	if s.Analysis.CountMerges {
		t.Error("count_merges from the repository file survived an explicit file that does not set it")
	}
}

func TestInvalidDateSourceIsAnError(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, "date_source: invalid\n")
	_, err := load("", dir)
	if err == nil {
		t.Fatal("expected an error for an invalid date_source")
	}
	if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), "date_source") {
		t.Errorf("error should name the offending key and value, got %q", err.Error())
	}
}

func TestInvalidThresholdIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := load("", writeConfig(t, "outlier_threshold_lines: 0\n")); err == nil {
		t.Error("expected an error for outlier_threshold_lines: 0")
	}
}

// hash_emails and theme were removed: the first offered a choice ADR-0033
// removed, and the second was validated and never read. Supplying either is an
// unknown key, which warns and never fails (ADR-0026 clause 7).
func TestRemovedKeysWarnRatherThanFail(t *testing.T) {
	t.Parallel()
	for _, body := range []string{"hash_emails: false\n", "theme: neon\n"} {
		var warnings []string
		s, err := Load(osFiles{}, "", writeConfig(t, body), func(format string, args ...any) {
			warnings = append(warnings, format)
		})
		if err != nil {
			t.Errorf("%q failed the run: %v", strings.TrimSpace(body), err)
		}
		if len(warnings) != 1 {
			t.Errorf("%q raised %d warnings, want 1", strings.TrimSpace(body), len(warnings))
		}
		if !reflect.DeepEqual(s.Analysis, Default()) {
			t.Errorf("%q changed the analysis plane", strings.TrimSpace(body))
		}
	}
}

func TestOperationalKeysNameEveryOperationalValue(t *testing.T) {
	t.Parallel()
	if got, want := len(OperationalKeys()), reflect.TypeOf(Operational{}).NumField(); got != want {
		t.Errorf("OperationalKeys names %d values, and Operational has %d", got, want)
	}
	for _, key := range OperationalKeys() {
		for _, analysis := range AnalysisKeys() {
			if key == analysis {
				t.Errorf("%s is named on both planes", key)
			}
		}
	}
}

func TestUnknownKeyWarnsButDoesNotFail(t *testing.T) {
	t.Parallel()
	var warnings []string
	dir := writeConfig(t, "excludePaths:\n  - foo\ncount_merges: true\n")

	s, err := Load(osFiles{}, "", dir, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !s.Analysis.CountMerges {
		t.Error("recognized keys should still be applied alongside an unknown one")
	}
	if len(warnings) != 1 {
		t.Errorf("got %d warnings, want 1: %v", len(warnings), warnings)
	}
}

func TestLoadWithWarnUsesCallLocalSink(t *testing.T) {
	t.Parallel()
	dir := writeConfig(t, "unknown_key: true\n")
	var first, second []string
	if _, err := Load(osFiles{}, "", dir, func(format string, args ...any) {
		first = append(first, format)
	}); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if _, err := Load(osFiles{}, "", dir, func(format string, args ...any) {
		second = append(second, format)
	}); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("call-local warnings = %d and %d, want one each", len(first), len(second))
	}
}

func TestIsBotIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, email string
		want        bool
	}{
		{"dependabot[bot]", "49699333+dependabot[bot]@users.noreply.github.com", true},
		{"renovate[bot]", "renovate@example.com", true},
		{"Ada Lovelace", "ada@example.com", false},
		{"Ada Lovelace", "ada@users.noreply.github.com", false},
		{"botman", "botman@example.com", false},
	}
	for _, c := range cases {
		if got := IsBotIdentity(c.name, c.email); got != c.want {
			t.Errorf("IsBotIdentity(%q, %q) = %v, want %v", c.name, c.email, got, c.want)
		}
	}
}
