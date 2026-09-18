package core

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// stageWidth is the column the detail text starts at, so the dotted leaders
// line up whatever the stage is called.
const stageWidth = 24

// Logger writes the interactive diagnostics a run prints: progress, warnings
// and debug lines. Both entry points send it to standard error, never standard
// output, so output can be piped without contamination (ADR-0034 clause 4).
//
// It is constructed at composition with its stream and clock and passed to
// whatever reports through it; there is no package-level log sink (ADR-0042
// clause 2). A checker can therefore construct one over a buffer and scan what
// it wrote, which is how the leak scan reaches log output (ADR-0067 clause 6).
type Logger struct {
	out     io.Writer
	clock   Clock
	quiet   bool
	verbose bool
	// terminal selects in-place progress updates over appended lines.
	terminal bool
	started  time.Time
	// open records whether an in-place line is waiting to be overwritten.
	open bool
}

// LoggerOptions selects what a Logger prints and how.
type LoggerOptions struct {
	// Quiet suppresses everything; Verbose adds debug lines.
	Quiet, Verbose bool
	// Terminal reports whether the stream is a terminal, which the caller
	// determines once at composition.
	Terminal bool
}

// NewLogger constructs a logger writing to out. The clock times the run from
// this moment, for the closing line.
func NewLogger(out io.Writer, clock Clock, options LoggerOptions) *Logger {
	return &Logger{
		out:      out,
		clock:    clock,
		quiet:    options.Quiet,
		verbose:  options.Verbose,
		terminal: options.Terminal,
		started:  clock.Now(),
	}
}

// Stage prints one line of the run log, updating in place on a terminal and
// appending line by line when the stream is redirected.
func (l *Logger) Stage(stage, detail string) {
	if l == nil || l.quiet {
		return
	}
	leader := stage + " " + strings.Repeat(".", max(3, stageWidth-len(stage)))
	line := fmt.Sprintf("%s %s", leader, detail)

	if l.terminal {
		fmt.Fprintf(l.out, "\r\033[K%s", line)
		l.open = true
		return
	}
	fmt.Fprintln(l.out, line)
}

// Done closes the log with the total elapsed time.
func (l *Logger) Done(detail string) {
	if l == nil || l.quiet {
		return
	}
	l.endLine()
	fmt.Fprintf(l.out, "Done in %.1fs — %s\n", l.clock.Now().Sub(l.started).Seconds(), detail)
}

// Warn reports a non-fatal problem. Warnings survive quiet only in the sense
// that they are also carried into the report; on a quiet run nothing is
// printed.
func (l *Logger) Warn(format string, args ...any) {
	if l == nil || l.quiet {
		return
	}
	l.endLine()
	fmt.Fprintf(l.out, "warning: "+format+"\n", args...)
}

// Debug prints only when verbose.
func (l *Logger) Debug(format string, args ...any) {
	if l == nil || l.quiet || !l.verbose {
		return
	}
	l.endLine()
	fmt.Fprintf(l.out, "debug: "+format+"\n", args...)
}

// endLine terminates an in-place line before writing something that must
// persist above it.
func (l *Logger) endLine() {
	if l.open {
		fmt.Fprintln(l.out)
		l.open = false
	}
}
