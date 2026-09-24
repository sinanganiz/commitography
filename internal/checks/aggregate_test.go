package checks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// The aggregate stage's checkers (WP-0061). The registry is the only route by
// which a family's output reaches the report, and each family's section lands
// under the namespace the family declares (ADR-0076 clauses 1 and 5). The
// stage runs from cached collect output and replay state with the repository
// gone (ADR-0020 clause 5), and the degree it runs families at changes
// nothing (ADR-0052 clause 6).

// registryFile is the one source file that runs metric families and places
// their sections into a report.
const registryFile = "internal/pipeline/aggregate/registry.go"

// renderReport writes a report as the command writes report.json, and
// returns what it wrote.
func renderReport(t *testing.T, record int, r *core.Report) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), render.ReportFile)
	if err := render.WriteReportJSON(r, out); err != nil {
		fatal(t, record, "writing the report: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		fatal(t, record, "reading the report back: %v", err)
	}
	return string(data)
}

// cacheInputs writes what aggregation runs over to files, the collect
// stage's history in its own artifact format and the replay state as JSON,
// and returns a function reading them back.
func cacheInputs(t *testing.T, in *pipeline.Inputs) func() pipeline.Inputs {
	t.Helper()
	dir := t.TempDir()
	historyFile, replayFile := filepath.Join(dir, "history.json"), filepath.Join(dir, "replay.json")
	if err := collect.WriteHistory(in.History, historyFile); err != nil {
		fatal(t, 20, "writing the history artifact: %v", err)
	}
	state, err := json.Marshal(in.Replay)
	if err != nil {
		fatal(t, 20, "encoding the replay state: %v", err)
	}
	if err := os.WriteFile(replayFile, state, 0o644); err != nil {
		fatal(t, 20, "writing the replay state: %v", err)
	}
	analysis := in.Analysis
	return func() pipeline.Inputs {
		history, err := collect.ReadHistory(historyFile)
		if err != nil {
			fatal(t, 20, "reading the history artifact: %v", err)
		}
		data, err := os.ReadFile(replayFile)
		if err != nil {
			fatal(t, 20, "reading the replay state: %v", err)
		}
		var replayed core.ReplayState
		if err := json.Unmarshal(data, &replayed); err != nil {
			fatal(t, 20, "decoding the replay state: %v", err)
		}
		return pipeline.Inputs{History: history, Replay: &replayed, Analysis: analysis}
	}
}

