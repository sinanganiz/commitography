// Package render turns a report into the self-contained HTML dashboard.
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinanganiz/commitography/internal/aggregate"
)

// assets holds the built frontend. Shipping it inside the binary is what makes
// commitography a single file with no runtime dependency but git itself.
//
//go:embed assets/app.js assets/app.css
var assets embed.FS

// IndexFile and ReportFile are the only files Render ever writes.
const (
	IndexFile  = "index.html"
	ReportFile = "report.json"
)

// pageTemplate is the whole document. CSS, JS and the report are inlined, so
// the output opens correctly from anywhere with no adjacent files and no
// network access.
var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en" data-mode="{{.Mode}}"{{if .Year}} data-year="{{.Year}}"{{end}}{{if .PreviousYearCommits}} data-previous-year-commits="{{.PreviousYearCommits}}"{{end}}>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>{{.Title}}</title>
<style>{{.CSS}}</style>
</head>
<body>
<div id="commitography-root"></div>
<script type="application/json" id="commitography-data">{{.Data}}</script>
<script>{{.JS}}</script>
</body>
</html>
`))

type pageData struct {
	Mode                string
	Title               string
	Year                int
	PreviousYearCommits string
	CSS                 template.CSS
	JS                  template.JS
	// Data is template.JS rather than template.HTML because html/template
	// treats the contents of any script element as JavaScript, whatever its
	// type attribute says, and would otherwise re-encode the payload as a
	// string literal. Break-out safety comes from scriptSafeEscapes below.
	Data template.JS
}

// Render writes the complete dashboard to the output directory.
//
// It produces exactly one page, index.html, with everything inlined, plus
// report.json as a separate machine-readable artifact that the page does not
// reference. No other file is created, and no existing file is removed.
func Render(r *aggregate.Report, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", outputDir, err)
	}
	if err := WriteReportJSON(r, filepath.Join(outputDir, ReportFile)); err != nil {
		return err
	}

	page, err := buildPage(r, pageData{
		Mode:  "dashboard",
		Title: pageTitle(r, 0),
	})
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(outputDir, IndexFile), page)
}

// RenderWrapped writes the year-in-review page, again as one self-contained
// file, alongside whatever the dashboard produced.
func RenderWrapped(r *aggregate.Report, outputDir string, year int, previousYearCommits *int) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", outputDir, err)
	}

	data := pageData{
		Mode:  "wrapped",
		Title: pageTitle(r, year),
		Year:  year,
	}
	// Omitted rather than zeroed when the preceding year is empty, so the
	// first card can leave the year-over-year delta out entirely.
	if previousYearCommits != nil && *previousYearCommits > 0 {
		data.PreviousYearCommits = fmt.Sprint(*previousYearCommits)
	}

	page, err := buildPage(r, data)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(outputDir, WrappedFileName(year)), page)
}

// WrappedFileName is the name of the year-in-review page for a given year.
func WrappedFileName(year int) string {
	return fmt.Sprintf("wrapped-%d.html", year)
}

// WriteReportJSON writes the report on its own, for --json runs and for tools
// that consume the artifact rather than the page.
func WriteReportJSON(r *aggregate.Report, path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding report: %w", err)
	}
	return writeFile(path, append(data, '\n'))
}

func buildPage(r *aggregate.Report, data pageData) ([]byte, error) {
	css, err := assets.ReadFile("assets/app.css")
	if err != nil {
		return nil, fmt.Errorf("reading embedded stylesheet: %w", err)
	}
	js, err := assets.ReadFile("assets/app.js")
	if err != nil {
		return nil, fmt.Errorf("reading embedded script: %w", err)
	}

	payload, err := encodeReport(r)
	if err != nil {
		return nil, err
	}

	data.CSS = template.CSS(css)
	data.JS = template.JS(js)
	data.Data = template.JS(payload)

	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering page: %w", err)
	}
	return buf.Bytes(), nil
}

// scriptSafeEscapes rewrites the characters that could end the surrounding
// script element or upset a JavaScript parser. Every one of them is a legal
// escape inside a JSON string, so the payload stays valid JSON.
//
// encoding/json already escapes the first three by default; doing it here as
// well makes the guarantee independent of that default ever changing. The two
// line separators are not escaped by encoding/json and have historically
// broken JavaScript parsers.
var scriptSafeEscapes = strings.NewReplacer(
	"<", "\\u003c",
	">", "\\u003e",
	"&", "\\u0026",
	" ", "\\u2028",
	" ", "\\u2029",
)

// encodeReport serializes the report for embedding in a script element. A
// commit subject containing "</script>" must not be able to close the tag.
func encodeReport(r *aggregate.Report) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("encoding report: %w", err)
	}
	return scriptSafeEscapes.Replace(string(data)), nil
}

func pageTitle(r *aggregate.Report, year int) string {
	name := r.Repository.Name
	if name == "" {
		name = "repository"
	}
	if year != 0 {
		return fmt.Sprintf("%s — Wrapped %d", name, year)
	}
	return fmt.Sprintf("%s — Commitography", name)
}

// writeFile writes atomically, so an interrupted run never leaves a truncated
// dashboard in place of a working one.
func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming %s to %s: %w", tmp, path, err)
	}
	return nil
}
