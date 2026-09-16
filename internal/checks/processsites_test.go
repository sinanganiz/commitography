package checks

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestProcessExecutionSitesNameTheirRecord enforces the reference half of
// ADR-0065 clause 4: every tracked Go file outside the git package that
// imports process execution carries a file-level reference to ADR-0065. The
// lint rule decides where such a file may exist; this checker makes each one
// say why. Build tags are ignored, so tagged verification packages are
// covered too.
func TestProcessExecutionSitesNameTheirRecord(t *testing.T) {
	repo := openRepository(t)
	fset := token.NewFileSet()
	sites := 0

	for _, file := range repo.tracked {
		if filepath.Ext(file) != ".go" || strings.HasPrefix(file, "internal/git/") {
			continue
		}
		content := repo.read(t, 65, file)
		parsed, err := parser.ParseFile(fset, file, content, parser.ImportsOnly)
		if err != nil {
			fatal(t, 65, "cannot parse %s: %v", file, err)
		}
		executes := false
		for _, spec := range parsed.Imports {
			if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == "os/exec" {
				executes = true
			}
		}
		if !executes {
			continue
		}
		sites++
		if !strings.Contains(fileComment(content), "ADR-0065") {
			report(t, 65, "%s imports os/exec but its file-level comment does not name ADR-0065", file)
		}
	}

	// The browser opener and the verification packages always exist, so
	// finding none means the checker is not reading the tree (ADR-0064).
	if sites == 0 {
		fatal(t, 64, "found no process execution site outside internal/git; the checker cannot fail")
	}
}
