package git

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The recording program the argument-vector and cancellation checkers run git
// through.
//
// ADR-0065 clause 2 is about what reaches the git process, so the checker for
// it reads what reached a process rather than what the source says is sent.
// This test binary doubles as that process: copied to a file named git, it
// recognises itself by its own name, writes the argument vector and the
// environment it was started with, and then runs the real git so the
// invocation still produces the output its caller parses.
//
// Nothing outside this package's tests can reach it: the recording program is
// selected through Spec.binary, which is unexported, so no call site can
// point an invocation anywhere but at git.

// shimRecord is one captured invocation.
type shimRecord struct {
	Args []string `json:"args"`
	Env  []string `json:"env"`
	PID  int      `json:"pid"`
}

const (
	// shimRecordFile holds the captured invocations, one JSON object per NUL
	// -delimited record, beside the recording program.
	shimRecordFile = "invocations"

	// shimChildFile holds the process identifier of each real git the
	// recording program started, one per NUL-delimited record. It is what
	// makes the descendant of ADR-0044 clause 4 nameable from outside.
	shimChildFile = "children"

	// shimTargetFile holds the path of the real git, beside the recording
	// program. It is a file rather than a variable because the environment is
	// the thing under test: a sanitised environment must not have to carry
	// anything for the recorder to work.
	shimTargetFile = "target"
)

// TestMain runs the recording program when this binary has been copied to a
// file named git, and the tests otherwise.
func TestMain(m *testing.M) {
	if name := filepath.Base(os.Args[0]); name == "git" || name == "git.exe" {
		os.Exit(runShim())
	}
	os.Exit(m.Run())
}

// runShim records the invocation and hands it to the real git.
func runShim() int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shim: locating itself:", err)
		return 127
	}
	dir := filepath.Dir(self)

	record, err := json.Marshal(shimRecord{Args: os.Args[1:], Env: os.Environ(), PID: os.Getpid()})
	if err != nil {
		fmt.Fprintln(os.Stderr, "shim: encoding the invocation:", err)
		return 127
	}
	if err := appendRecord(filepath.Join(dir, shimRecordFile), record); err != nil {
		fmt.Fprintln(os.Stderr, "shim: recording the invocation:", err)
		return 127
	}

	target, err := os.ReadFile(filepath.Join(dir, shimTargetFile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "shim: locating git:", err)
		return 127
	}
	cmd := exec.Command(strings.TrimSpace(string(target)), os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "shim: running git:", err)
		return 127
	}
	if err := appendRecord(filepath.Join(dir, shimChildFile),
		[]byte(strconv.Itoa(cmd.Process.Pid))); err != nil {
		fmt.Fprintln(os.Stderr, "shim: recording the descendant:", err)
		return 127
	}
	if err := cmd.Wait(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "shim: running git:", err)
		return 127
	}
	return 0
}

// appendRecord adds one NUL-terminated record to a file. O_APPEND keeps the
// concurrent invocations a sharded history read produces from interleaving.
func appendRecord(path string, record []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(record, 0)); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// recorder is a directory holding the recording program and what it captured.
type recorder struct {
	dir     string
	program string
}

// newRecorder copies this test binary to a file named git and points it at
// the real one.
func newRecorder(t *testing.T) recorder {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("locating the test binary: %v", err)
	}
	actual, err := LookPath()
	if err != nil {
		t.Fatalf("ADR-0064: git is not on the path, so the invocation cannot be recorded: %v", err)
	}

	dir := t.TempDir()
	name := "git"
	if filepath.Ext(self) == ".exe" {
		name = "git.exe"
	}
	source, err := os.Open(self)
	if err != nil {
		t.Fatalf("reading the test binary: %v", err)
	}
	defer source.Close()
	destination, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		t.Fatalf("writing the recording program: %v", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		t.Fatalf("writing the recording program: %v", err)
	}
	if err := destination.Close(); err != nil {
		t.Fatalf("writing the recording program: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, shimTargetFile), []byte(actual), 0o600); err != nil {
		t.Fatalf("writing the recording program's target: %v", err)
	}
	return recorder{dir: dir, program: filepath.Join(dir, name)}
}

// spec points an invocation at the recording program.
func (r recorder) spec(s Spec) Spec {
	s.binary = r.program
	return s
}

// invocations returns what the recording program captured, in order.
func (r recorder) invocations(t *testing.T) []shimRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(r.dir, shimRecordFile))
	if err != nil {
		t.Fatalf("ADR-0064: nothing was recorded, so the checker cannot fail: %v", err)
	}
	var records []shimRecord
	for _, chunk := range splitRecordFile(string(data)) {
		var record shimRecord
		if err := json.Unmarshal([]byte(chunk), &record); err != nil {
			t.Fatalf("reading a recorded invocation: %v", err)
		}
		records = append(records, record)
	}
	return records
}

// processes returns every process identifier the recording program is known
// to have put into the invocation's group: its own and the real git's.
func (r recorder) processes() []int {
	var pids []int
	for _, file := range []string{shimRecordFile, shimChildFile} {
		data, err := os.ReadFile(filepath.Join(r.dir, file))
		if err != nil {
			continue
		}
		for _, chunk := range splitRecordFile(string(data)) {
			if pid, err := strconv.Atoi(chunk); err == nil {
				pids = append(pids, pid)
				continue
			}
			var record shimRecord
			if err := json.Unmarshal([]byte(chunk), &record); err == nil && record.PID != 0 {
				pids = append(pids, record.PID)
			}
		}
	}
	return pids
}

func splitRecordFile(data string) []string {
	var out []string
	for _, chunk := range strings.Split(data, "\x00") {
		if strings.TrimSpace(chunk) != "" {
			out = append(out, chunk)
		}
	}
	return out
}
