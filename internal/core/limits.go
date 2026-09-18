// The cardinality limits of the report (ADR-0053 clause 3).
//
// `docs/metrics.md` section 12 is authoritative: a limit absent from it does
// not exist (ADR-0062 clause 6). This file is the code's copy of that section,
// and TestMetricCatalogueLimits in internal/checks compares the two in both
// directions. A family takes its limits from here and declares none of its
// own; the same checker fails on a limit constant declared under
// internal/metrics.
//
// Exceeding a limit marks the owning family degraded with reason
// cardinality_limit.

package core

import "fmt"

// The limits, one per row of section 12.
const (
	LimitIdentities             = 200
	LimitMostModifiedFiles      = 25
	LimitExtensions             = 20
	LimitDirectoryActivity      = 50
	LimitCouplingPairs          = 200
	LimitCouplingGraphNodes     = 150
	LimitCouplingGraphEdges     = 400
	LimitIdentitiesPerDirectory = 10
	LimitHotspots               = 100
	LimitChurnFiles             = 100
	LimitBulkCommits            = 10
)

// CardinalityLimit is one row of section 12, with its label and its value as
// the table spells them.
type CardinalityLimit struct {
	Label string
	Value string
}

// CardinalityLimits returns every limit as a section 12 row, for the checker
// to compare with the catalogue.
func CardinalityLimits() []CardinalityLimit {
	return []CardinalityLimit{
		{"Individually represented identities", fmt.Sprint(LimitIdentities)},
		{"Most modified files", fmt.Sprint(LimitMostModifiedFiles)},
		{"Extensions", fmt.Sprint(LimitExtensions)},
		{"Directory activity entries", fmt.Sprint(LimitDirectoryActivity)},
		{"Coupling pairs", fmt.Sprint(LimitCouplingPairs)},
		{"Coupling graph nodes / edges", fmt.Sprintf("%d / %d", LimitCouplingGraphNodes, LimitCouplingGraphEdges)},
		{"Identities per directory in `directory_ownership`", fmt.Sprint(LimitIdentitiesPerDirectory)},
		{"Hotspot entries", fmt.Sprint(LimitHotspots)},
		{"Churn files", fmt.Sprint(LimitChurnFiles)},
		{"Bulk commits listed", fmt.Sprint(LimitBulkCommits)},
	}
}
