package filter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/identity"
	"github.com/sinanganiz/commitography/internal/model"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture %q not built; run `make fixtures`", name)
	}
	return path
}

// analyze runs the full identity + filter pipeline over a fixture.
func analyze(t *testing.T, name string, cfg config.Config) (Result, *PathFilter) {
	t.Helper()
	repo := fixture(t, name)
	h, err := collect.Collect(collect.Options{RepoPath: repo, UseMailmap: cfg.UseMailmap})
	if err != nil {
		t.Fatalf("Collect(%s): %v", name, err)
	}
	pf, err := NewPathFilter(cfg, repo)
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	r := identity.NewResolver(cfg, h.Commits)
	return Apply(h.Commits, cfg, r, pf), pf
}

func TestLockfileContributesNoLines(t *testing.T) {
	res, pf := analyze(t, "noise", config.Default())

	var lockfileLines, includedLines int
	for _, c := range res.Commits {
		for _, f := range c.Files {
			if f.Path == "package-lock.json" {
				lockfileLines += f.Added
			}
		}
		if !CountsForLines(c) {
			continue
		}
		for _, f := range IncludedFiles(c, pf) {
			includedLines += f.Added
		}
	}

	if lockfileLines < 39000 {
		t.Fatalf("fixture lockfile added only %d lines; expected roughly 40,000", lockfileLines)
	}
	if pf.Excluded("package-lock.json") != true {
		t.Fatal("package-lock.json is not excluded by default")
	}
	if includedLines >= lockfileLines {
		t.Errorf("aggregate added lines (%d) still include the lockfile (%d)", includedLines, lockfileLines)
	}
}

func TestLinguistGeneratedIsExcluded(t *testing.T) {
	repo := fixture(t, "noise")
	if _, err := os.Stat(filepath.Join(repo, ".gitattributes")); err != nil {
		t.Skip("fixture has no .gitattributes; rebuild fixtures")
	}
	pf, err := NewPathFilter(config.Default(), repo)
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	for _, path := range []string{"api/client.go", "api/types.go"} {
		if !pf.Excluded(path) {
			t.Errorf("%s should be excluded by `api/*.go linguist-generated=true`", path)
		}
	}
	if pf.Excluded("main.go") {
		t.Error("main.go must not be excluded")
	}
}

func TestGitAttributesNegationReincludes(t *testing.T) {
	dir := t.TempDir()
	body := "generated/*.go linguist-generated=true\ngenerated/keep.go -linguist-generated\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.ExcludePaths = nil
	pf, err := NewPathFilter(cfg, dir)
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	if !pf.Excluded("generated/api.go") {
		t.Error("generated/api.go should be excluded")
	}
	if pf.Excluded("generated/keep.go") {
		t.Error("-linguist-generated should re-include generated/keep.go")
	}
}

func TestDefaultPatternsMatchAtAnyDepth(t *testing.T) {
	pf, err := NewPathFilter(config.Default(), t.TempDir())
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	excluded := []string{
		"yarn.lock",
		"deep/nested/thing.lock",
		"vendor/github.com/x/y.go",
		"node_modules/left-pad/index.js",
		"web/dist/app.min.js",
		"src/db/migrations/001_init.sql",
		"api/service.pb.go",
	}
	for _, path := range excluded {
		if !pf.Excluded(path) {
			t.Errorf("%s should be excluded by default", path)
		}
	}
	included := []string{"src/main.go", "README.md", "web/src/app.ts", "cmd/tool/main.go"}
	for _, path := range included {
		if pf.Excluded(path) {
			t.Errorf("%s must not be excluded by default", path)
		}
	}
}

