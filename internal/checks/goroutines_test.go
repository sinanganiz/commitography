package checks

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// goroutineOwnershipNarrowing lists the files whose bare `go` statements
// predate this checker, with the package that removes each one. A file is
// narrowed as a whole: a new bare `go` statement in one of them is not caught
// until the named package removes the entry.
func goroutineOwnershipNarrowing() map[string]string {
	return map[string]string{
		"internal/pipeline/collect/gitlog.go": "WP-0012 rewrites the sharded collect read",
		"internal/server/jobs.go":             "WP-0038 rewrites the job manager",
		"internal/server/listener.go":         "WP-0037 rewrites the server surface",
		"internal/server/listener_test.go":    "WP-0037 rewrites the server surface",
		"internal/analysis/run_test.go":       "WP-0011 onward rewrite the orchestrator this test drives",
	}
}

// TestGoroutineOwnership enforces ADR-0044 clause 1: every goroutine has an
// owner that waits for its completion, so no bare `go` statement exists.
func TestGoroutineOwnership(t *testing.T) {
	repo := openRepository(t)
	fset := token.NewFileSet()
	narrowed := goroutineOwnershipNarrowing()

	for _, file := range repo.tracked {
		if filepath.Ext(file) != ".go" {
			continue
		}
		if note, ok := narrowed[file]; ok {
			t.Logf("not checked: %s (%s)", file, note)
			continue
		}
		parsed, err := parser.ParseFile(fset, filepath.Join(repo.root, filepath.FromSlash(file)), nil, parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 44, "cannot parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			stmt, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			position := fset.Position(stmt.Go)
			report(t, 44, "%s:%d: bare go statement; every goroutine needs an owner that waits for it", file, position.Line)
			return true
		})
	}
}
