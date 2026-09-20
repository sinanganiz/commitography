// Command gitcount is the recording program TestSubprocessCount puts in front
// of git. Built to a file named git and placed first on the analysed
// process's path, it appends one record per invocation and then runs the real
// git, so the analysis behaves exactly as it would otherwise and the number
// of git processes it started can be counted from outside it.
//
// Process execution here is permitted by ADR-0065 clause 3: this is part of
// internal/checks, it runs the built binary's git with a fixed argument
// vector, through no shell (ADR-0065 clause 4). The timeout is the caller's:
// the measurement runs under `go test`, whose own bound stops the run, and
// adding one here would cap an invocation the analysis under measurement is
// entitled to make.
//
// The real git's path comes from a file beside this program rather than from
// the environment, because the environment is sanitised on the way in
// (internal/git) and a recorder that needed a variable to survive that would
// be measuring its own exception.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// countFile holds one record per invocation.
	countFile = "invocations"

	// targetFile holds the path of the real git.
	targetFile = "target"
)

func main() { os.Exit(run()) }

func run() int {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitcount: locating itself:", err)
		return 127
	}
	dir := filepath.Dir(self)

	// One NUL-terminated record per invocation, appended: concurrent
	// invocations, which a sharded history read produces, do not interleave
	// under O_APPEND.
	record := append([]byte(strings.Join(os.Args[1:], " ")), 0)
	file, err := os.OpenFile(filepath.Join(dir, countFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitcount: recording the invocation:", err)
		return 127
	}
	if _, err := file.Write(record); err != nil {
		_ = file.Close()
		fmt.Fprintln(os.Stderr, "gitcount: recording the invocation:", err)
		return 127
	}
	if err := file.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "gitcount: recording the invocation:", err)
		return 127
	}

	target, err := os.ReadFile(filepath.Join(dir, targetFile))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitcount: locating git:", err)
		return 127
	}
	cmd := exec.Command(strings.TrimSpace(string(target)), os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "gitcount: running git:", err)
		return 127
	}
	return 0
}
