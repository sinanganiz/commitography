// The family registry (ADR-0076 clauses 1 and 6): the one list of the metric
// families the report carries, in the order the report writes them. The
// family declaration checker in internal/checks reads it rather than holding
// a list of its own, so no second list exists to disagree with this one.

package aggregate

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/metrics/aiarchaeology"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/coupling"
	"github.com/sinanganiz/commitography/internal/metrics/files"
	"github.com/sinanganiz/commitography/internal/metrics/hotspot"
	"github.com/sinanganiz/commitography/internal/metrics/messages"
	"github.com/sinanganiz/commitography/internal/metrics/ownership"
	"github.com/sinanganiz/commitography/internal/metrics/staticanalysis"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
	"github.com/sinanganiz/commitography/internal/metrics/worktype"
)

// entry is one family's registration.
type entry struct {
	family core.MetricFamily
}

// entries returns every registration, in the order the report writes the
// families (ADR-0076 clause 6).
func entries() []entry {
	return []entry{
		{family: temporal.Family{}},
		{family: commitsize.Family{}},
		{family: messages.Family{}},
		{family: files.Family{}},
		{family: coupling.Family{}},
		{family: ownership.Family{}},
		{family: worktype.Family{}},
		{family: aiarchaeology.Family{}},
		{family: hotspot.Family{}},
		{family: staticanalysis.Family{}},
	}
}

// Registry returns every metric family the report carries, in the order the
// report writes them. It is the only list of families in the tree.
func Registry() []core.MetricFamily {
	registered := entries()
	out := make([]core.MetricFamily, 0, len(registered))
	for _, e := range registered {
		out = append(out, e.family)
	}
	return out
}
