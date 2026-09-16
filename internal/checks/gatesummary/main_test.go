package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadGoTestCounts(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Package":"p","Test":"TestPass"}`,
		`{"Action":"pass","Package":"p","Test":"TestPass"}`,
		`{"Action":"run","Package":"p","Test":"TestSkip"}`,
		`{"Action":"skip","Package":"p","Test":"TestSkip"}`,
		// A parent is counted through its children.
		`{"Action":"run","Package":"p","Test":"TestParent"}`,
		`{"Action":"run","Package":"p","Test":"TestParent/a"}`,
		`{"Action":"output","Package":"p","Test":"TestParent/a","Output":"a broke\n"}`,
		`{"Action":"fail","Package":"p","Test":"TestParent/a"}`,
		`{"Action":"run","Package":"p","Test":"TestParent/b"}`,
		`{"Action":"pass","Package":"p","Test":"TestParent/b"}`,
		`{"Action":"fail","Package":"p","Test":"TestParent"}`,
		// A parent failing on its own counts once more.
		`{"Action":"run","Package":"p","Test":"TestOwn"}`,
		`{"Action":"run","Package":"p","Test":"TestOwn/a"}`,
		`{"Action":"pass","Package":"p","Test":"TestOwn/a"}`,
		`{"Action":"fail","Package":"p","Test":"TestOwn"}`,
		// Started, never finished.
		`{"Action":"run","Package":"p","Test":"TestCrash"}`,
		`{"Action":"fail","Package":"p"}`,
		// A package that fails to build is one failed check.
		`{"Action":"build-output","ImportPath":"q","Output":"syntax error\n"}`,
		`{"Action":"build-fail","ImportPath":"q"}`,
		`{"Action":"fail","Package":"q"}`,
		// A package without tests is not a check.
		`{"Action":"skip","Package":"r"}`,
	}, "\n")
	var out bytes.Buffer
	c, err := readGoTest(strings.NewReader(stream), &out)
	if err != nil {
		t.Fatal(err)
	}
	want := counts{Run: 8, Passed: 3, Failed: 4, Skipped: 1}
	if c != want {
		t.Errorf("counts = %+v, want %+v", c, want)
	}
	for _, s := range []string{"a broke", "syntax error"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output does not show %q:\n%s", s, out.String())
		}
	}
}

func TestEmptyStepFails(t *testing.T) {
	dir := t.TempDir()
	err := run([]string{"gotest", dir, "empty"}, strings.NewReader(""), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "ran no checks") {
		t.Errorf("a step with no checks returned %v", err)
	}
}

func TestStepAndReport(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gate")
	pass := `{"Action":"pass","Package":"p","Test":"TestA"}`
	if err := run([]string{"gotest", dir, "tests"}, strings.NewReader(pass), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, step := range [][]string{
		{"step", dir, "build", "pass"},
		{"step", dir, "tests", "fail"}, // tests passed, but the step exited non-zero
		{"step", dir, "lint", "fail"},
	} {
		if err := run(step, nil, &bytes.Buffer{}); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	err := run([]string{"report", dir, "fast"}, nil, &out)
	if err == nil || !strings.Contains(err.Error(), "tests, lint") {
		t.Errorf("report returned %v, want a failure naming tests and lint", err)
	}
	total := rowFields(t, out.String(), "total")
	if total != "4 2 2 0" {
		t.Errorf("total row = %q, want run 4, passed 2, failed 2, skipped 0:\n%s", total, out.String())
	}
}

func rowFields(t *testing.T, table, row string) string {
	t.Helper()
	for _, line := range strings.Split(table, "\n") {
		f := strings.Fields(line)
		if len(f) == 5 && f[0] == row {
			return strings.Join(f[1:], " ")
		}
	}
	t.Fatalf("no %s row in:\n%s", row, table)
	return ""
}
