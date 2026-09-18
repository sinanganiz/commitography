package checks

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"
	"testing"
)

// sequentialPackages are the test packages whose tests may not run in
// parallel, with the reason. Each is a condition that is permanent for the
// package, not a missing precondition (ADR-0064 clause 3).
func sequentialPackages() map[string]string {
	return map[string]string{
		"internal/checks/perfcheck":   "it measures wall time and memory, which concurrent tests would distort",
		"internal/checks/dockersmoke": "it measures container stop time and shares one Docker daemon",
	}
}

// sequentialTests returns the line of every top-level test in a file whose
// first statement is not a call to t.Parallel.
func sequentialTests(fset *token.FileSet, parsed *ast.File) []int {
	var lines []int
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Name.Name == "TestMain" {
			continue
		}
		if fn.Body == nil || !startsWithParallel(fn) {
			lines = append(lines, fset.Position(fn.Pos()).Line)
		}
	}
	return lines
}

func startsWithParallel(fn *ast.FuncDecl) bool {
	params := fn.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 || len(fn.Body.List) == 0 {
		return false
	}
	stmt, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Parallel" {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == params[0].Names[0].Name
}

// TestTestsRunInParallel enforces ADR-0042's consequence that tests run in
// parallel without shared state, which ADR-0019 requires: every top-level
// test calls t.Parallel first. A test that cannot has shared state, and the
// state is what must go.
func TestTestsRunInParallel(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	sequential := sequentialPackages()
	checked := 0

	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, "_test.go") {
			continue
		}
		if _, ok := sequential[path.Dir(file)]; ok {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 42, file), parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 42, "cannot parse %s: %v", file, err)
		}
		checked++
		for _, line := range sequentialTests(fset, parsed) {
			report(t, 42, "%s:%d: the test does not call t.Parallel first; tests share no state and run in "+
				"parallel (consequences, and ADR-0019)", file, line)
		}
	}
	if checked == 0 {
		fatal(t, 64, "found no test file to check, so the checker cannot fail")
	}
	for dir, reason := range sequential {
		t.Logf("sequential by design: %s (%s)", dir, reason)
	}
}

// TestTestsRunInParallelRejectsASequentialTest is the failure demonstration
// ADR-0064 clause 6 requires, kept as a test.
func TestTestsRunInParallelRejectsASequentialTest(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"package example\n\nimport \"testing\"\n\nfunc TestSequential(t *testing.T) {\n\tt.Log(1)\n}\n",
		"package example\n\nimport \"testing\"\n\nfunc TestLate(t *testing.T) {\n\tt.Log(1)\n\tt.Parallel()\n}\n",
		"package example\n\nimport \"testing\"\n\nfunc TestEmpty(t *testing.T) {}\n",
	} {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, "example_test.go", source, parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 64, "parsing the example: %v", err)
		}
		if len(sequentialTests(fset, parsed)) != 1 {
			report(t, 64, "the parallel test checker accepted a test that does not call t.Parallel first, "+
				"so it cannot catch one:\n%s", source)
		}
	}
	fset := token.NewFileSet()
	source := "package example\n\nimport \"testing\"\n\nfunc TestMain(m *testing.M) {}\n\n" +
		"func TestParallel(t *testing.T) {\n\tt.Parallel()\n}\n"
	parsed, err := parser.ParseFile(fset, "example_test.go", source, parser.SkipObjectResolution)
	if err != nil {
		fatal(t, 64, "parsing the example: %v", err)
	}
	if lines := sequentialTests(fset, parsed); len(lines) != 0 {
		report(t, 42, "the parallel test checker refused a test that calls t.Parallel first")
	}
}
