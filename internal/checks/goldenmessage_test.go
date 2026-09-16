package checks

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/gitcmd"
)

// TestGoldenCommitMessages enforces ADR-0019 clause 2: every commit that
// changes a golden file states in its body why the output changed. The
// mechanical form of that rule is a body, apart from its trailers, containing
// a sentence that mentions the golden files.
//
// It reads the whole history, so a shallow clone is a missing precondition
// (ADR-0064 clause 2); the gates check out full history.
func TestGoldenCommitMessages(t *testing.T) {
	repo := openRepository(t)
	ctx := context.Background()
	shallow, err := gitcmd.RunContext(ctx, repo.root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		fatal(t, 19, "cannot tell whether the history is complete: %v", err)
	}
	if shallow != "false" {
		fatal(t, 64, "the repository is a shallow clone, so commits changing %s cannot all be read; "+
			"check out full history (fetch-depth: 0)", goldenDir)
	}
	out, err := gitcmd.RunContext(ctx, repo.root, "log", "-z", "--no-merges", "--format=%H%n%B", "--", goldenDir)
	if err != nil {
		fatal(t, 19, "cannot read the history of %s: %v", goldenDir, err)
	}
	for _, record := range strings.Split(out, "\x00") {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		hash, message, _ := strings.Cut(record, "\n")
		if reason := goldenReason(message); reason == "" {
			subject, _, _ := strings.Cut(strings.TrimSpace(message), "\n")
			report(t, 19, "commit %.12s (%q) changes %s but its body does not state why the output changed; "+
				"a body sentence must mention the golden files", hash, subject, goldenDir)
		}
	}
}

// goldenReason returns the first body sentence that mentions the golden files,
// ignoring the subject and a closing trailer block.
func goldenReason(message string) string {
	paragraphs := regexp.MustCompile(`\n[ \t]*\n`).Split(strings.TrimSpace(strings.ReplaceAll(message, "\r\n", "\n")), -1)
	if len(paragraphs) < 2 {
		return ""
	}
	body := paragraphs[1:]
	trailer := regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*: \S`)
	last := body[len(body)-1]
	isTrailers := true
	for _, line := range strings.Split(last, "\n") {
		if !trailer.MatchString(line) {
			isTrailers = false
			break
		}
	}
	if isTrailers {
		body = body[:len(body)-1]
	}
	golden := regexp.MustCompile(`(?i)\bgolden\b`)
	for _, p := range body {
		for _, sentence := range regexp.MustCompile(`[.!?](\s|$)`).Split(p, -1) {
			if golden.MatchString(sentence) {
				return strings.TrimSpace(sentence)
			}
		}
	}
	return ""
}

func TestGoldenReason(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		ok            bool
	}{
		{"subject only", "WP-1: update golden files", false},
		{"subject mentions golden, body does not", "WP-1: update golden files\n\nRefactor.", false},
		{"trailers only", "WP-1: x\n\nCo-Authored-By: A <a@example.com>", false},
		{"golden only in a trailer", "WP-1: x\n\nGolden-Files: changed", false},
		{"reason in body", "WP-1: x\n\nThe golden files change because counts now exclude bots.", true},
		{"reason before trailers", "WP-1: x\n\nGolden output changes: merges are counted.\n\nCo-Authored-By: A <a@example.com>", true},
		{"crlf", "WP-1: x\r\n\r\nGolden reports gain a field.\r\n", true},
	} {
		if got := goldenReason(tc.message) != ""; got != tc.ok {
			report(t, 19, "%s: goldenReason(%q) found a reason = %v, want %v", tc.name, tc.message, got, tc.ok)
		}
	}
}
