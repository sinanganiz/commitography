package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/aggregate"
)

// The projection checker (ADR-0078 clause 7, ADR-0019 clause 3, WP-0063).
// For every family whose registry slot is filled, a figure projected from the
// cells a report carries equals the figure recomputed from the inputs, in
// three dimensions:
//
//   - repository: the projection for the zero scope is the metrics the
//     report holds, cells aside;
//   - identities: the projection for two individually represented identities
//     is the projection for the one identity they become in an analysis whose
//     configuration merges them (config.Identity);
//   - years: the projection for a year is the family's section built over the
//     input restricted to that year (core.Input.RestrictToYear), cells aside.
//
// It extends the identity projection invariant ADR-0018 clause 2 requires of
// the work-type breakdown to every family with cells. It also requires the
// families with a filled slot to be the families whose catalogue section has
// a Cell field table, so a family cannot carry cells without joining it.
//
// The families are driven through the registry alone: a family joins by
// filling its slot, and nothing here names one.

// projectionFixtures are the fixtures the checker compares over: one person
// under two addresses, the same folded by a .mailmap, merges, and years
// without any commit.
func projectionFixtures() []string {
	return []string{"basic", "mailmap", "merges", "multi-year-gap"}
}

// projectionPairs is the most pairs of identities compared on one fixture.
const projectionPairs = 10

// projectionInput is one input as the aggregate stage runs its families over
// it, carrying its identity table, and the registered families' sections
// built over it, once.
type projectionInput struct {
	in       core.Input
	families *core.Families
}

// registered returns the registered families' sections built over the input.
func (p *projectionInput) registered(t *testing.T) *core.Families {
	t.Helper()
	if p.families == nil {
		families, _, err := aggregate.BuildFamilies(p.in)
		if err != nil {
			fatal(t, 78, "building the families over an input: %v", err)
		}
		p.families = families
	}
	return p.families
}

// projectionRun is one analysis of a fixture: the input its families were
// built over, and its report.
type projectionRun struct {
	input  *projectionInput
	report *core.Report
}

// projectionPair is two individually represented identities, and the analysis
// whose configuration merged them into the first.
type projectionPair struct {
	a, b   string
	merged projectionRun
}

// projectionFixture is what the checker compares over for one fixture.
type projectionFixture struct {
	name  string
	base  projectionRun
	pairs []projectionPair
	// years are the years with an analysed commit, ascending, and restricted
	// the base input restricted to each.
	years      []int
	restricted map[int]*projectionInput
}

// analyseForProjection analyses a fixture as the command does, under the
// configuration file given, if any, and returns the input the aggregate stage
// built its families over, with its report. The input is rebuilt from the
// prepared inputs as the pipeline root rebuilds it, and a report built over it
// must be the analysis's report byte for byte: otherwise it is not the stage's
// input, and nothing compared over it would show anything.
func analyseForProjection(t *testing.T, fixture, dir, configPath string) projectionRun {
	t.Helper()
	ctx := context.Background()
	analyzer, opts := newAnalyzer(), pipeline.Options{RepoPath: dir, ConfigPath: configPath}
	prepared, err := analyzer.Prepare(ctx, opts)
	if err != nil {
		fatal(t, 78, "preparing %s: %v", fixture, err)
	}
	report, _, err := analyzer.Aggregate(ctx, *prepared, opts)
	if err != nil {
		fatal(t, 78, "aggregating %s: %v", fixture, err)
	}

	paths, err := filter.NewPathFilterFromAttributes(prepared.Analysis, []byte(prepared.History.Attributes))
	if err != nil {
		fatal(t, 78, "building %s's path filter: %v", fixture, err)
	}
	in := core.Input{
		Context:    ctx,
		Repository: prepared.History.Repository,
		Config:     prepared.Analysis,
		Filtered:   filter.Summarize(prepared.History.Commits),
		Resolver:   identity.NewResolver(prepared.Analysis, prepared.History.Commits),
		PathFilter: paths,
		Replay:     prepared.Replay,
	}
	in.Identities = core.NewIdentityTable(in.Analyzed())
	rebuilt, _, err := aggregate.New(core.FixedClock(checkTime()), nil).Build(in)
	if err != nil {
		fatal(t, 78, "building a report over %s's rebuilt input: %v", fixture, err)
	}
	if want, got := renderReport(t, 78, report), renderReport(t, 78, rebuilt); got != want {
		fatal(t, 64, "the input rebuilt for %s is not the one the aggregate stage ran over: a report built over it "+
			"differs from the analysis's (- analysis, + rebuilt):\n%s", fixture, unifiedDiff(want, got))
	}
	// The families the checker builds over the input are the report's, so
	// what it compares with is what the report holds.
	run := projectionRun{input: &projectionInput{in: in}, report: report}
	want, err := json.Marshal(report.Families)
	if err != nil {
		fatal(t, 78, "encoding %s's families: %v", fixture, err)
	}
	got, err := json.Marshal(run.input.registered(t))
	if err != nil {
		fatal(t, 78, "encoding the families built over %s's input: %v", fixture, err)
	}
	if string(got) != string(want) {
		fatal(t, 64, "the families built over %s's input are not the report's", fixture)
	}
	return run
}