// TestAggregateWithoutTheRepository enforces ADR-0020 clause 5: the aggregate
// stage is stateless and re-runnable without touching git. Every fixture the
// analysis reports on is analysed in a copy. The collect stage's history and
// the replay stage's state are written to files, the copy is removed, and
// aggregating what is read back must produce the report the analysis
// produced, byte for byte.
func TestAggregateWithoutTheRepository(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	ctx := context.Background()
	for _, fixture := range collectedFixtures(t, repo) {
		dir := filepath.Join(t.TempDir(), fixture)
		copyTree(t, filepath.Join(repo.root, "testdata", "fixtures", fixture), dir)
		analyzer, opts := newAnalyzer(), pipeline.Options{RepoPath: dir}

		result, err := analyzer.Run(ctx, opts, nil)
		if err != nil {
			fatal(t, 20, "analysing %s: %v", fixture, err)
		}
		want := renderReport(t, 20, result.Report)
		prepared, err := analyzer.Prepare(ctx, opts)
		if err != nil {
			fatal(t, 20, "preparing %s: %v", fixture, err)
		}
		cached := cacheInputs(t, prepared)

		if err := os.RemoveAll(dir); err != nil {
			fatal(t, 64, "removing the %s repository: %v", fixture, err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
			fatal(t, 64, "the %s repository is still there, so aggregating without it proves nothing", fixture)
		}
		aggregated, _, err := analyzer.Aggregate(ctx, cached(), opts)
		if err != nil {
			report(t, 20, "aggregating %s from its cached inputs, with the repository removed: %v", fixture, err)
			continue
		}
		if got := renderReport(t, 20, aggregated); got != want {
			report(t, 20, "aggregating %s from its cached inputs, with the repository removed, produced another "+
				"report than the analysis (- analysis, + aggregation):\n%s", fixture, unifiedDiff(want, got))
		}
	}
}

// TestAggregateAcrossParallelism is ADR-0052 clause 6 for the families: each
// fixture's inputs are prepared once, and aggregating them at degrees 1, 2 and
// many produces byte-identical reports and the same warnings in the same
// order. Collect is not run again, so a difference is the family degree's
// alone; TestDeterminismAcrossParallelism holds the whole analysis, collect
// included, to the same degrees.
func TestAggregateAcrossParallelism(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	ctx := context.Background()
	analyzer := newAnalyzer()
	for _, fixture := range collectedFixtures(t, repo) {
		prepared, err := analyzer.Prepare(ctx, pipeline.Options{RepoPath: filepath.Join(repo.root, "testdata",
			"fixtures", fixture)})
		if err != nil {
			fatal(t, 52, "preparing %s: %v", fixture, err)
		}
		var first string
		var firstWarnings []string
		for i, degree := range parallelDegrees() {
			aggregated, warnings, err := analyzer.Aggregate(ctx, *prepared, pipeline.Options{Parallelism: degree})
			if err != nil {
				fatal(t, 52, "aggregating %s at degree %d: %v", fixture, degree, err)
			}
			got := renderReport(t, 52, aggregated)
			if i == 0 {
				first, firstWarnings = got, warnings
				continue
			}
			if got != first {
				report(t, 52, "aggregating %s at degree %d produced another report than at degree 1 "+
					"(- degree 1, + degree %d):\n%s", fixture, degree, degree, unifiedDiff(first, got))
			}
			if !reflect.DeepEqual(warnings, firstWarnings) {
				report(t, 52, "aggregating %s at degree %d raised the warnings %q, and at degree 1 %q",
					fixture, degree, warnings, firstWarnings)
			}
		}
	}
}

// TestAggregateNamespaceOwnership is ADR-0076 clause 5 as every golden report
// carries it: each registered family's section is the one under the namespace
// the family declares, carrying the version and the method statement the
// family declares, and no family is written under any other key. The golden
// checker holds the golden reports to what the analysis produces.
func TestAggregateNamespaceOwnership(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	declared := map[string]core.FamilyDeclaration{}
	for _, f := range aggregate.Registry() {
		d := f.Declaration()
		declared[d.Namespace] = d
	}
	checked := 0
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, goldenDir+"/") || !strings.HasSuffix(file, ".json") {
			continue
		}
		var document struct {
			Families map[string]struct {
				Version core.Version `json:"version"`
				Status  core.Status  `json:"status"`
				Reasons []string     `json:"reasons"`
				Method  string       `json:"method"`
			} `json:"families"`
		}
		if err := json.Unmarshal([]byte(repo.read(t, 76, file)), &document); err != nil {
			report(t, 76, "%s is not a report: %v", file, err)
			continue
		}
		checked++
		for _, namespace := range sortedKeys(document.Families) {
			if _, ok := declared[namespace]; !ok {
				report(t, 76, "%s carries a family under %s, which no registered family declares as its namespace",
					file, namespace)
			}
		}
		for _, namespace := range sortedKeys(declared) {
			d := declared[namespace]
			section, ok := document.Families[namespace]
			switch {
			case !ok:
				report(t, 32, "%s carries no section under %s, the namespace the family %s declares", file,
					namespace, d.Name)
			case section.Version != d.Version:
				report(t, 31, "%s carries version %s under %s, and the family %s declares version %s", file,
					section.Version, namespace, d.Name, d.Version)
			case section.Method != d.Method:
				report(t, 76, "%s carries another method statement under %s than the family %s declares", file,
					namespace, d.Name)
			case d.Status == core.StatusNotImplemented &&
				(section.Status != core.StatusSkipped || !containsString(section.Reasons, string(core.ReasonNotImplemented))):
				report(t, 32, "%s gives %s the status %s %v, and the family %s declares it not implemented", file,
					namespace, section.Status, section.Reasons, d.Name)
			}
		}
	}
	if checked == 0 {
		fatal(t, 64, "no golden report is tracked under %s, so there is nothing to check", goldenDir)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestAggregateNamespaceOwnershipRejectsAForeignWrite is the failure
// demonstration for the write itself (ADR-0064 clause 6): core.Place, the one
// write into a report's families, refuses a section under another family's
// namespace, under a key no family has, under a namespace already written,
// and a section with no status.
func TestAggregateNamespaceOwnershipRejectsAForeignWrite(t *testing.T) {
	t.Parallel()
	var families core.Families
	churn := core.Computed(core.Version{Major: 1}, core.HotspotMetrics{})

	if err := core.Place(&families, "files", churn); err == nil || !strings.Contains(err.Error(), "ADR-0076 clause 5") {
		report(t, 64, "a hotspot section was placed under files, the files family's namespace: %v", err)
	}
	if err := core.Place(&families, "commit-size", core.Computed(core.Version{Major: 2}, core.CommitSizeMetrics{})); err == nil {
		report(t, 64, "a section was placed under commit-size, which is no family's namespace")
	}
	if err := core.Place(&families, "temporal", core.Family[core.TemporalMetrics]{}); err == nil {
		report(t, 64, "a section with no status was placed under temporal")
	}
	if err := core.Place(&families, "hotspot", churn); err != nil {
		report(t, 76, "the hotspot section was refused under hotspot, its own namespace: %v", err)
	}
	if err := core.Place(&families, "hotspot", churn); err == nil || !strings.Contains(err.Error(), "one owner") {
		report(t, 64, "a second section was placed under hotspot: %v", err)
	}
	if missing := families.Unplaced(); len(missing) != reflect.TypeOf(families).NumField()-1 || containsString(missing, "hotspot") {
		report(t, 64, "after one placement the unplaced namespaces are %v", missing)
	}
}

// routeViolations returns what a parsed production file does that could put
// a family's output into a report other than through the registry: import a
// metric family, place a section, or write into a report's families, by
// assignment or by degrading a section in place.
//
// A write is recognised by what it is written through: a field named
// Families, or a variable declared as core.Families, a pointer to one, or the
// address of a report's Families field. Within internal/core the type is
// named Families alone.
func routeViolations(fset *token.FileSet, file string, parsed *ast.File) []string {
	var out []string
	corePath := modulePath + "/internal/core"
	inCore := strings.HasPrefix(file, "internal/core/")
	local := map[string]string{}
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		local[name] = path
		if strings.HasPrefix(path, modulePath+"/internal/metrics/") && file != registryFile {
			out = append(out, fmt.Sprintf("%s imports the metric family %s; only the registry runs a family",
				fset.Position(spec.Pos()), path))
		}
	}
	isCore := func(x ast.Expr, name string) bool {
		if ident, ok := x.(*ast.Ident); ok {
			return inCore && ident.Name == name
		}
		selector, ok := x.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != name {
			return false
		}
		pkg, ok := selector.X.(*ast.Ident)
		return ok && local[pkg.Name] == corePath
	}
	isFamiliesType := func(x ast.Expr) bool {
		if star, ok := x.(*ast.StarExpr); ok {
			x = star.X
		}
		return isCore(x, "Families")
	}
	// families holds the variables that hold a report's families.
	families := map[string]bool{}
	var throughFamilies func(ast.Expr) bool
	throughFamilies = func(x ast.Expr) bool {
		switch e := x.(type) {
		case *ast.Ident:
			return families[e.Name]
		case *ast.SelectorExpr:
			return e.Sel.Name == "Families" || throughFamilies(e.X)
		case *ast.IndexExpr:
			return throughFamilies(e.X)
		case *ast.StarExpr:
			return throughFamilies(e.X)
		case *ast.ParenExpr:
			return throughFamilies(e.X)
		case *ast.UnaryExpr:
			return e.Op == token.AND && throughFamilies(e.X)
		}
		return false
	}
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.Field:
			if isFamiliesType(node.Type) {
				for _, name := range node.Names {
					families[name.Name] = true
				}
			}
		case *ast.ValueSpec:
			if node.Type != nil && isFamiliesType(node.Type) {
				for _, name := range node.Names {
					families[name.Name] = true
				}
			}
		}
		return true
	})
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for i, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && len(node.Rhs) == len(node.Lhs) &&
						throughFamilies(node.Rhs[i]) {
						families[ident.Name] = true
					}
				}
				return true
			}
			for _, lhs := range node.Lhs {
				if throughFamilies(lhs) {
					out = append(out, fmt.Sprintf("%s writes into a report's families directly; a family's "+
						"output reaches the report through its registration alone", fset.Position(lhs.Pos())))
				}
			}
		case *ast.IncDecStmt:
			if throughFamilies(node.X) {
				out = append(out, fmt.Sprintf("%s writes into a report's families directly", fset.Position(node.Pos())))
			}
		case *ast.CompositeLit:
			if isCore(node.Type, "Families") {
				out = append(out, fmt.Sprintf("%s builds a report's families directly", fset.Position(node.Pos())))
			}
		case *ast.CallExpr:
			fun := node.Fun
			if index, ok := fun.(*ast.IndexExpr); ok {
				fun = index.X
			}
			if isCore(fun, "Place") && file != registryFile {
				out = append(out, fmt.Sprintf("%s places a family section; only the registry does",
					fset.Position(node.Pos())))
			}
			if selector, ok := fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Degrade" &&
				throughFamilies(selector.X) {
				out = append(out, fmt.Sprintf("%s degrades a section already in the report; the registry "+
					"applies every condition before it places the section", fset.Position(node.Pos())))
			}
		}
		return true
	})
	return out
}

