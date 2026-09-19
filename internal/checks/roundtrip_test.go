package checks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// The configuration round trip (ADR-0026 clause 4, WP-0010 clause 7): the
// analysis configuration a report embeds is a configuration file, and running
// the command with it reproduces that report byte for byte outside the
// metadata section.
//
// It is what makes a report reproducible by someone who has only the report,
// and it is why the section carries resolved values: the instant a date bound
// resolved to rather than the words that produced it, and a digest rather than
// an address. The guarantee is over the analysis plane. The operational plane
// is not in the report, so reproducing a run that needed an operational value
// — a shallow clone the operator allowed — needs that value on the command
// line as well.

// analyseWith runs the analysis with the given options and clock, and returns
// the report as the command writes it.
func analyseWith(t *testing.T, opts pipeline.Options, clock core.Clock) string {
	t.Helper()
	result, err := newAnalyzerAt(clock).Run(context.Background(), opts, nil)
	if err != nil {
		fatal(t, 26, "analysing under %+v: %v", opts, err)
	}
	out := filepath.Join(t.TempDir(), render.ReportFile)
	if err := render.WriteReportJSON(result.Report, out); err != nil {
		fatal(t, 26, "writing the report: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		fatal(t, 26, "reading the report: %v", err)
	}
	return string(data)
}

// embeddedConfiguration returns the report's configuration section, as
// `jq .configuration` returns it. JSON is YAML, so what comes out is a
// configuration file.
func embeddedConfiguration(t *testing.T, produced string) map[string]any {
	t.Helper()
	var document map[string]json.RawMessage
	if err := json.Unmarshal([]byte(produced), &document); err != nil {
		fatal(t, 26, "the report is not JSON: %v", err)
	}
	section, ok := document["configuration"]
	if !ok {
		fatal(t, 26, "the report carries no configuration section")
	}
	var configuration map[string]any
	if err := json.Unmarshal(section, &configuration); err != nil {
		fatal(t, 26, "the configuration section is not an object: %v", err)
	}
	return configuration
}

// configurationFile writes a configuration section to a file the command can
// be pointed at.
func configurationFile(t *testing.T, configuration map[string]any) string {
	t.Helper()
	data, err := json.Marshal(configuration)
	if err != nil {
		fatal(t, 26, "encoding the configuration: %v", err)
	}
	path := filepath.Join(t.TempDir(), "configuration.yml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fatal(t, 26, "writing the configuration: %v", err)
	}
	return path
}

// rerun analyses the same repository with nothing but the report's own
// configuration, and returns how the second report differs from the first
// outside the metadata section.
func rerun(t *testing.T, repoPath, produced string, clock core.Clock) string {
	t.Helper()
	path := configurationFile(t, embeddedConfiguration(t, produced))
	return sameInputDifference(produced, analyseWith(t, pipeline.Options{RepoPath: repoPath, ConfigPath: path}, clock))
}

// TestConfigRoundTripReproducesEveryFixture runs the round trip over every
// fixture, with anonymised output and without.
func TestConfigRoundTripReproducesEveryFixture(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fixtures := generatedFixtures(t, repo)
	if len(fixtures) == 0 {
		fatal(t, 64, "no fixture was generated; the gates generate them with `make fixtures`")
	}
	// A fixture the analysis refuses produces no report to reproduce, and its
	// golden file is the refusal rather than a report. The set is decided here
	// rather than inside the subtests, which run after this function returns.
	reproduced := 0
	for _, fixture := range fixtures {
		if !repo.isTracked(goldenFile(fixture, false)) {
			continue
		}
		reproduced++
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
			for _, anonymise := range []bool{false, true} {
				produced := analyseWith(t, pipeline.Options{RepoPath: dir, Anonymize: anonymise}, core.FixedClock(checkTime()))
				if diff := rerun(t, dir, produced, core.FixedClock(checkTime())); diff != "" {
					report(t, 26, "the report of fixture %s, anonymised %v, is not reproduced by its own "+
						"configuration (- first, + second):\n%s", fixture, anonymise, diff)
				}
			}
		})
	}
	if reproduced == 0 {
		fatal(t, 64, "no fixture produced a report, so nothing was reproduced")
	}
}

