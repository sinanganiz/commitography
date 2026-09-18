package pipeline

import (
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// testTime is the instant every test clock in this package reads, so nothing a
// test produces depends on when it ran (ADR-0042 clause 5).
func testTime() time.Time {
	return time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
}

// newCollector is the collect stage wired the way the entry points wire it,
// with the clock fixed.
func newCollector() *collect.Collector {
	return collect.New(core.FixedClock(testTime()), core.SystemFilesystem())
}
