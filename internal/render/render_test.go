package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/aggregate"
)

func sampleReport() *aggregate.Report {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	subject := "fix: strip </script> tags from user input"
	return &aggregate.Report{
		SchemaVersion: aggregate.SchemaVersion,
		GeneratedAt:   now,
		ToolVersion:   "test",
		Repository: aggregate.RepositorySummary{
			Name:            "sample",
			DefaultBranch:   "main",
			HeadCommit:      "0123456789abcdef",
			CommitsTotal:    12,
			CommitsAnalyzed: 12,
			Contributors:    3,
		},
		Temporal: aggregate.TemporalMetrics{
			HourHistogram:    make([]int, 24),
			WeekdayHistogram: make([]int, 7),
			HourWeekdayGrid:  [][]int{},
			CommitsPerMonth:  []aggregate.MonthCount{{Month: "2026-01", Count: 12}},
		},
		Messages: aggregate.MessageMetrics{
			TypeDistribution: map[string]int{"fix": 12},
			LongestSubject: &aggregate.LongestSubject{
				Hash: "abc", Length: len(subject), Subject: subject,
			},
		},
		Notables: aggregate.Notables{FirstCommitSubject: &subject},
		Warnings: []string{},
	}
}

func TestRenderWritesOnlyIndexAndReport(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "out")

	if err := Render(sampleReport(), dir); err != nil {
		t.Fatalf("Render: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 2 {
		t.Fatalf("Render produced %v, want exactly index.html and report.json", names)
	}
	for _, want := range []string{IndexFile, ReportFile} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s was not written", want)
		}
	}
}

func TestRenderedPageIsSelfContained(t *testing.T) {
	dir := t.TempDir()
	if err := Render(sampleReport(), dir); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := readFile(t, filepath.Join(dir, IndexFile))

	// Nothing may be fetched: no external scripts, stylesheets, images or fonts.
	for _, forbidden := range []string{"src=\"http", "href=\"http", "@import", "url(http", "<link "} {
		if strings.Contains(html, forbidden) {
			t.Errorf("page references an external resource via %q", forbidden)
		}
	}
	// The CSS and JS must be inline, not referenced.
	if !strings.Contains(html, "<style>") || !strings.Contains(html, "<script>") {
		t.Error("stylesheet and script are not inlined")
	}
	if strings.Contains(html, `src="app.js"`) || strings.Contains(html, `href="app.css"`) {
		t.Error("page references sibling asset files")
	}
	if !strings.Contains(html, `id="commitography-data"`) {
		t.Error("report payload is missing from the page")
	}
}

func TestScriptClosingTagInSubjectCannotBreakOut(t *testing.T) {
	dir := t.TempDir()
	if err := Render(sampleReport(), dir); err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := readFile(t, filepath.Join(dir, IndexFile))

	payload := between(t, html, `<script type="application/json" id="commitography-data">`, `</script>`)

	if strings.Contains(payload, "<") {
		t.Error("payload contains a raw '<', which could close the surrounding element")
	}
	// The subject survives as a JSON unicode escape rather than as literal
	// angle brackets. Built by concatenation so the expectation cannot itself
	// be mangled by source-level escaping.
	escapedTag := "\\u003c" + "/script" + "\\u003e"
	if !strings.Contains(payload, escapedTag) {
		t.Errorf("the subject's closing tag is not present in its escaped form; payload was %.200s", payload)
	}

	// And it must still be valid JSON that round-trips to the original text.
	var parsed aggregate.Report
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("embedded payload is not valid JSON: %v", err)
	}
	if parsed.Notables.FirstCommitSubject == nil ||
		!strings.Contains(*parsed.Notables.FirstCommitSubject, "</script>") {
		t.Error("escaping lost the original subject text")
	}
}

func TestPageMovesWithoutItsDirectory(t *testing.T) {
	src := t.TempDir()
	if err := Render(sampleReport(), src); err != nil {
		t.Fatalf("Render: %v", err)
	}
	original := readFile(t, filepath.Join(src, IndexFile))

	// Moving the file away from report.json must not change what it contains,
	// because it never referenced it.
	moved := filepath.Join(t.TempDir(), "somewhere-else.html")
	if err := os.WriteFile(moved, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if readFile(t, moved) != original {
		t.Error("the page is not portable")
	}
	if strings.Contains(original, ReportFile) {
		t.Error("the page references report.json, which is meant to be independent")
	}
}

func TestRenderLeavesUnrelatedFilesAlone(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keep, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Render(sampleReport(), dir); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if readFile(t, keep) != "keep me" {
		t.Error("Render disturbed a file it did not create")
	}
}

func TestRenderOverwritesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	if err := Render(sampleReport(), dir); err != nil {
		t.Fatalf("first Render: %v", err)
	}
	if err := Render(sampleReport(), dir); err != nil {
		t.Fatalf("second Render: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, IndexFile+".tmp")); err == nil {
		t.Error("a temporary file was left behind")
	}
}

func TestRenderWrapped(t *testing.T) {
	dir := t.TempDir()
	previous := 40

	if err := RenderWrapped(sampleReport(), dir, 2026, &previous); err != nil {
		t.Fatalf("RenderWrapped: %v", err)
	}
	html := readFile(t, filepath.Join(dir, WrappedFileName(2026)))

	if !strings.Contains(html, `data-mode="wrapped"`) {
		t.Error("wrapped page is not marked as such")
	}
	if !strings.Contains(html, `data-year="2026"`) {
		t.Error("wrapped page does not carry its year")
	}
	if !strings.Contains(html, `data-previous-year-commits="40"`) {
		t.Error("wrapped page does not carry the previous year's count")
	}
}

func TestWrappedOmitsDeltaWhenPreviousYearIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := RenderWrapped(sampleReport(), dir, 2026, nil); err != nil {
		t.Fatalf("RenderWrapped: %v", err)
	}
	html := readFile(t, filepath.Join(dir, WrappedFileName(2026)))
	if strings.Contains(html, "data-previous-year-commits") {
		t.Error("the delta attribute must be absent when the preceding year has no commits")
	}
}

func TestWriteReportJSONIsValidAndIndented(t *testing.T) {
	path := filepath.Join(t.TempDir(), ReportFile)
	if err := WriteReportJSON(sampleReport(), path); err != nil {
		t.Fatalf("WriteReportJSON: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed aggregate.Report
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("report.json is not valid JSON: %v", err)
	}
	if !strings.Contains(string(data), "\n  \"schemaVersion\"") {
		t.Error("report.json should be indented for humans reading it directly")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func between(t *testing.T, haystack, start, end string) string {
	t.Helper()
	i := strings.Index(haystack, start)
	if i < 0 {
		t.Fatalf("could not find %q in the page", start)
	}
	rest := haystack[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("could not find %q after %q", end, start)
	}
	return rest[:j]
}
