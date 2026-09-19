package core

import (
	"reflect"
	"strings"
	"testing"
)

// reportSections returns the top-level sections of the report that are not
// metric families, by the name the document gives them. document_version and
// sections are versions rather than sections, and families carry their own.
func reportSections() []string {
	var out []string
	report := reflect.TypeOf(Report{})
	for i := 0; i < report.NumField(); i++ {
		name, _, _ := strings.Cut(report.Field(i).Tag.Get("json"), ",")
		switch name {
		case "document_version", "sections", "families":
			continue
		}
		out = append(out, name)
	}
	return out
}

// ADR-0070 clause 1: every top-level section that is not a family carries its
// own version. The check is over the report type, so a section added later
// without a version fails here rather than reaching a stored report.
func TestEverySectionCarriesAVersion(t *testing.T) {
	t.Parallel()
	versions := (&Report{Sections: CurrentSectionVersions()}).Versions()
	for _, section := range reportSections() {
		// The local is not called version: package core declares the link-time
		// build metadata under that name, and the package variable checker
		// reads an assignment to it as a write to that (ADR-0061 clause 4).
		carried, ok := versions.Sections[section]
		if !ok {
			t.Errorf("the section %s carries no version", section)
			continue
		}
		if carried.Major == 0 && carried.Minor == 0 {
			t.Errorf("the section %s carries the zero version", section)
		}
	}
	if len(versions.Sections) != len(reportSections()) {
		t.Errorf("%d sections carry versions, and the report has %d",
			len(versions.Sections), len(reportSections()))
	}
	// The sections object itself carries one entry per section.
	if got := reflect.TypeOf(SectionVersions{}).NumField(); got != len(reportSections()) {
		t.Errorf("the sections object has %d entries, and the report has %d sections", got, len(reportSections()))
	}
}

// Every family present in a report is in the version set too (ADR-0017
// clause 2), including a skipped one, whose zero version is what a cached
// report would hold.
func TestVersionsCoverEveryFamily(t *testing.T) {
	t.Parallel()
	versions := (&Report{}).Versions()
	families := reflect.TypeOf(Families{})
	if len(versions.Families) != families.NumField() {
		t.Errorf("%d families carry versions, and the report has %d", len(versions.Families), families.NumField())
	}
	for i := 0; i < families.NumField(); i++ {
		name, _, _ := strings.Cut(families.Field(i).Tag.Get("json"), ",")
		if _, ok := versions.Families[name]; !ok {
			t.Errorf("the family %s is absent from the version set", name)
		}
	}
}

// ADR-0070 clause 3 and ADR-0031 clause 4: changing any version changes what
// the cache key carries, so a stored report cannot be read as though a newer
// derivation had produced it.
func TestChangingAnyVersionChangesTheKey(t *testing.T) {
	t.Parallel()
	base := &Report{DocumentVersion: DocumentVersion(), Sections: CurrentSectionVersions()}
	base.Families.Temporal.Version = Version{Major: 1, Minor: 0}
	key := base.Versions().Key()

	if again := base.Versions().Key(); again != key {
		t.Errorf("two readings of one report produced %q and %q", key, again)
	}

	document := *base
	document.DocumentVersion = Version{Major: 1, Minor: 99}
	sections := *base
	sections.Sections.Identities = Version{Major: 9, Minor: 0}
	family := *base
	family.Families.Temporal.Version = Version{Major: 2, Minor: 0}

	for name, changed := range map[string]*Report{
		"the document version": &document,
		"a section version":    &sections,
		"a family version":     &family,
	} {
		if got := changed.Versions().Key(); got == key {
			t.Errorf("changing %s left the key at %q", name, got)
		}
	}
}
