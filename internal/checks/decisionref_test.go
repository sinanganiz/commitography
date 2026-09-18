package checks

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// bindingSubjects are the files of ADR-0059 clause 1 that exist and therefore
// must carry a record reference. For a package, the file is the one holding
// its package comment.
//
// Subjects that do not exist yet are added by the package that creates each:
// the storage interface and its implementation (WP-0032), the mode switch and
// capability matrix (WP-0045) and the frontend design tokens (WP-0047).
// internal/storage exists only as a reserved, empty package.
func bindingSubjects() []string {
	return []string{
		// Each metric family package.
		"internal/metrics/commitsize/commitsize.go",
		"internal/metrics/coupling/coupling.go",
		"internal/metrics/files/files.go",
		"internal/metrics/hotspot/hotspot.go",
		"internal/metrics/messages/messages.go",
		"internal/metrics/ownership/ownership.go",
		"internal/metrics/temporal/temporal.go",
		// Each pipeline stage package.
		"internal/pipeline/collect/gitlog.go",
		"internal/pipeline/replay/doc.go",
		"internal/pipeline/aggregate/aggregate.go",
		"internal/pipeline/interpret/doc.go",
		"internal/pipeline/render/render.go",
		// The git invocation package.
		"internal/git/git.go",
		// The report type definitions.
		"internal/core/report.go",
		"internal/core/code.go",
		"internal/core/temporal.go",
		"internal/core/messages.go",
		"internal/core/social.go",
		"internal/core/notables.go",
		"internal/core/perauthor.go",
		// The archetype taxonomy definition file.
		"internal/pipeline/interpret/taxonomy/taxonomy.yml",
	}
}

// TestDecisionReference enforces ADR-0059 clause 2: every binding subject
// carries at least one record reference in its file-level comment, and every
// reference it carries resolves. It does not judge whether a reference is apt.
func TestDecisionReference(t *testing.T) {
	t.Parallel()
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
