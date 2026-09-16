package checks

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// bindingSubjects are the files of ADR-0059 clause 1 that exist today at their
// layout location and therefore must carry a record reference.
//
// Narrowed scope: the clause 1 subjects that exist only in their pre-layout
// form are not listed yet, because adding their references is application
// source this checker's package may not touch. WP-0005 clause 8 adds the
// references and widens this list: the git invocation package
// (internal/gitcmd), the report type definitions (internal/aggregate/report.go)
// and the pipeline stage packages (internal/collect, internal/aggregate). The
// remaining subjects (metric family packages, storage, the mode switch and
// capability matrix, frontend design tokens) are added by the package that
// creates each one.
func bindingSubjects() []string {
	return []string{
		"internal/pipeline/interpret/taxonomy/taxonomy.yml",
	}
}

// TestDecisionReference enforces ADR-0059 clause 2: every binding subject
// carries at least one record reference in its file-level comment, and every
// reference it carries resolves. It does not judge whether a reference is apt.
func TestDecisionReference(t *testing.T) {
	repo := openRepository(t)
	known := map[int]bool{}
	for _, r := range loadRecords(t, repo) {
		known[r.number] = true
	}
	reference := regexp.MustCompile(`ADR-([0-9]{4})`)

	for _, file := range bindingSubjects() {
		if !repo.isTracked(file) {
			report(t, 59, "%s is listed as a binding subject but is not tracked", file)
			continue
		}
		refs := reference.FindAllStringSubmatch(fileComment(repo.read(t, 59, file)), -1)
		if len(refs) == 0 {
			report(t, 59, "%s carries no record reference in its file-level comment", file)
			continue
		}
		for _, m := range refs {
			if n, _ := strconv.Atoi(m[1]); !known[n] {
				report(t, 59, "%s refers to %s, which is not a record", file, m[0])
			}
		}
	}
}

// fileComment returns the leading comment block of a source or data file: the
// lines before the first line that is neither blank nor a comment.
func fileComment(content string) string {
	var b strings.Builder
	inBlock := false
	for _, line := range lines(content) {
		trimmed := strings.TrimSpace(line)
		switch {
		case inBlock:
			if strings.Contains(trimmed, "*/") {
				inBlock = false
			}
		case trimmed == "",
			strings.HasPrefix(trimmed, "//"),
			strings.HasPrefix(trimmed, "#"):
		case strings.HasPrefix(trimmed, "/*"):
			inBlock = !strings.Contains(trimmed, "*/")
		default:
			return b.String()
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
