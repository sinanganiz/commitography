// Package core holds what every other package may depend on: the report
// types, the reason codes and cardinality limits, and the shared definitions
// the metric families compute over (ADR-0040 clause 1). The report types are the snapshot
// contract of ADR-0021 and are versioned under ADR-0031. Configuration,
// identity, the domain model and filtering live in subpackages beneath it
// (ADR-0066 clause 2).
//
// The report document follows docs/metrics.md, which is authoritative
// (ADR-0062): no field exists in it that the catalogue does not define, every
// family of ADR-0024 clause 5 is present with a status (ADR-0032 clause 1),
// and everything that varies between two runs over one commit is confined to
// the metadata section (ADR-0021 clause 6). docs/report-schema.json describes
// it.
package core

import "time"

// DocumentVersion is the version of the report's structure: its top-level
// shape, status fields and metadata section (ADR-0031 clause 1). A minor
// increment is additive only; within a major version no field is removed and
// no field's meaning changes.
func DocumentVersion() Version {
	return Version{Major: 1, Minor: 0}
}

// Report is the complete analysis artifact: one repository, at one commit,
// under one analysis configuration (ADR-0021 clause 1).
type Report struct {
	DocumentVersion Version `json:"document_version"`
	// Metadata is the one section whose values differ between two runs over
	// the same commit and configuration. Comparison tooling excludes it by
	// path.
	Metadata      Metadata      `json:"metadata"`
	Repository    Repository    `json:"repository"`
	Configuration Configuration `json:"configuration"`
	Families      Families      `json:"families"`
}

// Metadata holds the generation values that legitimately vary: when the
// report was generated and by which build (ADR-0021 clause 6, ADR-0061
// clause 6). Nothing outside this section may depend on either.
type Metadata struct {
	GeneratedAt time.Time `json:"generated_at"`
	ToolVersion string    `json:"tool_version"`
}

// Repository names what was analysed: the repository and the analysed commit.
type Repository struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

// Configuration is the section that carries the resolved analysis
// configuration the report was produced under. Its place is fixed here and
// its absence from a report is a schema violation; WP-0010 fills it.
type Configuration struct{}

// IdentityEntry is one entry of the identities section, docs/metrics.md
// section 14: a resolved contributor the reader can select (ADR-0010
// clause 2). It is not a metric family. It carries a digest and a display name
// and never a raw address (ADR-0033 clause 2).
type IdentityEntry struct {
	// ID is the identity's stable digest. The aggregate entry has none.
	ID              string `json:"id,omitzero"`
	DisplayName     string `json:"display_name"`
	FirstCommitDate string `json:"first_commit_date"`
	LastCommitDate  string `json:"last_commit_date"`
	CommitCount     int    `json:"commit_count"`
	// Aggregate marks the one entry folding every identity beyond the
	// individually represented limit (ADR-0018 clause 4).
	Aggregate bool `json:"aggregate,omitzero"`
}

// Families holds every family of ADR-0024 clause 5, in that order, keyed by
// family name. A struct rather than a map, so that no report can be built
// without one of them.
type Families struct {
	Temporal       Family[TemporalMetrics]       `json:"temporal"`
	CommitSize     Family[CommitSizeMetrics]     `json:"commit-size"`
	Messages       Family[MessagesMetrics]       `json:"messages"`
	Files          Family[FilesMetrics]          `json:"files"`
	Coupling       Family[CouplingMetrics]       `json:"coupling"`
	Ownership      Family[OwnershipMetrics]      `json:"ownership"`
	Worktype       Family[WorktypeMetrics]       `json:"worktype"`
	AIArchaeology  Family[AIArchaeologyMetrics]  `json:"ai-archaeology"`
	Hotspot        Family[HotspotMetrics]        `json:"hotspot"`
	StaticAnalysis Family[StaticAnalysisMetrics] `json:"static-analysis"`
}

// Degrade marks every computed family degraded for a condition that affects
// all of them, such as an incomplete history. Skipped families are left as
// they are.
func (f *Families) Degrade(reason Reason, confidence Confidence) {
	f.Temporal.Degrade(reason, confidence)
	f.CommitSize.Degrade(reason, confidence)
	f.Messages.Degrade(reason, confidence)
	f.Files.Degrade(reason, confidence)
	f.Coupling.Degrade(reason, confidence)
	f.Ownership.Degrade(reason, confidence)
	f.Worktype.Degrade(reason, confidence)
	f.AIArchaeology.Degrade(reason, confidence)
	f.Hotspot.Degrade(reason, confidence)
	f.StaticAnalysis.Degrade(reason, confidence)
}
