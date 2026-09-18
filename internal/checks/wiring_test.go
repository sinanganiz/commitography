package checks

import (
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/server"
)

// checkTime is the instant the checkers' clocks read, so nothing a checker
// produces depends on when it ran (ADR-0042 clause 5).
func checkTime() time.Time {
	return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
}

// newCollector is the collect stage wired the way the entry points wire it,
// with the clock fixed.
func newCollector() *collect.Collector {
	return collect.New(core.FixedClock(checkTime()), core.SystemFilesystem())
}

// newAnalyzer is the analysis service wired the way the entry points wire it,
// with the clock fixed.
func newAnalyzer() *pipeline.Analyzer {
	return newAnalyzerAt(core.FixedClock(checkTime()))
}

// newApp is the local application wired the way the server command wires it,
// with the clock fixed and the randomness seeded, allowing the given roots.
func newApp(roots []string) (*server.App, error) {
	clock, files := core.FixedClock(checkTime()), core.SystemFilesystem()
	collector := collect.New(clock, files)
	manager := server.NewManager(server.ManagerOptions{
		Clock:  clock,
		NewID:  server.RandomIDs(core.SeededRandom(1)),
		Runner: pipeline.New(collector, aggregate.New(clock, files), files).Run,
	})
	return server.NewApp(manager, collector, core.SeededRandom(2), roots)
}

// newAnalyzerAt is the analysis service wired with the given clock.
func newAnalyzerAt(clock core.Clock) *pipeline.Analyzer {
	files := core.SystemFilesystem()
	return pipeline.New(collect.New(clock, files), aggregate.New(clock, files), files)
}
