package filter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ADR-0064: fixture %q is missing; the gates generate it with `make fixtures`", name)
	}
	return path
}

func TestLinguistGeneratedIsExcluded(t *testing.T) {
	repo := fixture(t, "noise")
	if _, err := os.Stat(filepath.Join(repo, ".gitattributes")); err != nil {
		t.Fatal("ADR-0064: the noise fixture has no .gitattributes; regenerate it with `make fixtures`")
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
// are excluded too. Without these, the defaults excluded generated paths only in
// repositories with a single package at the root.
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