// TestAggregateIsTheOnlyRoute enforces that registration is the only route by
// which a family's output reaches the report (ADR-0076 clauses 1 and 5): in
// the product's source, only the registry imports a metric family and places
// a section, and nothing writes into a report's families directly. The metric
// families' own packages are left out, since a family never holds a report,
// and so are the tests, which may build any report they examine.
func TestAggregateIsTheOnlyRoute(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	if !repo.isTracked(registryFile) {
		fatal(t, 76, "%s is not tracked, so no registry exists to route through", registryFile)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") ||
			strings.HasPrefix(file, "internal/checks/") || strings.HasPrefix(file, "internal/metrics/") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 76, file), 0)
		if err != nil {
			fatal(t, 76, "cannot parse %s: %v", file, err)
		}
		scanned++
		for _, finding := range routeViolations(fset, file, parsed) {
			report(t, 76, "%s", finding)
		}
	}
	if scanned == 0 {
		fatal(t, 64, "no production source was found, so nothing was checked")
	}
}

// TestAggregateOnlyRouteRejectsADirectWrite is the failure demonstration
// (ADR-0064 clause 6): the writes the stage and the pipeline root made before
// the registry, and a section placed outside it, are each found, and reading
// a report's families is not.
func TestAggregateOnlyRouteRejectsADirectWrite(t *testing.T) {
	t.Parallel()
	violations := func(file, source string) []string {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, source, 0)
		if err != nil {
			fatal(t, 64, "parsing a demonstration source: %v", err)
		}
		return routeViolations(fset, file, parsed)
	}
	const header = `import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
)
`
	for _, c := range []struct {
		name, file, source string
		want               int
	}{
		{"the pipeline root's post-hoc method write", "internal/pipeline/run.go", `package pipeline
import "github.com/sinanganiz/commitography/internal/core"
func f(r *core.Report) { r.Families.CommitSize.Method = "text" }
`, 1},
		{"a family run and written by the stage", "internal/pipeline/aggregate/aggregate.go", "package aggregate\n" +
			header + `func f(r *core.Report, in core.Input) { r.Families.Temporal = temporal.Build(in, nil) }
`, 2},
		{"a family run by the registry and written directly", registryFile, "package aggregate\n" + header +
			`func f(f *core.Families, in core.Input) { f.Temporal = temporal.Build(in, nil) }
`, 1},
		{"a families object built outside the registry", "internal/server/api.go", `package server
import "github.com/sinanganiz/commitography/internal/core"
func f() core.Report { return core.Report{Families: core.Families{}} }
`, 1},
		{"a section placed outside the registry", "internal/pipeline/aggregate/aggregate.go", `package aggregate
import "github.com/sinanganiz/commitography/internal/core"
func f(f *core.Families) error { return core.Place(f, "temporal", core.Family[core.TemporalMetrics]{}) }
`, 1},
		{"a write through the address of a report's families", "internal/pipeline/aggregate/aggregate.go",
			`package aggregate
import "github.com/sinanganiz/commitography/internal/core"
func f(r *core.Report) { f := &r.Families; f.Ownership = core.Family[core.OwnershipMetrics]{} }
`, 1},
		{"a section degraded where it lies in the report", "internal/pipeline/aggregate/aggregate.go",
			`package aggregate
import "github.com/sinanganiz/commitography/internal/core"
func f(r *core.Report) { r.Families.Temporal.Degrade(core.ReasonShallowClone, core.ConfidenceLow) }
`, 1},
		{"a write in core outside the placement", "internal/core/report.go", `package core
func (f *Families) reset() { f.Hotspot = Family[HotspotMetrics]{} }
`, 1},
	} {
		if got := violations(c.file, c.source); len(got) != c.want {
			report(t, 64, "%s gave %d findings, want %d: %v", c.name, len(got), c.want, got)
		}
	}
	if clean := violations("internal/pipeline/render/render.go", `package render
import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)
func f(r *core.Report, c *model.Commit) core.Status {
	c.Files = nil
	s := r.Families.Temporal.Status
	return s
}
`); len(clean) != 0 {
		report(t, 76, "reading a report's families, or writing a field of the same name elsewhere, was "+
			"reported as a write: %v", clean)
	}
}
