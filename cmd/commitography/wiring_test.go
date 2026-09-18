package main

import (
	"io"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

// testEnvironment is the process environment the command's tests compose
// with: a fixed clock, seeded randomness, and discarded output. Each call
// constructs its own, so no two tests share one (ADR-0042 clause 5).
func testEnvironment() environment {
	return environment{
		build:  buildInfo{version: "test", commit: "test", date: "test"},
		clock:  core.FixedClock(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)),
		random: core.SeededRandom(1),
		files:  core.SystemFilesystem(),
		stdout: io.Discard,
		stderr: io.Discard,
	}
}

// run performs one analysis wired the way the command wires it.
func run(t *testing.T, opts Options) error {
	t.Helper()
	analyzer, logger := composeRun(testEnvironment(), opts)
	return Run(opts, analyzer, logger)
}