// mergedConfiguration returns a configuration file reproducing a report's
// analysis with the identities a and b merged into one (config.Identity): the
// report's own configuration, which reproduces the report (ADR-0026
// clause 4), with the configured identity of each, where it has one, replaced
// by one identity listing a's references and then b's. A reference resolves
// as the address it is the digest of (ADR-0068 clause 3), and the merged
// identity's id is its first reference, a.
func mergedConfiguration(t *testing.T, report *core.Report, a, b string) string {
	t.Helper()
	configuration := report.Configuration
	references := map[string][]string{}
	var kept []core.ConfiguredIdentity
	for _, configured := range configuration.Identities {
		if len(configured.Emails) > 0 && (configured.Emails[0] == a || configured.Emails[0] == b) {
			references[configured.Emails[0]] = append(references[configured.Emails[0]], configured.Emails...)
			continue
		}
		kept = append(kept, configured)
	}
	merged := core.ConfiguredIdentity{}
	for _, id := range []string{a, b} {
		if len(references[id]) == 0 {
			references[id] = []string{id}
		}
		merged.Emails = append(merged.Emails, references[id]...)
	}
	for _, entry := range report.Identities {
		if entry.ID == a {
			merged.Name = entry.DisplayName
		}
	}
	configuration.Identities = append(kept, merged)

	data, err := json.Marshal(configuration)
	if err != nil {
		fatal(t, 78, "encoding a merged configuration: %v", err)
	}
	path := filepath.Join(t.TempDir(), "configuration.yml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		fatal(t, 78, "writing a merged configuration: %v", err)
	}
	return path
}

// prepareProjectionFixture analyses a fixture, analyses it again for each of
// its first pairs of individually represented identities with the pair
// merged, and restricts its input to each year it has an analysed commit in.
func prepareProjectionFixture(t *testing.T, repo repository, fixture string) projectionFixture {
	t.Helper()
	dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
	if _, err := os.Stat(dir); err != nil {
		fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", fixture)
	}
	base := analyseForProjection(t, fixture, dir, "")
	f := projectionFixture{name: fixture, base: base, restricted: map[int]*projectionInput{}}

	var individual []string
	for _, row := range base.input.in.Identities.Rows() {
		if row.Individual {
			individual = append(individual, row.ID)
		}
	}
	for i := 0; i < len(individual) && len(f.pairs) < projectionPairs; i++ {
		for j := i + 1; j < len(individual) && len(f.pairs) < projectionPairs; j++ {
			a, b := individual[i], individual[j]
			merged := analyseForProjection(t, fixture, dir, mergedConfiguration(t, base.report, a, b))
			table := merged.input.in.Identities
			if _, still := table.Lookup(b); still || !table.Individual(a) {
				fatal(t, 64, "merging %s and %s in %s did not leave one individually represented identity %s, "+
					"so comparing with it proves nothing", a, b, fixture, a)
			}
			f.pairs = append(f.pairs, projectionPair{a: a, b: b, merged: merged})
		}
	}

	seen := map[int]bool{}
	for _, c := range base.input.in.Analyzed() {
		if year := core.CommitYear(c); !seen[year] {
			seen[year] = true
			f.years = append(f.years, year)
			f.restricted[year] = &projectionInput{in: base.input.in.RestrictToYear(year)}
		}
	}
	sort.Ints(f.years)
	return f
}