// TestConfigRoundTripWithDateBoundsAndYear covers the values that reach the
// analysis from the command line: a bare date, which git completes with the
// time of day, and the year. The rerun reads a clock seven hours later, which
// is what the resolved bound exists to survive.
func TestConfigRoundTripWithDateBoundsAndYear(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := filepath.Join(repo.root, "testdata", "fixtures", "basic")
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "the basic fixture is missing; the gates generate it with `make fixtures`")
	}
	morning := core.FixedClock(checkTime())
	evening := core.FixedClock(checkTime().Add(7 * time.Hour))

	produced := analyseWith(t, pipeline.Options{
		RepoPath: dir,
		Since:    "2025-11-15",
		Until:    "2026-02-01",
	}, morning)

	configuration := embeddedConfiguration(t, produced)
	for _, bound := range []string{"since", "until"} {
		value, _ := configuration[bound].(string)
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			report(t, 26, "the embedded %s is %q, which is not the instant git resolved it to", bound, value)
		}
	}
	if diff := rerun(t, dir, produced, evening); diff != "" {
		report(t, 26, "a report produced with a bare date bound is not reproduced from its configuration "+
			"seven hours later (- first, + second):\n%s", diff)
	}

	// The year is an analysis value too, and reaches the rerun the same way.
	withYear := analyseWith(t, pipeline.Options{RepoPath: dir, Year: 2025}, morning)
	if got := embeddedConfiguration(t, withYear)["year"]; got != 2025.0 {
		report(t, 26, "the embedded year is %v, want 2025", got)
	}
	if diff := rerun(t, dir, withYear, evening); diff != "" {
		report(t, 26, "a report produced for one year is not reproduced from its configuration "+
			"(- first, + second):\n%s", diff)
	}
}

// TestConfigRoundTripWithIdentitiesAndExclusions covers the values ADR-0068
// turns into references: an identity merge and an author exclusion by address.
// A file naming addresses and one naming their digests must produce the same
// report, which is what lets the embedded form be handed back.
func TestConfigRoundTripWithIdentitiesAndExclusions(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := filepath.Join(repo.root, "testdata", "fixtures", "basic")
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "the basic fixture is missing; the gates generate it with `make fixtures`")
	}

	byAddress := `
identities:
  - name: Ada Lovelace
    emails:
      - ada@example.com
      - ada.lovelace@corp.example.com
exclude_authors:
  - alan@example.com
`
	byDigest := `
identities:
  - name: Ada Lovelace
    emails:
      - ` + core.IdentityDigest("ada@example.com") + `
      - ` + core.IdentityDigest("ada.lovelace@corp.example.com") + `
exclude_authors:
  - ` + core.IdentityDigest("alan@example.com") + `
`
	write := func(body string) string {
		path := filepath.Join(t.TempDir(), "configuration.yml")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			fatal(t, 26, "writing a configuration: %v", err)
		}
		return path
	}

	for _, anonymise := range []bool{false, true} {
		addresses := analyseWith(t, pipeline.Options{
			RepoPath: dir, ConfigPath: write(byAddress), Anonymize: anonymise,
		}, core.FixedClock(checkTime()))
		digests := analyseWith(t, pipeline.Options{
			RepoPath: dir, ConfigPath: write(byDigest), Anonymize: anonymise,
		}, core.FixedClock(checkTime()))

		if diff := sameInputDifference(addresses, digests); diff != "" {
			report(t, 68, "a configuration naming addresses and one naming their digests produced different "+
				"reports, anonymised %v (- addresses, + digests):\n%s", anonymise, diff)
		}
		if strings.Contains(embeddedConfigurationText(t, addresses), "@") {
			report(t, 68, "the configuration section carries an address, anonymised %v: %s",
				anonymise, embeddedConfigurationText(t, addresses))
		}
		if diff := rerun(t, dir, addresses, core.FixedClock(checkTime())); diff != "" {
			report(t, 26, "a report produced with identity merges and exclusions is not reproduced from its "+
				"configuration, anonymised %v (- first, + second):\n%s", anonymise, diff)
		}
	}
}

// embeddedConfigurationText returns the configuration section as text.
func embeddedConfigurationText(t *testing.T, produced string) string {
	t.Helper()
	data, err := json.Marshal(embeddedConfiguration(t, produced))
	if err != nil {
		fatal(t, 26, "encoding the configuration section: %v", err)
	}
	return string(data)
}

