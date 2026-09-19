package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
)

// resolvedAnalysis is a configuration with every analysis value set to
// something other than its default, so that changing any one of them can be
// seen.
func resolvedAnalysis() config.Analysis {
	a := config.Default()
	a.Identities = []config.Identity{{Name: "Ada Lovelace", Emails: []string{"ada@example.com", "ada@corp.example.com"}}}
	a.ExcludeAuthors = append(a.ExcludeAuthors, "release-robot")
	a.ExcludePaths = append(a.ExcludePaths, "generated/**")
	a.Since = "2025-01-01T00:00:00Z"
	a.Until = "2025-12-31T00:00:00Z"
	a.Year = 2025
	a.CardinalityLimits = LimitValues()
	return a
}

// Two analyses differing in one analysis value produce different digests
// (ADR-0026 clause 5).
func TestConfigurationDigestChangesWithEveryValue(t *testing.T) {
	t.Parallel()
	base := resolvedAnalysis()
	digest := ConfigurationDigest(base)
	if again := ConfigurationDigest(resolvedAnalysis()); again != digest {
		t.Fatalf("one configuration produced two digests, %s and %s", digest, again)
	}

	changes := map[string]func(a *config.Analysis){
		"an identity's name":        func(a *config.Analysis) { a.Identities[0].Name = "A. Lovelace" },
		"an identity's address":     func(a *config.Analysis) { a.Identities[0].Emails[1] = "ada@other.example.com" },
		"an identity dropped":       func(a *config.Analysis) { a.Identities = nil },
		"an excluded author":        func(a *config.Analysis) { a.ExcludeAuthors[0] = "someone-else" },
		"an excluded path":          func(a *config.Analysis) { a.ExcludePaths[0] = "elsewhere/**" },
		"the exclusion order":       func(a *config.Analysis) { a.ExcludePaths[0], a.ExcludePaths[1] = a.ExcludePaths[1], a.ExcludePaths[0] },
		"the outlier threshold":     func(a *config.Analysis) { a.OutlierThresholdLines = 500 },
		"merge counting":            func(a *config.Analysis) { a.CountMerges = !a.CountMerges },
		"the date source":           func(a *config.Analysis) { a.DateSource = config.DateSourceCommitter },
		"mailmap use":               func(a *config.Analysis) { a.UseMailmap = !a.UseMailmap },
		"anonymisation":             func(a *config.Analysis) { a.Anonymize = !a.Anonymize },
		"the lower date bound":      func(a *config.Analysis) { a.Since = "2024-01-01T00:00:00Z" },
		"the upper date bound":      func(a *config.Analysis) { a.Until = "2026-12-31T00:00:00Z" },
		"the year":                  func(a *config.Analysis) { a.Year = 2026 },
		"the recency window":        func(a *config.Analysis) { a.RecencyWindowDays = 14 },
		"an added excluded author":  func(a *config.Analysis) { a.ExcludeAuthors = append(a.ExcludeAuthors, "another") },
		"an identity's second name": func(a *config.Analysis) { a.Identities[0].Emails = a.Identities[0].Emails[:1] },
	}
	for name, change := range changes {
		changed := resolvedAnalysis()
		change(&changed)
		if got := ConfigurationDigest(changed); got == digest {
			t.Errorf("changing %s left the digest at %s", name, got)
		}
	}
}

// A configuration naming addresses and one naming their digests are the same
// configuration, so they have one digest (ADR-0068 clause 3).
func TestConfigurationDigestIsTheSameForAddressesAndDigests(t *testing.T) {
	t.Parallel()
	addresses := resolvedAnalysis()
	addresses.ExcludeAuthors = append(addresses.ExcludeAuthors, "grace@example.com")

	digests := resolvedAnalysis()
	digests.Identities = []config.Identity{{
		Name:   "Ada Lovelace",
		Emails: []string{identity.Digest("ada@example.com"), identity.Digest("ada@corp.example.com")},
	}}
	digests.ExcludeAuthors = append(digests.ExcludeAuthors, identity.Digest("grace@example.com"))

	if a, b := ConfigurationDigest(addresses), ConfigurationDigest(digests); a != b {
		t.Errorf("the address form digests to %s and the digest form to %s", a, b)
	}
}

// The digest is of resolved values, so nothing about the text they came from
// can reach it: no raw address, and no configuration file's spacing.
func TestConfigurationDigestCarriesNoAddress(t *testing.T) {
	t.Parallel()
	digest := ConfigurationDigest(resolvedAnalysis())
	if strings.Contains(digest, "@") || len(digest) != 64 {
		t.Errorf("the digest is %q, want 64 hexadecimal characters", digest)
	}
}

// The configuration section is a configuration file: its keys are the loader's
// analysis keys, exactly, so what a report embeds can be handed back.
func TestEmbeddedConfigurationKeysAreTheAnalysisKeys(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(EmbedConfiguration(resolvedAnalysis(), nil, nil))
	if err != nil {
		t.Fatalf("encoding the configuration section: %v", err)
	}
	var section map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &section); err != nil {
		t.Fatalf("reading the configuration section: %v", err)
	}
	for _, key := range config.AnalysisKeys() {
		if _, ok := section[key]; !ok {
			t.Errorf("the configuration section has no %s, which a configuration file may set", key)
		}
	}
	if len(section) != len(config.AnalysisKeys()) {
		t.Errorf("the configuration section has %d keys and the analysis plane has %d",
			len(section), len(config.AnalysisKeys()))
	}
	for _, key := range config.OperationalKeys() {
		if _, ok := section[key]; ok {
			t.Errorf("the configuration section carries the operational key %s", key)
		}
	}
}

// Under anonymised output the section carries pseudonyms and references in
// place of names (ADR-0068 clause 4); outside it, the names as configured.
func TestEmbeddedConfigurationNamesUnderAnonymisation(t *testing.T) {
	t.Parallel()
	a := resolvedAnalysis()
	a.Identities = append(a.Identities, config.Identity{Name: "deploy[bot]", Emails: []string{"deploy@example.com"}})
	shown := []IdentityEntry{{ID: identity.Digest("ada@example.com"), DisplayName: "Ada Lovelace"}}

	plain := EmbedConfiguration(a, nil, shown)
	if plain.Identities[0].Name != "Ada Lovelace" {
		t.Errorf("the configured name is %q, want it as configured", plain.Identities[0].Name)
	}
	if got := plain.Identities[0].Emails[0]; got != identity.Digest("ada@example.com") {
		t.Errorf("the configured address is %q, want its digest", got)
	}

	a.Anonymize = true
	anonymised := EmbedConfiguration(a, nil, shown)
	if got := anonymised.Identities[0].Name; got != identity.Digest("ada@example.com") {
		t.Errorf("the anonymised name is %q, want the pseudonym the identities section uses", got)
	}
	// A class pattern names nobody, so anonymisation leaves it alone; an
	// identity no entry names has no pseudonym a reader could resolve.
	if got := anonymised.Identities[1].Name; got != "deploy[bot]" {
		t.Errorf("the anonymised class pattern is %q, want it literal", got)
	}
	notShown := EmbedConfiguration(a, nil, nil)
	if got := notShown.Identities[0].Name; got != "" {
		t.Errorf("the anonymised name of an identity no entry names is %q, want it absent", got)
	}
}
