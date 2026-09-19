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
//
// The limits belong to the analysis configuration plane, so a report states
// which limits produced it and they enter the cache key (ADR-0053 clause 6,
// ADR-0026). They are not operator-settable: a configuration that supplies one
// is verified against this catalogue and refused on a mismatch, and nothing
// ever applies a supplied value (ADR-0062 clause 6).

package core

import (
	"fmt"
	"sort"
)

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

// NamedLimit is one cardinality limit under the name the analysis
// configuration gives it.
type NamedLimit struct {
	Name  string
	Value int
}

// NamedLimits returns every limit by its configuration name, in section 12
// order. The coupling graph's row carries two limits and has two names.
func NamedLimits() []NamedLimit {
	return []NamedLimit{
		{"identities", LimitIdentities},
		{"most_modified_files", LimitMostModifiedFiles},
		{"extensions", LimitExtensions},
		{"directory_activity_entries", LimitDirectoryActivity},
		{"coupling_pairs", LimitCouplingPairs},
		{"coupling_graph_nodes", LimitCouplingGraphNodes},
		{"coupling_graph_edges", LimitCouplingGraphEdges},
		{"identities_per_directory", LimitIdentitiesPerDirectory},
		{"hotspot_entries", LimitHotspots},
		{"churn_files", LimitChurnFiles},
		{"bulk_commits_listed", LimitBulkCommits},
	}
}

// LimitValues returns every limit by its configuration name: the resolved
// value of the analysis plane's cardinality limits, whatever a configuration
// supplied.
func LimitValues() map[string]int {
	out := make(map[string]int, len(NamedLimits()))
	for _, l := range NamedLimits() {
		out[l.Name] = l.Value
	}
	return out
}

// VerifyCardinalityLimits checks the limits a configuration supplied against
// the catalogue. Every supplied name must be a limit section 12 defines and
// every supplied value must equal the catalogue's; a limit left out is not a
// mismatch. The error names the first offending limit, in name order, with no
// path, so it can serve as the offending value of an invalid_configuration
// refusal.
func VerifyCardinalityLimits(supplied map[string]int) error {
	names := make([]string, 0, len(supplied))
	for name := range supplied {
		names = append(names, name)
	}
	sort.Strings(names)
	catalogue := LimitValues()
	for _, name := range names {
		want, known := catalogue[name]
		if !known {
			return fmt.Errorf("cardinality_limits.%s is not a limit docs/metrics.md section 12 defines", name)
		}
		if got := supplied[name]; got != want {
			return fmt.Errorf("cardinality_limits.%s is %d, and docs/metrics.md section 12 fixes it at %d; "+
				"limits are verified, never set", name, got, want)
		}
	}
	return nil
}
