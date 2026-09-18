package server

import (
	"sync"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// The tests in this package wire the server the way the command does, with the
// clock and randomness fixed (ADR-0042 clause 5). Each call constructs its own
// dependencies, so no two tests share one.

// steppingClock reads one second later on every call, starting from a fixed
// instant. It moves the way the process clock does, so ordering by time still
// means something, but every run reads the same instants.
type steppingClock struct {
	mu  sync.Mutex
	now time.Time
}

func newSteppingClock() *steppingClock {
	return &steppingClock{now: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)}
}

func (c *steppingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(time.Second)
	return c.now
}

// testAnalyzer is the analysis service with a fixed clock.
func testAnalyzer() *pipeline.Analyzer {
	clock := core.FixedClock(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC))
	files := core.SystemFilesystem()
	return pipeline.New(collect.New(clock, files), aggregate.New(clock, files), files)
}

// testCollector is the collect stage with a fixed clock.
func testCollector() *collect.Collector {
	return collect.New(core.FixedClock(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)), core.SystemFilesystem())
}

// newTestManager constructs a manager, supplying each dependency the test did
// not: a stepping clock, identifiers from a seeded source, and the real
// analysis with a fixed clock.
func newTestManager(options ManagerOptions) *Manager {
	if options.Clock == nil {
		options.Clock = newSteppingClock()
	}
	if options.NewID == nil {
		options.NewID = RandomIDs(core.SeededRandom(1))
	}
	if options.Runner == nil {
		options.Runner = testAnalyzer().Run
	}
	return NewManager(options)
}

// newAppWithRoots constructs the application with the given allowed roots and
// a seeded session secret. A nil manager is replaced by a test manager.
func newAppWithRoots(manager *Manager, roots []string) (*App, error) {
	return newAppWithRandom(manager, core.SeededRandom(2), roots)
}

// newAppWithRandom constructs the application with the given random source.
func newAppWithRandom(manager *Manager, random core.Random, roots []string) (*App, error) {
	if manager == nil {
		manager = newTestManager(ManagerOptions{})
	}
	return NewApp(manager, testCollector(), random, roots)
}

// newApp constructs the application with the working directory as its allowed
// root, failing the test when it cannot.
func newApp(t *testing.T, manager *Manager) *App {
	t.Helper()
	app, err := newAppWithRoots(manager, nil)
	if err != nil {
		t.Fatal(err)
	}
	return app
}
