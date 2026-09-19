// The versions a report carries (ADR-0031, ADR-0070).
//
// The document version governs structure: the top-level shape, identity
// representation, status fields and the metadata section's place. Every metric
// family carries its own version for the meaning of its values. So does every
// top-level section that is not a family, because their values are derived too
// and a stored report would otherwise carry values computed one way beside
// values computed another, under the same name (ADR-0070 clauses 1 and 2).
//
// Versions returns all of them together: the version half of the report cache
// key (ADR-0017 clause 2, ADR-0070 clause 3), which WP-0033 composes with the
// configuration digest and the repository's identity.

package core

import (
	"fmt"
	"sort"
	"strings"
)

// SectionVersions is the version of each top-level section of the report that
// is not a metric family. A change to how a section's values are derived
// increments it; adding a field to a section increments its minor component
// only (ADR-0070 clause 1).
type SectionVersions struct {
	Metadata      Version `json:"metadata"`
	Repository    Version `json:"repository"`
	Configuration Version `json:"configuration"`
	Identities    Version `json:"identities"`
}

// CurrentSectionVersions returns the version of each section as this build
// derives it.
//
//	metadata       1.0  the generation time and the build that produced it
//	repository     1.0  the repository's name and the analysed commit
//	configuration  1.0  the resolved analysis configuration, with every value
//	                    that identifies a person as a reference (ADR-0026
//	                    clause 2, ADR-0068)
//	identities     1.0  the section as it was first emitted: a digest, a
//	                    display name, the dates and the commit count
//	               1.1  source address counts and merge candidates added
//	               2.0  merge candidates derived from the analysed commits
//	                    alone, where they had also drawn on configured values
//	                    (ADR-0069)
func CurrentSectionVersions() SectionVersions {
	return SectionVersions{
		Metadata:      Version{Major: 1, Minor: 0},
		Repository:    Version{Major: 1, Minor: 0},
		Configuration: Version{Major: 1, Minor: 0},
		Identities:    Version{Major: 2, Minor: 0},
	}
}

// VersionSet is every version one report carries, by the name the report gives
// its section or family.
type VersionSet struct {
	Document Version
	Sections map[string]Version
	Families map[string]Version
}

// Versions returns the report's versions. A family that is skipped carries the
// zero version and is included as it stands: what a cached report holds is
// what the key must cover.
func (r *Report) Versions() VersionSet {
	f := r.Families
	return VersionSet{
		Document: r.DocumentVersion,
		Sections: map[string]Version{
			"metadata":      r.Sections.Metadata,
			"repository":    r.Sections.Repository,
			"configuration": r.Sections.Configuration,
			"identities":    r.Sections.Identities,
		},
		Families: map[string]Version{
			"temporal":        f.Temporal.Version,
			"commit-size":     f.CommitSize.Version,
			"messages":        f.Messages.Version,
			"files":           f.Files.Version,
			"coupling":        f.Coupling.Version,
			"ownership":       f.Ownership.Version,
			"worktype":        f.Worktype.Version,
			"ai-archaeology":  f.AIArchaeology.Version,
			"hotspot":         f.Hotspot.Version,
			"static-analysis": f.StaticAnalysis.Version,
		},
	}
}

// Key returns the versions as one ordered string, so that changing any of them
// changes it. It is what the report cache key carries of the versions; the key
// itself is WP-0033's, as the configuration digest is.
func (v VersionSet) Key() string {
	var b strings.Builder
	fmt.Fprintf(&b, "document=%s", v.Document)
	for _, group := range []struct {
		prefix   string
		versions map[string]Version
	}{
		{"section", v.Sections},
		{"family", v.Families},
	} {
		names := make([]string, 0, len(group.versions))
		for name := range group.versions {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintf(&b, ";%s.%s=%s", group.prefix, name, group.versions[name])
		}
	}
	return b.String()
}

// String returns the version as major.minor.
func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }
