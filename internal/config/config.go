// Package config resolves commitography's effective settings from built-in
// defaults, an optional YAML file, and command-line overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the configuration file looked for in the analyzed repository root.
const FileName = ".commitography.yml"

// Warn receives non-fatal configuration diagnostics. It is a variable so tests
// can capture warnings instead of writing to stderr.
var Warn = func(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
}

// Identity groups several git email addresses under one person.
type Identity struct {
	Name   string   `yaml:"name"`
	Emails []string `yaml:"emails"`
}

// Config is the effective configuration for one analysis run.
type Config struct {
	Identities            []Identity `yaml:"identities"`
	ExcludeAuthors        []string   `yaml:"exclude_authors"`
	ExcludePaths          []string   `yaml:"exclude_paths"`
	OutlierThresholdLines int        `yaml:"outlier_threshold_lines"`
	CountMerges           bool       `yaml:"count_merges"`
	DateSource            string     `yaml:"date_source"`
	UseMailmap            bool       `yaml:"use_mailmap"`
	Anonymize             bool       `yaml:"anonymize"`
	HashEmails            bool       `yaml:"hash_emails"`
	OutputDir             string     `yaml:"output_dir"`
	Theme                 string     `yaml:"theme"`
}

// Date sources.
const (
	DateSourceAuthor    = "author"
	DateSourceCommitter = "committer"
)

// DefaultExcludeAuthors are the automation accounts excluded unless the user
// replaces the list with an explicit empty one.
var DefaultExcludeAuthors = []string{
	"dependabot[bot]",
	"renovate[bot]",
	"github-actions[bot]",
	"gitlab-bot",
	"imgbot[bot]",
	"allcontributors[bot]",
	"snyk-bot",
	"greenkeeper[bot]",
	"semantic-release-bot",
}

// DefaultExcludePaths are the generated, vendored and bundled paths that would
// otherwise dominate every line-based metric.
//
// Patterns are matched literally with doublestar.Match, as Task 3.2 requires,
// which anchors an unprefixed pattern to the repository root. A monorepo keeps
// its lockfiles and dependency trees one level down — web/pnpm-lock.yaml,
// packages/api/node_modules/ — so each root-anchored default is paired with a
// `**/` form that reaches any depth. Without the pair, exit criterion 4 held
// only for single-package repositories. The matching rule is unchanged; only
// this list is.
var DefaultExcludePaths = []string{
	"**/*.lock",
	"package-lock.json",
	"**/package-lock.json",
	"yarn.lock",
	"**/yarn.lock",
	"pnpm-lock.yaml",
	"**/pnpm-lock.yaml",
	"Gemfile.lock",
	"**/Gemfile.lock",
	"composer.lock",
	"**/composer.lock",
	"go.sum",
	"**/go.sum",
	"Cargo.lock",
	"**/Cargo.lock",
	"poetry.lock",
	"**/poetry.lock",
	"vendor/**",
	"**/vendor/**",
	"node_modules/**",
	"**/node_modules/**",
	"third_party/**",
	"**/third_party/**",
	"Pods/**",
	"**/Pods/**",
	"dist/**",
	"**/dist/**",
	"build/**",
	"**/build/**",
	"out/**",
	"**/out/**",
	"target/**",
	"**/target/**",
	"**/*.min.js",
	"**/*.min.css",
	"**/*.map",
	"**/*.generated.*",
	"**/*.pb.go",
	"**/*_pb2.py",
	"**/migrations/**",
	"**/*.snap",
}

// botNameRe matches the `[bot]` suffix git hosts append to automation accounts.
var botNameRe = regexp.MustCompile(`\[bot\]$`)

// noreplySuffix is the address git hosts use for accounts without a public email.
const noreplySuffix = "@users.noreply.github.com"

// IsBotIdentity reports whether a name/email pair belongs to an automation
// account by pattern, independent of the configured exclusion list.
func IsBotIdentity(name, email string) bool {
	name = strings.TrimSpace(name)
	if botNameRe.MatchString(name) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(email)), noreplySuffix) &&
		botNameRe.MatchString(name)
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Identities:            nil,
		ExcludeAuthors:        append([]string(nil), DefaultExcludeAuthors...),
		ExcludePaths:          append([]string(nil), DefaultExcludePaths...),
		OutlierThresholdLines: 10000,
		CountMerges:           false,
		DateSource:            DateSourceAuthor,
		UseMailmap:            true,
		Anonymize:             false,
		HashEmails:            true,
		OutputDir:             "./out",
		Theme:                 "default",
	}
}

