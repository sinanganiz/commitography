// Command gatesummary records and prints what a gate executed: the number of
// checks run, passed, failed and skipped (ADR-0064 clause 4).
//
// It reads results only; the Makefile runs every command. Subcommands:
//
//	gotest <dir> <step>       read `go test -json` from stdin, print failures,
//	                          record one check per test
//	vitest <dir> <step> <f>   record the checks in a vitest JSON report
//	step <dir> <step> <pass|fail>
//	                          record a step's exit status; a step that recorded
//	                          no counts is one check, and a failed step whose
//	                          counts show no failure gains one
//	report <dir> <gate>       print the table; fail if anything failed
//
// A test step that ran no test fails: a gate that executed nothing has not
// shown anything (ADR-0064 clause 1).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// counts is one step's record.
type counts struct {
	Step    string `json:"step"`
	Run     int    `json:"run"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
}

func (c *counts) add(result string) {
	c.Run++
	switch result {
	case "pass":
		c.Passed++
	case "skip":
		c.Skipped++
	default:
		c.Failed++
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gatesummary:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) < 3 {
		return errors.New("usage: gatesummary gotest|vitest|step|report <dir> <name> [...]")
	}
	cmd, dir, name := args[0], args[1], args[2]
	switch cmd {
	case "gotest":
		c, err := readGoTest(stdin, stdout)
		if err != nil {
			return err
		}
		c.Step = name
		if err := save(dir, c); err != nil {
			return err
		}
		return verdict(c)
	case "vitest":
		if len(args) != 4 {
			return errors.New("usage: gatesummary vitest <dir> <step> <report.json>")
		}
		c, err := readVitest(args[3])
		if err != nil {
			return err
		}
		c.Step = name
		if err := save(dir, c); err != nil {
			return err
		}
		return verdict(c)
	case "step":
		if len(args) != 4 {
			return errors.New("usage: gatesummary step <dir> <step> pass|fail")
		}
		return recordStep(dir, name, args[3])
	case "report":
		return report(dir, name, stdout)
	}
	return fmt.Errorf("unknown subcommand %q", cmd)
}

func verdict(c counts) error {
	switch {
	case c.Run == 0:
		return fmt.Errorf("ADR-0064: step %s ran no checks", c.Step)
	case c.Failed > 0:
		return fmt.Errorf("step %s: %d of %d checks failed", c.Step, c.Failed, c.Run)
	}
	return nil
}

// event is the subset of a test2json record this command reads.
type event struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Output     string
}

type testResult struct {
	result string
	output []string
}

type packageResult struct {
	result string
	output []string
	tests  map[string]*testResult
	order  []string
}

// readGoTest counts leaf tests, since a parent's result is its children's.
// A parent that fails while every child passed failed on its own and counts
// once more. A package that fails outside any test, such as a build error, a
// panic or a failing TestMain, counts as one failed check.
func readGoTest(r io.Reader, w io.Writer) (counts, error) {
	packages := map[string]*packageResult{}
	var packageOrder []string
	pkg := func(name string) *packageResult {
		p, ok := packages[name]
		if !ok {
			p = &packageResult{tests: map[string]*testResult{}}
			packages[name] = p
			packageOrder = append(packageOrder, name)
		}
		return p
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var e event
		if err := json.Unmarshal(line, &e); err != nil || e.Action == "" {
			// Not a test2json record: go itself reporting a problem.
			fmt.Fprintln(w, string(line))
			continue
		}
		switch e.Action {
		case "build-output":
			p := pkg(e.ImportPath)
			p.output = append(p.output, e.Output)
			continue
		case "build-fail":
			pkg(e.ImportPath).result = "fail"
			continue
		}
		p := pkg(e.Package)
		if e.Test == "" {
			switch e.Action {
			case "output":
				p.output = append(p.output, e.Output)
			case "pass", "fail", "skip":
				p.result = e.Action
			}
			continue
		}
		t, ok := p.tests[e.Test]
		if !ok {
			t = &testResult{}
			p.tests[e.Test] = t
			p.order = append(p.order, e.Test)
		}
		switch e.Action {
		case "output":
			t.output = append(t.output, e.Output)
		case "pass", "fail", "skip":
			t.result = e.Action
		}
	}
	if err := scanner.Err(); err != nil {
		return counts{}, err
	}

	var c counts
	for _, name := range packageOrder {
		p := packages[name]
		pc := countPackage(p, w)
		if p.result == "fail" && pc.Failed == 0 {
			fmt.Fprint(w, strings.Join(p.output, ""))
			pc.add("fail")
		}
		switch {
		case p.result == "fail" || pc.Failed > 0:
			fmt.Fprintf(w, "FAIL\t%s\t%d checks, %d failed\n", name, pc.Run, pc.Failed)
		case pc.Run > 0:
			fmt.Fprintf(w, "ok  \t%s\t%d checks, %d skipped\n", name, pc.Run, pc.Skipped)
		}
		c.Run += pc.Run
		c.Passed += pc.Passed
		c.Failed += pc.Failed
		c.Skipped += pc.Skipped
	}
	return c, nil
}

func countPackage(p *packageResult, w io.Writer) counts {
	var c counts
	for _, name := range p.order {
		t := p.tests[name]
		result := t.result
		if result == "" {
			// Started and never finished: the binary died underneath it.
			result = "fail"
		}
		leaf, childFailed := true, false
		for _, other := range p.order {
			if strings.HasPrefix(other, name+"/") {
				leaf = false
				if r := p.tests[other].result; r == "fail" || r == "" {
					childFailed = true
				}
			}
		}
		if !leaf && (result != "fail" || childFailed) {
			continue
		}
		switch result {
		case "fail":
			fmt.Fprint(w, strings.Join(t.output, ""))
		case "skip":
			// A skip must state its reason (ADR-0064 clause 3); the summary
			// shows it so that every skip is visible where the gate reports.
			for _, line := range t.output {
				if !strings.HasPrefix(line, "=== ") {
					fmt.Fprint(w, line)
				}
			}
		}
		c.add(result)
	}
	return c
}

// vitestReport is the subset of vitest's JSON reporter output this command
// reads.
type vitestReport struct {
	NumTotalTests   int `json:"numTotalTests"`
	NumPassedTests  int `json:"numPassedTests"`
	NumFailedTests  int `json:"numFailedTests"`
	NumPendingTests int `json:"numPendingTests"`
	NumTodoTests    int `json:"numTodoTests"`
}

func readVitest(path string) (counts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return counts{}, fmt.Errorf("ADR-0064: the vitest report is missing: %w", err)
	}
	var v vitestReport
	if err := json.Unmarshal(data, &v); err != nil {
		return counts{}, fmt.Errorf("reading %s: %w", path, err)
	}
	c := counts{
		Run:     v.NumTotalTests,
		Passed:  v.NumPassedTests,
		Failed:  v.NumFailedTests,
		Skipped: v.NumPendingTests + v.NumTodoTests,
	}
	if c.Passed+c.Failed+c.Skipped != c.Run {
		return counts{}, fmt.Errorf("%s: %d tests but %d passed, %d failed, %d skipped", path, c.Run, c.Passed, c.Failed, c.Skipped)
	}
	return c, nil
}

func countsPath(dir, step string) string {
	return filepath.Join(dir, step+".json")
}

func save(dir string, c counts) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(countsPath(dir, c.Step), append(data, '\n'), 0o644)
}

func load(dir, step string) (counts, bool, error) {
	data, err := os.ReadFile(countsPath(dir, step))
	if errors.Is(err, os.ErrNotExist) {
		return counts{}, false, nil
	}
	if err != nil {
		return counts{}, false, err
	}
	var c counts
	if err := json.Unmarshal(data, &c); err != nil {
		return counts{}, false, fmt.Errorf("reading counts for %s: %w", step, err)
	}
	return c, true, nil
}

const orderFile = "steps.txt"

func recordStep(dir, step, status string) error {
	if status != "pass" && status != "fail" {
		return fmt.Errorf("step status %q is neither pass nor fail", status)
	}
	c, found, err := load(dir, step)
	if err != nil {
		return err
	}
	switch {
	case !found:
		c = counts{Step: step}
		c.add(status)
	case status == "fail" && c.Failed == 0:
		// The step exited non-zero after its tests passed, or before
		// recording anything that failed.
		c.add("fail")
	}
	if err := save(dir, c); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, orderFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, step); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func report(dir, gate string, w io.Writer) error {
	data, err := os.ReadFile(filepath.Join(dir, orderFile))
	if err != nil {
		return fmt.Errorf("ADR-0064: gate %s recorded no steps: %w", gate, err)
	}
	var steps []string
	for _, s := range strings.Split(strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n")), "\n") {
		if s != "" {
			steps = append(steps, s)
		}
	}
	width := len("total")
	for _, s := range steps {
		width = max(width, len(s))
	}
	var total counts
	var failed []string
	fmt.Fprintf(w, "\n%s gate summary\n", gate)
	fmt.Fprintf(w, "  %-*s %7s %7s %7s %7s\n", width, "step", "run", "passed", "failed", "skipped")
	for _, s := range steps {
		c, found, err := load(dir, s)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("ADR-0064: step %s recorded no counts", s)
		}
		fmt.Fprintf(w, "  %-*s %7d %7d %7d %7d\n", width, s, c.Run, c.Passed, c.Failed, c.Skipped)
		total.Run += c.Run
		total.Passed += c.Passed
		total.Failed += c.Failed
		total.Skipped += c.Skipped
		if c.Failed > 0 {
			failed = append(failed, s)
		}
	}
	fmt.Fprintf(w, "  %-*s %7d %7d %7d %7d\n", width, "total", total.Run, total.Passed, total.Failed, total.Skipped)
	if len(failed) > 0 {
		return fmt.Errorf("gate %s failed: %s", gate, strings.Join(failed, ", "))
	}
	return nil
}
