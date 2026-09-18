package checks

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestGoroutineOwnership enforces ADR-0044 clause 1: every goroutine has an
// owner that waits for its completion, so no bare `go` statement exists. A
// goroutine is started through its owner's wait or error group instead, as
// sync.WaitGroup.Go does. Build tags are ignored, so the tagged verification
// packages are covered too, and no file is exempt.
func TestGoroutineOwnership(t *testing.T) {
	repo := openRepository(t)
	fset := token.NewFileSet()

	for _, file := range repo.tracked {
		if filepath.Ext(file) != ".go" {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 44, file), parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 44, "cannot parse %s: %v", file, err)
		}
		for _, line := range bareGoStatements(fset, parsed) {
			report(t, 44, "%s:%d: bare go statement; every goroutine needs an owner that waits for it", file, line)
		}
	}
}

// bareGoStatements returns the line of every go statement in a file.
func bareGoStatements(fset *token.FileSet, parsed *ast.File) []int {
	var lines []int
	ast.Inspect(parsed, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.GoStmt); ok {
			lines = append(lines, fset.Position(stmt.Go).Line)
		}
		return true
	})
	return lines
}

// TestGoroutineOwnershipRejectsABareGoStatement is the failure demonstration
// ADR-0064 clause 6 requires, kept as a test.
func TestGoroutineOwnershipRejectsABareGoStatement(t *testing.T) {
	fset := token.NewFileSet()
	source := `package example

func start(work func()) {
	go work()
}
`
	parsed, err := parser.ParseFile(fset, "example.go", source, parser.SkipObjectResolution)
	if err != nil {
		fatal(t, 64, "parsing the example: %v", err)
	}
	if len(bareGoStatements(fset, parsed)) != 1 {
		report(t, 64, "the goroutine ownership checker accepted a bare go statement, so it cannot catch one")
	}
	owned := `package example

import "sync"

func start(work func()) {
	var wg sync.WaitGroup
	wg.Go(work)
	wg.Wait()
}
`
	parsed, err = parser.ParseFile(fset, "owned.go", owned, parser.SkipObjectResolution)
	if err != nil {
		fatal(t, 64, "parsing the example: %v", err)
	}
	if lines := bareGoStatements(fset, parsed); len(lines) != 0 {
		report(t, 44, "the goroutine ownership checker refused a goroutine started through its wait group")
	}
}
