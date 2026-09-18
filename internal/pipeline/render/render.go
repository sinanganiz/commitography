// Package render is the render stage (ADR-0020): it turns a report into its
// output files and holds the embedded frontend bundle (ADR-0034, ADR-0036).
package render

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
)

// assets holds the built frontend. Shipping it inside the binary is what makes
// commitography a single file with no runtime dependency but git itself.
//
// It is the one package-level variable outside the build metadata file, and it
// is not state: the toolchain fills it at compile time, go:embed accepts
// nothing but a package-level variable, and nothing assigns to it. The
// package-variable lint rule exempts embedded files for that reason, so no
// suppression is needed.
//
//go:embed assets/app.js assets/app.css
var assets embed.FS

// AssetFS exposes the embedded frontend assets to the local HTTP server. The
// returned filesystem is read-only and remains backed by the binary.
func AssetFS() fs.FS { return assets }

// IndexFile and ReportFile are the only files Render ever writes.
const (
	IndexFile  = "index.html"
	ReportFile = "report.json"
)

// pageSource is the whole document. CSS, JS and the report are inlined, so
// the output opens correctly from anywhere with no adjacent files and no
// network access.
const pageSource = `<!doctype html>
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
`

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
func Render(r *core.Report, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return core.Internalf(err, "creating the output directory")
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
func RenderWrapped(r *core.Report, outputDir string, year int, previousYearCommits *int) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return core.Internalf(err, "creating the output directory")
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
func WriteReportJSON(r *core.Report, path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return core.Internalf(err, "creating the report's directory")
		}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return core.Internalf(err, "encoding the report")
	}
	return writeFile(path, append(data, '\n'))
}

func buildPage(r *core.Report, data pageData) ([]byte, error) {
	css, err := assets.ReadFile("assets/app.css")
	if err != nil {
		return nil, core.Internalf(err, "reading the embedded stylesheet")
	}
	js, err := assets.ReadFile("assets/app.js")
	if err != nil {
		return nil, core.Internalf(err, "reading the embedded script")
	}

	payload, err := encodeReport(r)
	if err != nil {
		return nil, err
	}

	data.CSS = template.CSS(css)
	data.JS = template.JS(js)
	data.Data = template.JS(payload)

	page, err := template.New("page").Parse(pageSource)
	if err != nil {
		return nil, core.Internalf(err, "parsing the page template")
	}
	var buf bytes.Buffer
	if err := page.Execute(&buf, data); err != nil {
		return nil, core.Internalf(err, "rendering the page")
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
func scriptSafeEscapes() *strings.Replacer {
	return strings.NewReplacer(
		"<", "\\u003c",
		">", "\\u003e",
		"&", "\\u0026",
		" ", "\\u2028",
		" ", "\\u2029",
	)
}

// encodeReport serializes the report for embedding in a script element. A
// commit subject containing "</script>" must not be able to close the tag.
func encodeReport(r *core.Report) (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", core.Internalf(err, "encoding the report")
	}
	return scriptSafeEscapes().Replace(string(data)), nil
}

func pageTitle(r *core.Report, year int) string {
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
		return core.Internalf(err, "writing the temporary output file")
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return core.Internalf(err, "moving the temporary output file into place")
	}
	return nil
}
