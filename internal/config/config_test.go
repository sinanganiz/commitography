package config

import (
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

func captureWarnings(t *testing.T) *[]string {
	t.Helper()
	var got []string
	original := Warn
	Warn = func(format string, args ...any) {
		got = append(got, format)
	}
	t.Cleanup(func() { Warn = original })
	return &got
}

func TestLoadWithoutFileYieldsDefaults(t *testing.T) {
	cfg, err := Load("", t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("Load without a file did not return Default()")
	}
	if len(cfg.ExcludePaths) != len(DefaultExcludePaths) {
		t.Errorf("ExcludePaths has %d entries, want %d", len(cfg.ExcludePaths), len(DefaultExcludePaths))
	}
	if len(cfg.ExcludeAuthors) != len(DefaultExcludeAuthors) {
		t.Errorf("ExcludeAuthors has %d entries, want %d", len(cfg.ExcludeAuthors), len(DefaultExcludeAuthors))
	}
}

func TestLoadAppendsToDefaultLists(t *testing.T) {
	dir := writeConfig(t, `
exclude_paths:
  - "generated/**"
exclude_authors:
  - "release-robot"
`)
	cfg, err := Load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.ExcludePaths) != len(DefaultExcludePaths)+1 {
		t.Errorf("ExcludePaths has %d entries, want %d", len(cfg.ExcludePaths), len(DefaultExcludePaths)+1)
	}
	if cfg.ExcludePaths[len(cfg.ExcludePaths)-1] != "generated/**" {
		t.Error("user pattern was not appended to the defaults")
	}
	if cfg.ExcludeAuthors[len(cfg.ExcludeAuthors)-1] != "release-robot" {
		t.Error("user author was not appended to the defaults")
	}
}

func TestExplicitEmptyListClearsDefaults(t *testing.T) {
	dir := writeConfig(t, "exclude_paths: []\n")
	cfg, err := Load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.ExcludePaths) != 0 {
		t.Errorf("exclude_paths: [] left %d exclusions in place", len(cfg.ExcludePaths))
	}
	// The other list must be untouched by the empty one.
	if len(cfg.ExcludeAuthors) != len(DefaultExcludeAuthors) {
		t.Error("clearing exclude_paths also affected exclude_authors")
	}
}

func TestScalarOverrides(t *testing.T) {
	dir := writeConfig(t, `
outlier_threshold_lines: 500
count_merges: true
date_source: committer
use_mailmap: false
anonymize: true
hash_emails: false
output_dir: ./report
identities:
  - name: Ada Lovelace
    emails:
      - ada@example.com
      - ada.lovelace@corp.example.com
`)
	cfg, err := Load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OutlierThresholdLines != 500 || !cfg.CountMerges ||
		cfg.DateSource != DateSourceCommitter || cfg.UseMailmap ||
		!cfg.Anonymize || cfg.HashEmails || cfg.OutputDir != "./report" {
		t.Errorf("scalar overrides not applied: %+v", cfg)
	}
	if len(cfg.Identities) != 1 || len(cfg.Identities[0].Emails) != 2 {
		t.Errorf("identities not parsed: %+v", cfg.Identities)
	}
}

func TestExplicitPathReplacesRepositoryConfig(t *testing.T) {
	repoDir := writeConfig(t, "outlier_threshold_lines: 111\n")
	otherDir := writeConfig(t, "outlier_threshold_lines: 222\n")
	explicit := filepath.Join(otherDir, FileName)

	cfg, err := Load(explicit, repoDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OutlierThresholdLines != 222 {
		t.Errorf("threshold = %d, want 222 from the explicit file", cfg.OutlierThresholdLines)
	}
}

func TestInvalidDateSourceIsAnError(t *testing.T) {
	dir := writeConfig(t, "date_source: invalid\n")
	_, err := Load("", dir)
	if err == nil {
		t.Fatal("expected an error for an invalid date_source")
	}
	if !strings.Contains(err.Error(), "invalid") || !strings.Contains(err.Error(), "date_source") {
		t.Errorf("error should name the offending key and value, got %q", err.Error())
	}
}

func TestInvalidThresholdAndThemeAreErrors(t *testing.T) {
	for _, body := range []string{"outlier_threshold_lines: 0\n", "theme: neon\n"} {
		if _, err := Load("", writeConfig(t, body)); err == nil {
			t.Errorf("expected an error for %q", strings.TrimSpace(body))
		}
	}
}

func TestUnknownKeyWarnsButDoesNotFail(t *testing.T) {
	warnings := captureWarnings(t)
	dir := writeConfig(t, "excludePaths:\n  - foo\ncount_merges: true\n")

	cfg, err := Load("", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.CountMerges {
		t.Error("recognized keys should still be applied alongside an unknown one")
	}
	if len(*warnings) != 1 {
		t.Errorf("got %d warnings, want 1: %v", len(*warnings), *warnings)
	}
}

func TestIsBotIdentity(t *testing.T) {
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