// fileConfig mirrors Config with pointer fields, so that a key which is absent
// from the file is distinguishable from one set to an empty value. That
// distinction is what makes `exclude_paths: []` mean "drop the defaults" while
// omitting the key means "keep them".
type fileConfig struct {
	Identities            *[]Identity `yaml:"identities"`
	ExcludeAuthors        *[]string   `yaml:"exclude_authors"`
	ExcludePaths          *[]string   `yaml:"exclude_paths"`
	OutlierThresholdLines *int        `yaml:"outlier_threshold_lines"`
	CountMerges           *bool       `yaml:"count_merges"`
	DateSource            *string     `yaml:"date_source"`
	UseMailmap            *bool       `yaml:"use_mailmap"`
	Anonymize             *bool       `yaml:"anonymize"`
	HashEmails            *bool       `yaml:"hash_emails"`
	OutputDir             *string     `yaml:"output_dir"`
	Theme                 *string     `yaml:"theme"`
}

var knownKeys = map[string]bool{
	"identities":              true,
	"exclude_authors":         true,
	"exclude_paths":           true,
	"outlier_threshold_lines": true,
	"count_merges":            true,
	"date_source":             true,
	"use_mailmap":             true,
	"anonymize":               true,
	"hash_emails":             true,
	"output_dir":              true,
	"theme":                   true,
}

// Load resolves configuration from defaults, file, and flag overrides.
//
// Resolution order, later winning: built-in defaults, the repository's own
// .commitography.yml, then the file named by explicitPath which replaces the
// repository-local file rather than merging with it. Command-line flags are
// applied by the caller afterwards.
func Load(explicitPath string, repoPath string) (Config, error) {
	cfg := Default()

	path := explicitPath
	if path == "" {
		candidate := filepath.Join(repoPath, FileName)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		}
	}
	if path == "" {
		return cfg, cfg.Validate()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading %s: %w", path, err)
	}

	warnUnknownKeys(path, data)

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", path, err)
	}
	fc.applyTo(&cfg)

	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// applyTo folds file values over the defaults. List keys append to the
// built-ins; setting one to an explicit empty list clears them.
func (fc fileConfig) applyTo(cfg *Config) {
	if fc.Identities != nil {
		cfg.Identities = append(cfg.Identities, *fc.Identities...)
	}
	if fc.ExcludeAuthors != nil {
		if len(*fc.ExcludeAuthors) == 0 {
			cfg.ExcludeAuthors = nil
		} else {
			cfg.ExcludeAuthors = append(cfg.ExcludeAuthors, *fc.ExcludeAuthors...)
		}
	}
	if fc.ExcludePaths != nil {
		if len(*fc.ExcludePaths) == 0 {
			cfg.ExcludePaths = nil
		} else {
			cfg.ExcludePaths = append(cfg.ExcludePaths, *fc.ExcludePaths...)
		}
	}
	if fc.OutlierThresholdLines != nil {
		cfg.OutlierThresholdLines = *fc.OutlierThresholdLines
	}
	if fc.CountMerges != nil {
		cfg.CountMerges = *fc.CountMerges
	}
	if fc.DateSource != nil {
		cfg.DateSource = *fc.DateSource
	}
	if fc.UseMailmap != nil {
		cfg.UseMailmap = *fc.UseMailmap
	}
	if fc.Anonymize != nil {
		cfg.Anonymize = *fc.Anonymize
	}
	if fc.HashEmails != nil {
		cfg.HashEmails = *fc.HashEmails
	}
	if fc.OutputDir != nil {
		cfg.OutputDir = *fc.OutputDir
	}
	if fc.Theme != nil {
		cfg.Theme = *fc.Theme
	}
}

// warnUnknownKeys reports top-level keys commitography does not recognize.
// A typo in a configuration file should be visible without being fatal.
func warnUnknownKeys(path string, data []byte) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return
	}
	if len(doc.Content) == 0 {
		return
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		if !knownKeys[key] {
			Warn("%s:%d: unknown configuration key %q, ignored", path, root.Content[i].Line, key)
		}
	}
}

// Validate checks the invariants every downstream stage relies on.
func (c Config) Validate() error {
	switch c.DateSource {
	case DateSourceAuthor, DateSourceCommitter:
	default:
		return fmt.Errorf("date_source must be %q or %q, got %q",
			DateSourceAuthor, DateSourceCommitter, c.DateSource)
	}
	if c.OutlierThresholdLines <= 0 {
		return fmt.Errorf("outlier_threshold_lines must be greater than 0, got %d", c.OutlierThresholdLines)
	}
	if c.Theme != "default" {
		return fmt.Errorf("theme must be %q, got %q", "default", c.Theme)
	}
	for i, id := range c.Identities {
		if len(id.Emails) == 0 {
			return fmt.Errorf("identities[%d] (%q) lists no emails", i, id.Name)
		}
	}
	return nil
}