// projectionSubject is one family as the checker compares it.
type projectionSubject struct {
	name string
	// section returns the family's section built over an input: what its
	// projection reads.
	section func(t *testing.T, p *projectionInput) any
	// metrics returns the family's metrics as a section built by section
	// carries them.
	metrics func(t *testing.T, section any) any
	// project returns the family's metrics for a scope, from a section built
	// by section and nothing else.
	project func(section any, scope core.Scope) any
}

// registeredSubject is a registered family whose projection slot is filled.
func registeredSubject(declared core.FamilyDeclaration, slot aggregate.Projection) projectionSubject {
	return projectionSubject{
		name: declared.Name,
		section: func(t *testing.T, p *projectionInput) any {
			return p.registered(t)
		},
		metrics: func(t *testing.T, section any) any {
			return familyMetrics(t, section.(*core.Families), declared.Namespace)
		},
		project: func(section any, scope core.Scope) any {
			return slot(*section.(*core.Families), scope)
		},
	}
}

// familyMetrics returns the metrics a report's families hold under a
// namespace, as the report writes them.
func familyMetrics(t *testing.T, families *core.Families, namespace string) any {
	t.Helper()
	data, err := json.Marshal(families)
	if err != nil {
		fatal(t, 78, "encoding the families: %v", err)
	}
	var sections map[string]struct {
		Metrics json.RawMessage `json:"metrics"`
	}
	if err := json.Unmarshal(data, &sections); err != nil {
		fatal(t, 78, "decoding the families: %v", err)
	}
	section, ok := sections[namespace]
	if !ok {
		fatal(t, 78, "the report holds no family under %s", namespace)
	}
	return section.Metrics
}

