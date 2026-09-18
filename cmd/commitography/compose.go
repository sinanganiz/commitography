package main

import (
	"io"
	"os"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/server"
)

// This file is where every dependency is constructed and passed (ADR-0042
// clause 1). There are two composition locations, one per entry point:
// composeRun for the analysis command and composeServe for the server. Nothing
// else in the tree constructs a dependency it holds, and nothing else reads
// the process's ambient state; systemEnvironment reads it once, here.

// environment is the process's ambient state and link-time metadata, read once
// by main. Tests construct their own with a fixed clock and randomness.
type environment struct {
	build  buildInfo
	clock  core.Clock
	random core.Random
	files  core.Filesystem
	stdout io.Writer
	stderr io.Writer
	// stderrTerminal selects in-place progress lines.
	stderrTerminal bool
	// inContainer selects the container-specific hints and announcements.
	inContainer bool
}

// systemEnvironment reads the process's ambient state. It is called once, by
// main.
func systemEnvironment() environment {
	version, commit, date := core.BuildMetadata()
	files := core.SystemFilesystem()
	return environment{
		build:          buildInfo{version: version, commit: commit, date: date},
		clock:          core.SystemClock(),
		random:         core.SystemRandom(),
		files:          files,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		stderrTerminal: isTerminal(os.Stderr),
		inContainer:    server.Running(files),
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

// composeRun wires the analysis command: the analysis service and the logger
// its progress and warnings go to.
func composeRun(env environment, opts Options) (*pipeline.Analyzer, *core.Logger) {
	analyzer := pipeline.New(
		collect.New(env.clock, env.files),
		aggregate.New(env.clock, env.files),
		env.files,
	)
	logger := core.NewLogger(env.stderr, env.clock, core.LoggerOptions{
		Quiet:    opts.Quiet,
		Verbose:  opts.Verbose,
		Terminal: env.stderrTerminal,
	})
	return analyzer, logger
}

// composeServe wires the server: the analysis service, the job manager that
// runs it, the application around both, and the listener's streams. It
// returns the application and the listener options completed with it.
func composeServe(env environment, options server.Options) (*server.App, server.Options, *core.Logger, error) {
	collector := collect.New(env.clock, env.files)
	analyzer := pipeline.New(collector, aggregate.New(env.clock, env.files), env.files)
	manager := server.NewManager(server.ManagerOptions{
		Clock:       env.clock,
		NewID:       server.RandomIDs(env.random),
		Runner:      analyzer.Run,
		ToolVersion: env.build.version,
	})
	app, err := server.NewApp(manager, collector, env.random, options.AllowedRoots)
	if err != nil {
		return nil, options, nil, err
	}
	app.AllowListenHost(options.ListenAddress)

	inContainer := env.inContainer
	options.Handler = app.Handler()
	options.OnShutdown = app.Jobs.CancelAll
	options.Output = env.stdout
	options.Errors = env.stderr
	options.InContainer = func() bool { return inContainer }
	logger := core.NewLogger(env.stderr, env.clock, core.LoggerOptions{Terminal: env.stderrTerminal})
	return app, options, logger, nil
}
