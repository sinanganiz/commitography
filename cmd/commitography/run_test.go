package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// emailShaped matches anything that looks like an email address.
func emailShaped() *regexp.Regexp {
	return regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ADR-0064: fixture %q is missing; the gates generate it with `make fixtures`", name)
	}
	return path
}

// baseOptions is a quiet run with blame skipped, so the tests stay fast and
// produce no output of their own.
func baseOptions(t *testing.T, name string) Options {
	t.Helper()
	return Options{
		RepoPath:  fixture(t, name),
		OutputDir: t.TempDir(),
		NoBlame:   true,
		Quiet:     true,
	}
}

func readAll(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(data)
	}
	return sb.String()
}

func TestRunProducesDashboard(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true

	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	index := filepath.Join(opts.OutputDir, render.IndexFile)
	info, err := os.Stat(index)
	if err != nil {
		t.Fatalf("index.html was not written: %v", err)
	}
	if info.Size() < 10_000 {
		t.Errorf("index.html is only %d bytes; the bundle does not appear to be inlined", info.Size())
	}
	if _, err := os.Stat(filepath.Join(opts.OutputDir, render.ReportFile)); err != nil {
		t.Errorf("report.json was not written: %v", err)
	}
}

func TestCLIAndAnalysisServiceProduceTheSameReport(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true
	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(opts.OutputDir, render.ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	var cliReport core.Report
	if err := json.Unmarshal(data, &cliReport); err != nil {
		t.Fatalf("decode CLI report: %v", err)
	}

	analyzer, _ := composeRun(testEnvironment(), opts)
	serviceResult, err := analyzer.Run(context.Background(), pipeline.Options{
		RepoPath: opts.RepoPath,
		NoBlame:  true,
	}, nil)
	if err != nil {
		t.Fatalf("pipeline.Run: %v", err)
	}

	// Generation time is intentionally different because the CLI renders after
	// the service returns. All measured values must remain identical.
	cliReport.Metadata.GeneratedAt = time.Time{}
	serviceResult.Report.Metadata.GeneratedAt = time.Time{}
	if !reflect.DeepEqual(cliReport, *serviceResult.Report) {
		t.Fatal("CLI and analysis service reports differ")
	}
}

func TestDefaultOutputContainsNoPlaintextEmail(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true
	opts.PerAuthor = true // the route by which addresses once reached the output

	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if match := emailShaped().FindString(readAll(t, opts.OutputDir)); match != "" {
		t.Errorf("output contains an email-shaped string: %q", match)
	}
}

func TestAnonymizeRemovesRealNames(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true
	opts.PerAuthor = true
	opts.Anonymize = true

	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := readAll(t, opts.OutputDir)
	for _, name := range []string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Ada L."} {
		if strings.Contains(out, name) {
			t.Errorf("author name %q survived --anonymize", name)
		}
	}
	if match := emailShaped().FindString(out); match != "" {
		t.Errorf("output contains an email-shaped string: %q", match)
	}
}

func TestJSONOnlySkipsHTML(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true
	opts.JSONOnly = true

	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries, err := os.ReadDir(opts.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != render.ReportFile {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("--json produced %v, want only %s", names, render.ReportFile)
	}
}

func TestShallowCloneIsRefused(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "shallow")
	opts.outputDirSet = true

	err := run(t, opts)
	if err == nil {
		t.Fatal("a shallow clone must be refused")
	}
	if core.ExitCode(err) != core.ExitUser {
		t.Errorf("exit code = %d, want %d", core.ExitCode(err), core.ExitUser)
	}
	if got := core.ReasonOf(err); got != core.ReasonShallowClone {
		t.Fatalf("reason = %q, want %q", got, core.ReasonShallowClone)
	}
	for _, want := range []string{"git fetch --unshallow", "fetch-depth: 0", "--allow-shallow"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal message is missing %q", want)
		}
	}
}

func TestAllowShallowProceeds(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "shallow")
	opts.outputDirSet = true
	opts.AllowShallow = true

	if err := run(t, opts); err != nil {
		t.Fatalf("Run with --allow-shallow: %v", err)
	}
	report, err := os.ReadFile(filepath.Join(opts.OutputDir, render.ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(report), `"shallow_clone"`) {
		t.Error("a shallow run must mark its families degraded with shallow_clone")
	}
}

func TestEmptyAndMissingRepositoriesExitTwo(t *testing.T) {
	t.Parallel()
	empty := baseOptions(t, "empty")
	empty.outputDirSet = true
	if err := run(t, empty); err == nil || core.ExitCode(err) != core.ExitUser {
		t.Errorf("empty repository: err = %v, exit = %d", err, core.ExitCode(err))
	}

	missing := Options{RepoPath: t.TempDir(), OutputDir: t.TempDir(), Quiet: true, outputDirSet: true}
	err := run(t, missing)
	if err == nil || core.ExitCode(err) != core.ExitUser {
		t.Errorf("non-repository path: err = %v, exit = %d", err, core.ExitCode(err))
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unexpected message: %v", err)
	}
}

func TestQuietAndVerboseConflict(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.Verbose = true

	err := run(t, opts)
	if err == nil {
		t.Fatal("--quiet with --verbose must be an error")
	}
	if core.ExitCode(err) != core.ExitUser {
		t.Errorf("exit code = %d, want %d", core.ExitCode(err), core.ExitUser)
	}
	if got := core.ReasonOf(err); got != core.ReasonInvalidInvocation {
		t.Errorf("reason = %q, want %q", got, core.ReasonInvalidInvocation)
	}
	if !strings.Contains(err.Error(), "--quiet") || !strings.Contains(err.Error(), "--verbose") {
		t.Errorf("the message should name both flags, got %q", err.Error())
	}
}

func TestWrappedRefusesThinYears(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true
	opts.Wrapped = 1999

	err := run(t, opts)
	if err == nil {
		t.Fatal("a year with no commits must be refused")
	}
	if core.ExitCode(err) != core.ExitUser {
		t.Errorf("exit code = %d, want %d", core.ExitCode(err), core.ExitUser)
	}
	if got := core.ReasonOf(err); got != core.ReasonYearBelowThreshold {
		t.Errorf("reason = %q, want %q", got, core.ReasonYearBelowThreshold)
	}
	for _, want := range []string{"1999", "0 analysed commits", "needs 10", "Choose a year"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q is missing %q", err.Error(), want)
		}
	}
}

func TestWrappedProducesItsOwnPage(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "basic")
	opts.outputDirSet = true
	opts.Wrapped = 2026

	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	path := filepath.Join(opts.OutputDir, render.WrappedFileName(2026))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("wrapped page was not written: %v", err)
	}
	// The dashboard is not produced as a side effect of asking for wrapped.
	if _, err := os.Stat(filepath.Join(opts.OutputDir, render.IndexFile)); err == nil {
		t.Error("--wrapped should not also write index.html")
	}
}

func TestBotsAreExcludedFromOutput(t *testing.T) {
	t.Parallel()
	opts := baseOptions(t, "bots")
	opts.outputDirSet = true

	if err := run(t, opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	report, err := os.ReadFile(filepath.Join(opts.OutputDir, render.ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(report), "dependabot") || strings.Contains(string(report), "renovate") {
		t.Error("a bot identity leaked into the report")
	}
}
