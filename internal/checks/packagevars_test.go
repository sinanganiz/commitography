package checks

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// buildMetadataFile is the one file ADR-0061 clause 2 lets declare package
// variables, and buildMetadataNames are the variables it declares.
const buildMetadataFile = "internal/core/buildinfo.go"

func buildMetadataNames() map[string]bool {
	return map[string]bool{"version": true, "commit": true, "buildDate": true}
}

// packageVariableFinding is one violation, located in its file.
type packageVariableFinding struct {
	line    int
	message string
}

// packageVariableFindings applies ADR-0042 clause 2 and ADR-0061 to one parsed
// file: no package-level variable outside the build metadata file, except one
// filled by go:embed, which the toolchain accepts in no other form and which
// is written at compile time; no suppression of the package-variable lint rule
// outside the build metadata file, and the one there names ADR-0061; and no
// assignment to a build metadata variable anywhere in package core.
func packageVariableFindings(fset *token.FileSet, file string, parsed *ast.File) []packageVariableFinding {
	var findings []packageVariableFinding
	add := func(pos token.Pos, format string, args ...any) {
		findings = append(findings, packageVariableFinding{fset.Position(pos).Line, fmt.Sprintf(format, args...)})
	}
	metadata := file == buildMetadataFile

	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR || metadata {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			if embedded(gen, value) {
				continue
			}
			for _, name := range value.Names {
				if name.Name == "_" {
					continue
				}
				add(name.Pos(), "package-level variable %s; pass it through a constructor, or make it a constant "+
					"or a function (clause 2)", name.Name)
			}
		}
	}

	for _, group := range parsed.Comments {
		for _, comment := range group.List {
			if !strings.Contains(comment.Text, "nolint:gochecknoglobals") {
				continue
			}
			switch {
			case !metadata:
				add(comment.Pos(), "suppresses the package-variable rule; only %s may (ADR-0061 clause 2)",
					buildMetadataFile)
			case !strings.Contains(comment.Text, "ADR-0061"):
				add(comment.Pos(), "the build metadata suppression does not name ADR-0061 (ADR-0061 clause 3)")
			}
		}
	}

	if filepath.ToSlash(filepath.Dir(file)) == "internal/core" {
		names := buildMetadataNames()
		written := func(expr ast.Expr) {
			if ident, ok := expr.(*ast.Ident); ok && names[ident.Name] {
				add(ident.Pos(), "writes the build metadata variable %s; it is set at link time only "+
					"(ADR-0061 clause 4)", ident.Name)
			}
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					written(lhs)
				}
			case *ast.IncDecStmt:
				written(node.X)
			case *ast.UnaryExpr:
				// Taking the address hands out a way to write it.
				if node.Op == token.AND {
					written(node.X)
				}
			}
			return true
		})
	}
	return findings
}

// embedded reports whether a variable declaration is filled by go:embed.
func embedded(gen *ast.GenDecl, spec *ast.ValueSpec) bool {
	for _, group := range []*ast.CommentGroup{gen.Doc, spec.Doc} {
		if group == nil {
			continue
		}
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:embed ") {
				return true
			}
		}
	}
	return false
}

// TestPackageVariables enforces ADR-0042 clause 2 as narrowed by ADR-0061
// over every tracked Go file, build tags ignored, so the tagged verification
// packages the linter does not load are covered too. It also requires exactly
// one suppression of the package-variable lint rule, the one ADR-0061 clause 3
// describes.
func TestPackageVariables(t *testing.T) {
	repo := openRepository(t)
	fset := token.NewFileSet()
	suppressions := 0

	for _, file := range repo.tracked {
		if filepath.Ext(file) != ".go" {
			continue
		}
		content := repo.read(t, 42, file)
		parsed, err := parser.ParseFile(fset, file, content, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 42, "cannot parse %s: %v", file, err)
		}
		// Only comments count: a string that names the rule, as this checker's
		// own examples do, suppresses nothing.
		for _, group := range parsed.Comments {
			for _, comment := range group.List {
				if strings.Contains(comment.Text, "nolint:gochecknoglobals") {
					suppressions++
				}
			}
		}
		for _, finding := range packageVariableFindings(fset, file, parsed) {
			report(t, 42, "%s:%d: %s", file, finding.line, finding.message)
		}
	}
	if suppressions != 1 {
		report(t, 61, "found %d suppressions of the package-variable rule; exactly one, in %s, is permitted "+
			"(clause 2)", suppressions, buildMetadataFile)
	}
}

// TestPackageVariablesRejectViolations is the failure demonstration ADR-0064
// clause 6 requires, kept as a test: each source below carries one violation
// the checker exists to catch, and the checker must name it.
func TestPackageVariablesRejectViolations(t *testing.T) {
	for _, tc := range []struct {
		name, file, source string
	}{
		{"a package variable", "internal/pipeline/example.go",
			"package pipeline\n\nvar counter int\n"},
		{"a package variable in a block", "internal/server/example.go",
			"package server\n\nvar (\n\tfirst = 1\n\tsecond = 2\n)\n"},
		{"a suppression outside the metadata file", "internal/server/example.go",
			"package server\n\n//nolint:gochecknoglobals // ADR-0061\nvar x = 1\n"},
		{"a metadata suppression naming no record", buildMetadataFile,
			"package core\n\n//nolint:gochecknoglobals // link-time values\nvar version = \"dev\"\n"},
		{"an assignment to build metadata", "internal/core/example.go",
			"package core\n\nfunc set() { version = \"forged\" }\n"},
		{"an address of build metadata", "internal/core/example.go",
			"package core\n\nfunc leak() *string { return &commit }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, tc.file, tc.source, parser.ParseComments|parser.SkipObjectResolution)
			if err != nil {
				fatal(t, 64, "parsing the example: %v", err)
			}
			if len(packageVariableFindings(fset, tc.file, parsed)) == 0 {
				report(t, 64, "the package-variable checker accepted %s, so it cannot catch one", tc.name)
			}
		})
	}

	// The two permitted forms must pass, or the checker would fail the tree.
	for _, tc := range []struct{ file, source string }{
		{buildMetadataFile, "package core\n\n//nolint:gochecknoglobals // ADR-0061: link-time.\nvar (\n\tversion = \"dev\"\n)\n\nfunc BuildMetadata() string { return version }\n"},
		{"internal/pipeline/render/example.go", "package render\n\nimport \"embed\"\n\n//go:embed assets/app.js\nvar assets embed.FS\n"},
	} {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, tc.file, tc.source, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 64, "parsing the example: %v", err)
		}
		if findings := packageVariableFindings(fset, tc.file, parsed); len(findings) != 0 {
			report(t, 64, "the package-variable checker refused a permitted form in %s: %s", tc.file, findings[0].message)
		}
	}
}