// Matching stays literal, but the built-in list carries a `**/` form of every
// root-anchored default so a monorepo's nested lockfiles and dependency trees
// are excluded too. Without these, exit criterion 4 held only for repositories
// with a single package at the root.
func TestDefaultsCoverNestedMonorepoPaths(t *testing.T) {
	pf, err := NewPathFilter(config.Default(), t.TempDir())
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}

	nested := []string{
		"web/pnpm-lock.yaml",
		"web/package-lock.json",
		"packages/api/yarn.lock",
		"services/worker/go.sum",
		"crates/core/Cargo.lock",
		"backend/Gemfile.lock",
		"php/composer.lock",
		"py/poetry.lock",
		"packages/api/node_modules/left-pad/index.js",
		"services/web/vendor/github.com/x/y.go",
		"apps/site/dist/bundle.js",
		"apps/site/build/output.txt",
		"tools/gen/out/report.txt",
		"rust/app/target/debug/main",
		"ios/App/Pods/Alamofire/Source.swift",
		"deps/third_party/zlib/zlib.c",
	}
	for _, path := range nested {
		if !pf.Excluded(path) {
			t.Errorf("%s should be excluded: nested dependency paths are still generated content", path)
		}
	}

	// The `**/` forms must not turn into a blanket exclusion of anything that
	// merely contains one of these words.
	kept := []string{
		"src/build.go",
		"web/src/dist.ts",
		"cmd/out.go",
		"internal/vendored_test.go",
		"docs/node_modules.md",
	}
	for _, path := range kept {
		if pf.Excluded(path) {
			t.Errorf("%s must not be excluded: it is a source file, not a dependency tree", path)
		}
	}
}

func TestEmptyExcludeListDisablesFiltering(t *testing.T) {
	cfg := config.Default()
	cfg.ExcludePaths = nil
	pf, err := NewPathFilter(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	if pf.Excluded("package-lock.json") {
		t.Error("no patterns configured, yet a path was excluded")
	}
}

func TestMergesAreExcludedByDefault(t *testing.T) {
	res, _ := analyze(t, "merges", config.Default())
	if res.ExcludedMerges != 5 {
		t.Errorf("ExcludedMerges = %d, want 5", res.ExcludedMerges)
	}
	for _, c := range res.Commits {
		if c.IsMerge && !c.Excluded {
			t.Errorf("merge commit %s was not excluded", c.Hash)
		}
	}
}

func TestCountMergesKeepsMerges(t *testing.T) {
	cfg := config.Default()
	cfg.CountMerges = true
	res, _ := analyze(t, "merges", cfg)
	if res.ExcludedMerges != 0 {
		t.Errorf("ExcludedMerges = %d, want 0 when count_merges is set", res.ExcludedMerges)
	}
}

func TestBulkCommitIsFlagged(t *testing.T) {
	res, _ := analyze(t, "noise", config.Default())

	if len(res.BulkCommits) == 0 {
		t.Fatal("no bulk commit detected in the noise fixture")
	}
	found := false
	for _, c := range res.Commits {
		if c.IsBulk {
			found = true
			if c.Excluded {
				t.Error("a bulk commit must stay in commit and temporal metrics")
			}
			if CountsForLines(c) {
				t.Error("a bulk commit must be kept out of line-based metrics")
			}
		}
	}
	if !found {
		t.Error("expected the lockfile-and-data-table commit to be flagged as bulk")
	}
}

func TestBotCommitsAreExcluded(t *testing.T) {
	res, _ := analyze(t, "bots", config.Default())
	if res.ExcludedBots != 2 {
		t.Errorf("ExcludedBots = %d, want 2", res.ExcludedBots)
	}
	if res.AnalyzedCommits != res.TotalCommits-2 {
		t.Errorf("analyzed %d of %d commits, want %d", res.AnalyzedCommits, res.TotalCommits, res.TotalCommits-2)
	}
	for _, c := range res.Commits {
		if c.IdentityID == "" {
			t.Errorf("commit %s was not assigned an identity", c.Hash)
		}
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	commits := []model.Commit{{
		Hash:        "a",
		AuthorName:  "Ada",
		AuthorEmail: "ada@example.com",
		AuthorDate:  time.Now(),
		Parents:     []string{"x", "y"},
		IsMerge:     true,
	}}
	cfg := config.Default()
	pf, err := NewPathFilter(cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	Apply(commits, cfg, identity.NewResolver(cfg, commits), pf)
	if commits[0].Excluded || commits[0].IdentityID != "" {
		t.Error("Apply modified the caller's slice")
	}
}
