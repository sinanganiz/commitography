// The report's configuration section: the fully resolved analysis
// configuration the report was produced under (ADR-0026 clauses 2 and 3). It
// carries the resolved result only — no layer, no precedence — and passing it
// back to the command reproduces the report (clause 4).
//
// Every value in it that identifies a person is a reference, never a raw
// address (ADR-0068): an address is written as its identity digest, and under
// anonymised output a display name is written as the pseudonym the identities
// section uses. A pattern that names a class rather than a person, such as a
// bot name suffix, stays literal, because it identifies nobody. The leak scan
// covers this section exactly as it covers the rest of the report.
//
// Nothing operational appears here (ADR-0026 clause 1); TestOperationalLeak
// holds the report to that.

package core

import (
	"regexp"
	"strings"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
)

// Configuration is the report's configuration section. Its keys are the
// analysis plane's keys, so the section is a configuration file.
type Configuration struct {
	Identities            []ConfiguredIdentity `json:"identities"`
	ExcludeAuthors        []string             `json:"exclude_authors"`
	ExcludePaths          []string             `json:"exclude_paths"`
	OutlierThresholdLines int                  `json:"outlier_threshold_lines"`
	CountMerges           bool                 `json:"count_merges"`
	DateSource            string               `json:"date_source"`
	UseMailmap            bool                 `json:"use_mailmap"`
	Anonymize             bool                 `json:"anonymize"`
	// Since and Until are the instants git resolved the date bounds to, in
	// RFC 3339, or empty for no bound. The words an operator wrote are not
	// reproducible; the instant is.
	Since             string         `json:"since"`
	Until             string         `json:"until"`
	Year              int            `json:"year"`
	RecencyWindowDays int            `json:"recency_window_days"`
	CardinalityLimits map[string]int `json:"cardinality_limits"`
}

// ConfiguredIdentity is one identity merge as the report carries it: the
// addresses are digests (ADR-0068 clause 2), and the name is absent where it
// would carry an address or reveal what anonymised output hides.
type ConfiguredIdentity struct {
	Name   string   `json:"name,omitzero"`
	Emails []string `json:"emails"`
}

// EmbedConfiguration returns the configuration section for a resolved analysis
// plane. entries is the report's identities section, which decides which
// pseudonyms a reader can resolve, and resolver is the identity layer, which
// knows what an exclusion entry named.
func EmbedConfiguration(a config.Analysis, resolver *identity.Resolver, entries []IdentityEntry) Configuration {
	shown := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.Aggregate && e.ID != "" {
			shown[e.ID] = true
		}
	}

	out := Configuration{
		Identities:            []ConfiguredIdentity{},
		ExcludeAuthors:        embeddedExclusions(a, resolver),
		ExcludePaths:          append([]string{}, a.ExcludePaths...),
		OutlierThresholdLines: a.OutlierThresholdLines,
		CountMerges:           a.CountMerges,
		DateSource:            a.DateSource,
		UseMailmap:            a.UseMailmap,
		Anonymize:             a.Anonymize,
		Since:                 a.Since,
		Until:                 a.Until,
		Year:                  a.Year,
		RecencyWindowDays:     a.RecencyWindowDays,
		CardinalityLimits:     LimitValues(),
	}
	for _, id := range a.Identities {
		entry := ConfiguredIdentity{Emails: []string{}}
		for _, email := range id.Emails {
			if reference := identity.Reference(email); reference != "" {
				entry.Emails = append(entry.Emails, reference)
			}
		}
		entry.Name = embeddedName(id.Name, entry.Emails, a.Anonymize, shown)
		out.Identities = append(out.Identities, entry)
	}
	return out
}

// embeddedName returns the name an identity merge is written with. A name that
// carries an address is dropped, as it is in the identities section. Under
// anonymised output a name is replaced by the identity's pseudonym, which is
// the digest it is keyed by, and dropped where that pseudonym names no entry a
// reader can see; a class pattern stays literal (ADR-0068 clauses 4 and 5).
func embeddedName(name string, emails []string, anonymise bool, shown map[string]bool) string {
	name = strings.TrimSpace(name)
	if name == "" || addressPattern().MatchString(name) {
		return ""
	}
	if !anonymise || config.IsBotIdentity(name, "") {
		return name
	}
	if len(emails) > 0 && shown[emails[0]] {
		return emails[0]
	}
	return ""
}

// embeddedExclusions returns the author exclusion list as the report carries
// it. An entry that is already a reference stays one; an entry that is an
// address becomes its digest; a class pattern and, outside anonymised output,
// a name stay literal. Under anonymised output a name becomes a reference to
// each identity it excluded, and an entry that excluded nobody is dropped,
// which changes no analysis and reveals no name.
func embeddedExclusions(a config.Analysis, resolver *identity.Resolver) []string {
	builtin := make(map[string]bool)
	for _, entry := range config.DefaultExcludeAuthors() {
		builtin[entry] = true
	}
	out := []string{}
	seen := map[string]bool{}
	add := func(entry string) {
		if entry == "" || seen[entry] {
			return
		}
		seen[entry] = true
		out = append(out, entry)
	}
	for _, entry := range a.ExcludeAuthors {
		switch {
		case identity.IsReference(entry):
			add(entry)
		case strings.Contains(entry, "@"):
			add(identity.Reference(entry))
		case !a.Anonymize || builtin[entry] || config.IsBotIdentity(entry, ""):
			add(entry)
		case resolver != nil:
			for _, digest := range resolver.MatchedBy(entry) {
				add(digest)
			}
		}
	}
	return out
}

// addressPattern is the shape of an email address, as the identities section
// uses it: a name that contains one is not written to the report, because a
// name is attacker-controlled (ADR-0045) and could put an address list into
// the artifact.
func addressPattern() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
}