// withoutCells returns metrics as the report writes them, cells left out.
func withoutCells(t *testing.T, metrics any) map[string]any {
	t.Helper()
	data, err := json.Marshal(metrics)
	if err != nil {
		fatal(t, 78, "encoding metrics: %v", err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		fatal(t, 78, "metrics %s are not an object: %v", data, err)
	}
	delete(out, "cells")
	return out
}

// projectionDifference returns how the metrics got differ from want, cells
// aside, or nothing where they are equal.
func projectionDifference(t *testing.T, want, got any) string {
	t.Helper()
	a, b := withoutCells(t, want), withoutCells(t, got)
	if reflect.DeepEqual(a, b) {
		return ""
	}
	wantText, _ := json.MarshalIndent(a, "", "  ")
	gotText, _ := json.MarshalIndent(b, "", "  ")
	return unifiedDiff(string(wantText), string(gotText))
}

// The three dimensions of ADR-0078 clause 7, as projectionViolations names
// them.
const (
	dimensionRepository = "repository"
	dimensionIdentities = "identities"
	dimensionYears      = "years"
)

// projectionViolations compares a subject over one fixture in the three
// dimensions of ADR-0078 clause 7, and returns by dimension what differs and
// how many comparisons were made.
func projectionViolations(t *testing.T, s projectionSubject, f projectionFixture) (map[string][]string, map[string]int) {
	t.Helper()
	violations, compared := map[string][]string{}, map[string]int{}
	add := func(dimension, format string, args ...any) {
		violations[dimension] = append(violations[dimension], fmt.Sprintf(format, args...))
	}
	base := s.section(t, f.base.input)
	table := f.base.input.in.Identities

	compared[dimensionRepository]++
	if diff := projectionDifference(t, s.metrics(t, base), s.project(base, core.Scope{})); diff != "" {
		add(dimensionRepository, "%s: the %s projection for every identity and every year differs from the metrics "+
			"the report holds, cells aside (- report, + projection):\n%s", f.name, s.name, diff)
	}

	for _, pair := range f.pairs {
		both, err := core.NewScope(table, []core.CellIdentity{{ID: pair.a}, {ID: pair.b}}, 0)
		if err != nil {
			fatal(t, 78, "%s: selecting %s and %s: %v", f.name, pair.a, pair.b, err)
		}
		one, err := core.NewScope(pair.merged.input.in.Identities, []core.CellIdentity{{ID: pair.a}}, 0)
		if err != nil {
			fatal(t, 78, "%s: selecting the identity %s and %s become: %v", f.name, pair.a, pair.b, err)
		}
		compared[dimensionIdentities]++
		recomputed := s.project(s.section(t, pair.merged.input), one)
		if diff := projectionDifference(t, recomputed, s.project(base, both)); diff != "" {
			add(dimensionIdentities, "%s: the %s projection for %s and %s differs from the projection for the one "+
				"identity they become when the configuration merges them (- merged, + projection):\n%s",
				f.name, s.name, pair.a, pair.b, diff)
		}
	}

	for _, year := range f.years {
		scope, err := core.NewScope(table, nil, year)
		if err != nil {
			fatal(t, 78, "%s: selecting %d: %v", f.name, year, err)
		}
		compared[dimensionYears]++
		recomputed := s.metrics(t, s.section(t, f.restricted[year]))
		if diff := projectionDifference(t, recomputed, s.project(base, scope)); diff != "" {
			add(dimensionYears, "%s: the %s projection for %d differs from the family's section built over that "+
				"year's commits alone, cells aside (- recomputed, + projection):\n%s", f.name, s.name, year, diff)
		}
	}
	return violations, compared
}

// TestProjectionEqualsRecomputation holds every family that fills its
// projection slot to ADR-0078 clause 7 on the fixtures basic, mailmap, merges
// and multi-year-gap, and the filled slots to the families with cells. While
// no family fills its slot the fixtures are still analysed, merged and
// restricted, so the machinery the comparison runs on is exercised and held
// to the stage's own input.
func TestProjectionEqualsRecomputation(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	withCells := map[string]bool{}
	for family, section := range catalogueCellSections(repo.read(t, 62, metricsCatalogue)) {
		if section.cellFields != nil {
			withCells[family] = true
		}
	}
	slots := aggregate.Projections()
	for _, family := range sortedKeys(slots) {
		if !withCells[family] {
			report(t, 78, "the family %s fills its projection slot, and its section in %s has no Cell field table, "+
				"so it has no cells to project", family, metricsCatalogue)
		}
	}
	for _, family := range sortedKeys(withCells) {
		if slots[family] == nil {
			report(t, 78, "the family %s carries cells and leaves its projection slot empty, so nothing holds its "+
				"projections to recomputation (clause 7)", family)
		}
	}

	var subjects []projectionSubject
	var names []string
	for _, family := range aggregate.Registry() {
		declared := family.Declaration()
		if slot := slots[declared.Name]; slot != nil {
			subjects = append(subjects, registeredSubject(declared, slot))
			names = append(names, declared.Name)
		}
	}
	t.Logf("families checked: %v", names)

	for _, fixture := range projectionFixtures() {
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()
			f := prepareProjectionFixture(t, repo, fixture)
			t.Logf("%d pairs of identities and the years %v", len(f.pairs), f.years)
			for _, s := range subjects {
				violations, compared := projectionViolations(t, s, f)
				t.Logf("%s: compared for every identity, for %d pairs of identities and for %d years", s.name,
					compared[dimensionIdentities], compared[dimensionYears])
				for _, dimension := range []string{dimensionRepository, dimensionIdentities, dimensionYears} {
					for _, v := range violations[dimension] {
						report(t, 78, "%s", v)
					}
				}
			}
		})
	}
}

