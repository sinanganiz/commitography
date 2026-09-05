// Package cli implements the commitography command.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// stageWidth is the column the detail text starts at, so the dotted leaders
// line up whatever the stage is called.
const stageWidth = 24

// Progress reports what the run is doing. Everything it writes goes to stderr,
// never stdout, so a future stdout mode can be piped without contamination.
type Progress struct {
	out     io.Writer
	quiet   bool
	verbose bool
	tty     bool
	started time.Time
	// open records whether an in-place line is waiting to be overwritten.
	open bool
}

// NewProgress builds a reporter writing to stderr.
func NewProgress(quiet, verbose bool) *Progress {
	return &Progress{
		out:     os.Stderr,
		quiet:   quiet,
		verbose: verbose,
		tty:     isTerminal(os.Stderr),
		started: time.Now(),
	}
}

// isTerminal reports whether the stream is a character device, which is close
// enough to "a human is watching" for deciding between in-place updates and
// plain lines. Using os.Stat keeps the dependency list at three.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Stage prints one line of the run log, updating in place on a terminal and
// appending line by line when stderr is redirected.
func (p *Progress) Stage(stage, detail string) {
	if p == nil || p.quiet {
		return
	}
	leader := stage + " " + strings.Repeat(".", max(3, stageWidth-len(stage)))
	line := fmt.Sprintf("%s %s", leader, detail)

	if p.tty {
		fmt.Fprintf(p.out, "\r\033[K%s", line)
		p.open = true
		return
	}
	fmt.Fprintln(p.out, line)
}

// Done closes the log with the total elapsed time.
func (p *Progress) Done(detail string) {
	if p == nil || p.quiet {
		return
	}
	p.endLine()
	fmt.Fprintf(p.out, "Done in %.1fs — %s\n", time.Since(p.started).Seconds(), detail)
}

// Warn reports a non-fatal problem. Warnings survive --quiet only in the sense
// that they are also carried into the report; on a quiet run nothing is printed.
func (p *Progress) Warn(format string, args ...any) {
	if p == nil || p.quiet {
		return
	}
	p.endLine()
	fmt.Fprintf(p.out, "warning: "+format+"\n", args...)
}

// Debug prints only under --verbose.
func (p *Progress) Debug(format string, args ...any) {
	if p == nil || p.quiet || !p.verbose {
		return
	}
	p.endLine()
	fmt.Fprintf(p.out, "debug: "+format+"\n", args...)
}

// endLine terminates an in-place line before writing something that must
// persist above it.
func (p *Progress) endLine() {
	if p.open {
		fmt.Fprintln(p.out)
		p.open = false
	}
}