// TestConfigRoundTripAnonymisedNamesAreResolvable is ADR-0068 clause 4's
// acceptance criterion: with anonymisation on, every value in the
// configuration section that would otherwise carry a person's name is either a
// reference the identities section carries, or a class pattern, which names
// nobody.
func TestConfigRoundTripAnonymisedNamesAreResolvable(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := filepath.Join(repo.root, "testdata", "fixtures", "basic")
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "the basic fixture is missing; the gates generate it with `make fixtures`")
	}
	path := filepath.Join(t.TempDir(), "configuration.yml")
	body := `
identities:
  - name: Ada Lovelace
    emails:
      - ada@example.com
      - ada.lovelace@corp.example.com
exclude_authors:
  - Grace Hopper
  - deploy[bot]
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		fatal(t, 26, "writing a configuration: %v", err)
	}
	produced := analyseWith(t, pipeline.Options{RepoPath: dir, ConfigPath: path, Anonymize: true},
		core.FixedClock(checkTime()))

	var document struct {
		Identities []struct {
			ID string `json:"id"`
		} `json:"identities"`
		Configuration struct {
			Identities []struct {
				Name string `json:"name"`
			} `json:"identities"`
			ExcludeAuthors []string `json:"exclude_authors"`
		} `json:"configuration"`
	}
	if err := json.Unmarshal([]byte(produced), &document); err != nil {
		fatal(t, 26, "reading the report: %v", err)
	}
	shown := map[string]bool{}
	for _, entry := range document.Identities {
		shown[entry.ID] = true
	}
	if len(shown) == 0 {
		fatal(t, 64, "the anonymised report names no identity, so there is nothing to resolve against")
	}
	class := map[string]bool{}
	for _, entry := range config.DefaultExcludeAuthors() {
		class[entry] = true
	}

	// Under anonymised output a value that would name a person is either a
	// reference, which reveals nothing and is the same digest the identities
	// section would show, or a class pattern, which names nobody. A literal
	// name is neither, and is what this check exists to catch.
	resolvable := func(what, value string, mustBeShown bool) {
		t.Helper()
		switch {
		case value == "", class[value], config.IsBotIdentity(value, ""):
		case identity.IsReference(value):
			// A name is written as a pseudonym only where a reader can see the
			// entry it names; an exclusion entry may name an identity the
			// report leaves out, because excluding it is why it is not there.
			if mustBeShown && !shown[value] {
				report(t, 68, "the anonymised configuration names the %s %q, which the identities section "+
					"does not carry, so no reader can resolve it", what, value)
			}
		default:
			report(t, 68, "the anonymised configuration carries the %s %q, which is a name rather than a "+
				"reference or a class pattern", what, value)
		}
	}
	named := 0
	for _, merge := range document.Configuration.Identities {
		if merge.Name != "" {
			named++
		}
		resolvable("configured name", merge.Name, true)
	}
	if named == 0 {
		fatal(t, 64, "the anonymised configuration names no identity, so the check proves nothing")
	}
	for _, entry := range document.Configuration.ExcludeAuthors {
		resolvable("exclusion entry", entry, false)
	}
	if strings.Contains(embeddedConfigurationText(t, produced), "Grace") {
		report(t, 68, "the anonymised configuration carries the name of an excluded person")
	}
}

// TestConfigRoundTripRejectsADroppedValue is the failure demonstration
// ADR-0064 clause 6 requires: a configuration section missing an analysis
// value no longer reproduces its report, and the round trip says so.
func TestConfigRoundTripRejectsADroppedValue(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := filepath.Join(repo.root, "testdata", "fixtures", "basic")
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "the basic fixture is missing; the gates generate it with `make fixtures`")
	}
	// Each value below differs from the built-in one, so dropping it from the
	// section changes the analysis. A value that happens to equal the default
	// would prove nothing by its absence.
	produced := analyseWith(t, pipeline.Options{
		RepoPath:       dir,
		Since:          "2025-11-15",
		Year:           2026,
		CountMerges:    true,
		CountMergesSet: true,
	}, core.FixedClock(checkTime()))

	for _, dropped := range []string{"since", "year", "count_merges"} {
		configuration := embeddedConfiguration(t, produced)
		delete(configuration, dropped)
		second := analyseWith(t, pipeline.Options{
			RepoPath: dir, ConfigPath: configurationFile(t, configuration),
		}, core.FixedClock(checkTime()))
		if sameInputDifference(produced, second) == "" {
			report(t, 64, "dropping %s from the embedded configuration still reproduced the report, so the "+
				"round trip cannot catch a value the section fails to carry", dropped)
		}
	}
}
