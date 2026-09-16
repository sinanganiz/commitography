package aggregate

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/metrics/coupling"
	"github.com/sinanganiz/commitography/internal/metrics/hotspot"
	"github.com/sinanganiz/commitography/internal/metrics/ownership"
)

// buildSocial assembles the social section from the ownership, coupling and
// hotspot families.
func buildSocial(in core.Input, commits []model.Commit) (core.SocialMetrics, []string) {
	var warnings []string
	m := core.SocialMetrics{
		DirectoryBusFactor:     []core.DirectoryBusFactor{},
		Coupling:               []core.CoupledPair{},
		Churn:                  []core.ChurnHotspot{},
		KnowledgeConcentration: []core.KnowledgeShare{},
	}

	scoped := core.ScopedCommits(in, commits)

	ownership.BuildOwnership(in, scoped, &m)

	pairs, couplingWarnings := coupling.BuildCoupling(scoped)
	m.Coupling = pairs
	warnings = append(warnings, couplingWarnings...)

	m.Churn = hotspot.BuildChurn(scoped)

	return m, warnings
}
