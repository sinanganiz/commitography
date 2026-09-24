// Package core holds what every other package may depend on: the report
// types, the reason codes and cardinality limits, and the shared definitions
// the metric families compute over (ADR-0040 clause 1). The report types are the snapshot
// contract of ADR-0021 and are versioned under ADR-0031. Configuration,
// identity, the domain model and filtering live in subpackages beneath it
// (ADR-0066 clause 2).
//
// The report document follows docs/metrics.md, which is authoritative
// (ADR-0062): no field exists in it that the catalogue does not define, every
// family of ADR-0076 clause 6 is present with a status (ADR-0032 clause 1)
// under the namespace the catalogue gives it, and everything that varies
// between two runs over one commit is confined to the metadata section
// (ADR-0021 clause 6). docs/report-schema.json describes it.
package core

import (
	"reflect"
	"strings"
	"time"
)

// DocumentVersion is the version of the report's structure: its top-level
// shape, identity representation, status fields and metadata section
// (ADR-0031 clause 1). A minor increment is additive only; within a major
// version no field is removed and no field's meaning changes.
//
//	1.0  the document docs/metrics.md defines
//	1.1  the identities section added, representing an identity by its digest
//	     and display name (docs/metrics.md section 14)
//	1.2  each identity's source address count and merge candidates added, and
//	     the family status code unresolved_identity (docs/metrics.md sections
//	     13 and 14)
//	1.3  the configuration section filled with the resolved analysis
//	     configuration (ADR-0026 clause 2)
//	1.4  the sections object added, carrying a version for every top-level
//	     section that is not a family (ADR-0070 clause 1)
//	2.0  every family written under the namespace docs/metrics.md gives it,
//	     so commit-size, ai-archaeology and static-analysis became
//	     commit_size, ai_archaeology and static_analysis (ADR-0062 clause 3,
//	     ADR-0076 clause 1); and the family status code worktree_unavailable
//	     removed, which nothing produced once no family read the working tree
//	     (ADR-0076 clause 11)
func DocumentVersion() Version {
	return Version{Major: 2, Minor: 0}
}

// Report is the complete analysis artifact: one repository, at one commit,
// under one analysis configuration (ADR-0021 clause 1).
type Report struct {
	DocumentVersion Version `json:"document_version"`
	// Sections is the version of each top-level section that is not a metric
	// family, governing the meaning of that section's values as a family's
	// version governs its own (ADR-0070 clause 1). versions.go defines it.
	Sections SectionVersions `json:"sections"`
	// Metadata is the one section whose values differ between two runs over
	// the same commit and configuration. Comparison tooling excludes it by
	// path.
	Metadata   Metadata   `json:"metadata"`
	Repository Repository `json:"repository"`
	// Configuration is the resolved analysis configuration the report was
	// produced under (ADR-0026 clause 2). configuration.go defines it.
	Configuration Configuration `json:"configuration"`
	// Identities is the list of resolved contributors a reader selects from
	// (ADR-0010 clause 2). It is a reference section, not a family.
	Identities []IdentityEntry `json:"identities"`
	Families   Families        `json:"families"`
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
	// SourceAddressCount is how many distinct addresses, as the commits
	// record them, are folded into the identity. The count is exported; the
	// addresses are not (ADR-0033 clause 2).
	SourceAddressCount int `json:"source_address_count"`
	// MergeCandidates are the other identities a signal suggests may be the
	// same person. They are suggestions: nothing applies them (ADR-0010
	// clause 4). Every individual entry carries the list, empty where there is
	// none; the aggregate entry has none.
	MergeCandidates []MergeCandidate `json:"merge_candidates,omitzero"`
	// Aggregate marks the one entry folding every identity beyond the
	// individually represented limit (ADR-0018 clause 4).
	Aggregate bool `json:"aggregate,omitzero"`
}

// MergeCandidate is one suggestion on an identities entry: another entry, by
// id, and the signal that connects the two (docs/metrics.md section 14).
type MergeCandidate struct {
	ID     string `json:"id"`
	Signal string `json:"signal"`
}

// Families holds every family of ADR-0076 clause 6, in that order, each keyed
// by the namespace docs/metrics.md gives it, which is the namespace the family
// declares (ADR-0076 clause 1, ADR-0062 clause 3). A struct rather than a map,
// so that no report can be built without one of them.
type Families struct {
	Temporal       Family[TemporalMetrics]       `json:"temporal"`
	CommitSize     Family[CommitSizeMetrics]     `json:"commit_size"`
	Messages       Family[MessagesMetrics]       `json:"messages"`
	Files          Family[FilesMetrics]          `json:"files"`
	Coupling       Family[CouplingMetrics]       `json:"coupling"`
	Ownership      Family[OwnershipMetrics]      `json:"ownership"`
	Worktype       Family[WorktypeMetrics]       `json:"worktype"`
	AIArchaeology  Family[AIArchaeologyMetrics]  `json:"ai_archaeology"`
	Hotspot        Family[HotspotMetrics]        `json:"hotspot"`
	StaticAnalysis Family[StaticAnalysisMetrics] `json:"static_analysis"`
}

// Place writes one family's section into the report under namespace, the key
// the report writes that family under (ADR-0076 clauses 1 and 5). The
// aggregate stage's registry places every family's section through it, each
// under the namespace the family declares.
//
// It refuses a namespace the report has no family for, a section whose
// metric type is not the one that namespace holds, a section with no status,
// and a namespace already written. So a family's output cannot land outside
// its namespace, or in a namespace another family has written.
func Place[M any](f *Families, namespace string, section Family[M]) error {
	slot, ok := familySlot(f, namespace)
	if !ok {
		return Internalf(nil, "placing a family section under %q, which is no family namespace of the report", namespace)
	}
	target, ok := slot.Addr().Interface().(*Family[M])
	if !ok {
		return Internalf(nil, "placing a %T under %q, which holds a %s: a family writes only into its own "+
			"namespace (ADR-0076 clause 5)", section, namespace, slot.Type())
	}
	if section.Status == "" {
		return Internalf(nil, "placing a section with no status under %q (ADR-0032 clause 2)", namespace)
	}
	if target.Status != "" {
		return Internalf(nil, "placing a second section under %q: a namespace has one owner (ADR-0076 clause 5)",
			namespace)
	}
	*target = section
	return nil
}

// Unplaced returns the namespaces no section has been placed under, in the
// order the report writes them. A report with any is missing a family, which
// no report may be (ADR-0032 clause 1).
func (f *Families) Unplaced() []string {
	var out []string
	v := reflect.ValueOf(f).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).FieldByName("Status").String() == "" {
			out = append(out, familyKey(v.Type().Field(i)))
		}
	}
	return out
}

// familySlot returns the field of f the report writes under namespace.
func familySlot(f *Families, namespace string) (reflect.Value, bool) {
	v := reflect.ValueOf(f).Elem()
	for i := 0; i < v.NumField(); i++ {
		if familyKey(v.Type().Field(i)) == namespace {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// familyKey returns the key the report writes a family field under.
func familyKey(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	return name
}