// The scratch family of TestProjectionRejectsANonAdditiveCell. Each cell holds
// the median effective lines of its commits, and the projection takes the
// median of the medians its scope includes. A median of medians is not the
// median of the union, so the cell is not additive (ADR-0078 clause 4) and the
// checker must refuse it. The family is not registered and changes no report.
type (
	scratchCell struct {
		core.CellIdentity
		Year        int     `json:"year"`
		MedianLines float64 `json:"median_lines"`
	}
	scratchMetrics struct {
		MedianLines *float64      `json:"median_lines,omitzero"`
		Cells       []scratchCell `json:"cells,omitzero"`
	}
)

// scratchSection builds the scratch family's section over an input: the
// median effective lines of its non-bulk commits, and a cell for each cell
// identity and year.
func scratchSection(in core.Input) scratchMetrics {
	type key struct {
		cell core.CellIdentity
		year int
	}
	lines := map[key][]int{}
	var all []int
	for _, c := range in.LineScoped() {
		k := key{in.Identities.Cell(c.IdentityID), core.CommitYear(c)}
		lines[k] = append(lines[k], c.EffectiveLines)
		all = append(all, c.EffectiveLines)
	}
	var out scratchMetrics
	if len(all) > 0 {
		median := core.Median(all)
		out.MedianLines = &median
	}
	for k, l := range lines {
		out.Cells = append(out.Cells, scratchCell{CellIdentity: k.cell, Year: k.year, MedianLines: core.Median(l)})
	}
	sort.Slice(out.Cells, func(i, j int) bool {
		a, b := out.Cells[i], out.Cells[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Year < b.Year
	})
	return out
}

// scratchProject is the scratch family's projection: the median of the
// medians of the cells a scope includes.
func scratchProject(m scratchMetrics, scope core.Scope) scratchMetrics {
	var medians []float64
	for _, c := range m.Cells {
		if scope.IncludesIdentity(c.CellIdentity) && scope.IncludesYear(c.Year) {
			medians = append(medians, c.MedianLines)
		}
	}
	if len(medians) == 0 {
		return scratchMetrics{}
	}
	sort.Float64s(medians)
	median := medians[len(medians)/2]
	if len(medians)%2 == 0 {
		median = (medians[len(medians)/2-1] + medians[len(medians)/2]) / 2
	}
	return scratchMetrics{MedianLines: &median}
}

// TestProjectionRejectsANonAdditiveCell is the failure demonstration ADR-0064
// clause 6 requires. The scratch family runs through the checker's comparison
// on the fixture basic, and each of the three dimensions must refuse it.
func TestProjectionRejectsANonAdditiveCell(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	scratch := projectionSubject{
		name: "scratch",
		section: func(_ *testing.T, p *projectionInput) any {
			return scratchSection(p.in)
		},
		metrics: func(_ *testing.T, section any) any {
			return section
		},
		project: func(section any, scope core.Scope) any {
			return scratchProject(section.(scratchMetrics), scope)
		},
	}
	violations, compared := projectionViolations(t, scratch, prepareProjectionFixture(t, repo, "basic"))
	for _, dimension := range []string{dimensionRepository, dimensionIdentities, dimensionYears} {
		if compared[dimension] == 0 {
			report(t, 64, "basic gave nothing to compare in the %s dimension, so the checker's refusal there is "+
				"not shown", dimension)
		}
		if len(violations[dimension]) == 0 {
			report(t, 64, "the scratch family's median of medians was accepted in the %s dimension on basic, so the "+
				"checker cannot refuse a cell that does not sum", dimension)
		}
		for _, v := range violations[dimension] {
			t.Logf("refused as required: %.200s", v)
		}
	}
}
