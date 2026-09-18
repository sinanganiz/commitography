package checks

import (
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
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

// newAnalyzerAt is the analysis service wired with the given clock.
func newAnalyzerAt(clock core.Clock) *pipeline.Analyzer {
	files := core.SystemFilesystem()
	return pipeline.New(collect.New(clock, files), aggregate.New(clock, files), files)
}
