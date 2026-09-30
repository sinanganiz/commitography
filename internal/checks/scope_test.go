package checks

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// The identity bound checker (ADR-0078 clause 9, WP-0063): the individually
// represented identities are selected once, in core, by core.IdentityTable,
// and the identities section and every family's cells fold through that one
// selection, so an identity is individual in every part of the report or in
// none. A second selection, however faithful to the first when written, is a
// second rule free to drift from it.
//
// A selection is recognised by the one thing every selection needs, the
// limit: no Go file under internal/ outside internal/core names
// core.LimitIdentities. The test files of this package are left out, since
// they verify the bound rather than apply it.

// boundLimit is the name of the individually represented identities limit.
const boundLimit = "LimitIdentities"

// boundReferences returns the position of every reference a parsed file makes
// to core.LimitIdentities: through any name the file imports core under, or
// bare where it imports core into its own scope.
func boundReferences(fset *token.FileSet, parsed *ast.File) []string {
	corePath := modulePath + "/internal/core"
	imported := map[string]bool{}
	dotted := false
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != corePath {
			continue
		}
		switch {
		case spec.Name == nil:
			imported["core"] = true
		case spec.Name.Name == ".":
			dotted = true
		case spec.Name.Name != "_":
			imported[spec.Name.Name] = true
		}
	}

	var out []string
	var visit func(ast.Node) bool
	visit = func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := node.X.(*ast.Ident); ok && imported[pkg.Name] && node.Sel.Name == boundLimit {
				out = append(out, fset.Position(node.Pos()).String())
			}
			// The selected name is a field or a method of whatever X is, never
			// the constant itself, so only X is searched further.
			ast.Inspect(node.X, visit)
			return false
		case *ast.Ident:
			if dotted && node.Name == boundLimit {
				out = append(out, fset.Position(node.Pos()).String())
			}
		}
		return true
	}
	ast.Inspect(parsed, visit)
	return out
}

// TestScopeBoundIsSelectedOnce enforces ADR-0078 clause 9 over the tracked
// source: outside internal/core, nothing names the identities limit, so
// nothing but core.IdentityTable selects the individually represented
// identities.
func TestScopeBoundIsSelectedOnce(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	scanned := 0
	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, ".go") || !strings.HasPrefix(file, "internal/") ||
			strings.HasPrefix(file, "internal/core/") ||
			(strings.HasPrefix(file, "internal/checks/") && strings.HasSuffix(file, "_test.go")) {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 78, file), parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 78, "cannot parse %s: %v", file, err)
		}
		scanned++
		for _, at := range boundReferences(fset, parsed) {
			report(t, 78, "%s names core.%s; the individually represented identities are selected once, by "+
				"core.IdentityTable, and everything else folds through the table it computes (clause 9)",
				at, boundLimit)
		}
	}
	if scanned == 0 {
		fatal(t, 64, "no Go file under internal/ outside internal/core was found, so nothing was checked")
	}
}

// TestScopeBoundCheckerFindsEveryForm exercises the recognition the checker
// depends on (ADR-0064 clause 6): the limit named through core's own name,
// through an alias and through a dot import is found, and a field of the same
// name, a mention in a comment and an unrelated package's constant are not.
func TestScopeBoundCheckerFindsEveryForm(t *testing.T) {
	t.Parallel()
	references := func(source string) []string {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, "internal/pipeline/aggregate/demo.go", source, parser.ParseComments)
		if err != nil {
			fatal(t, 64, "parsing a demonstration source: %v", err)
		}
		return boundReferences(fset, parsed)
	}
	for _, c := range []struct {
		name, source string
		want         int
	}{
		{"core's own name", `package aggregate
import "github.com/sinanganiz/commitography/internal/core"
func f(n int) bool { return n > core.LimitIdentities }
`, 1},
		{"an alias", `package aggregate
import c "github.com/sinanganiz/commitography/internal/core"
func f(all []string) []string { return all[:c.LimitIdentities] }
`, 1},
		{"a dot import", `package aggregate
import . "github.com/sinanganiz/commitography/internal/core"
func f() int { return LimitIdentities + 1 }
`, 1},
		{"a field, a comment and another package", `package aggregate
import (
	"github.com/sinanganiz/commitography/internal/core"
	other "github.com/sinanganiz/commitography/internal/core/config"
)
type limits struct{ LimitIdentities int }
// f does not use core.LimitIdentities.
func f(l limits) int { return l.LimitIdentities + other.LimitIdentities + core.LimitHotspots }
`, 0},
	} {
		if got := references(c.source); len(got) != c.want {
			report(t, 64, "%s gave %d references, want %d: %v", c.name, len(got), c.want, got)
		}
	}
}
