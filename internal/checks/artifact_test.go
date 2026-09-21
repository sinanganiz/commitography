package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/coupling"
	"github.com/sinanganiz/commitography/internal/metrics/messages"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// The collect artifact checkers (WP-0012). The collect stage's output is the
// normalized commit records, written as an independently cacheable artifact
// (ADR-0020 clause 2). Five properties are held here:
//
//   - writing the artifact and reading it back yields identical records;
//   - a consumer of commit records alone runs from it with the repository
//     unreadable. The whole aggregate stage is not such a consumer yet: it
//     lists the tree and reads the working tree until WP-0013 moves both to
//     replay, and WP-0061 proves the whole stage runs without the repository;
//   - the stage writes into no family namespace, because it computes no
//     metric (ADR-0020 clause 2);
//   - no route, output path or exported artifact reads it, because it carries
//     raw addresses (ADR-0033 clause 1) and must not become an export by
//     accident;
//   - the stage reads no file from the working tree, which ADR-0020 clause 3
//     reserves for replay.

// collectedFixtures returns the fixtures the analysis produces a report for:
// the ones whose golden file is a report rather than a refusal.
func collectedFixtures(t *testing.T, repo repository) []string {
	t.Helper()
	var out []string
	for _, fixture := range generatedFixtures(t, repo) {
		if repo.isTracked(goldenFile(fixture, false)) {
			out = append(out, fixture)
		}
	}
	if len(out) == 0 {
		fatal(t, 64, "no fixture produces a report; the gates generate them with `make fixtures`")
	}
	return out
}

// collectFixture runs the collect stage on a directory under the built-in
// analysis plane, as the pipeline runs it.
func collectFixture(t *testing.T, collector *collect.Collector, dir string) *model.History {
	t.Helper()
	cfg := config.Default()
	history, err := collector.Collect(collect.Options{RepoPath: dir, Analysis: &cfg, Context: context.Background()})
	if err != nil {
		fatal(t, 20, "collecting %s: %v", filepath.Base(dir), err)
	}
	return history
}

// ---------------------------------------------------------------- round trip

// recordDifference returns how two histories differ, or the empty string when
// every record is identical. Times are compared as instants and by the offset
// they carry, which is what a local time is; everything else is compared as
// it stands, so a value lost on the way through the file is a difference even
// where the file format would not have carried it.
func recordDifference(a, b *model.History) string {
	normalise := func(h *model.History) (*model.History, []string) {
		out := *h
		out.Commits = append([]model.Commit(nil), h.Commits...)
		offsets := make([]string, 0, 3*len(out.Commits)+1)
		zone := func(at time.Time) string { return at.Format(time.RFC3339Nano) }
		offsets = append(offsets, zone(out.GeneratedAt))
		out.GeneratedAt = out.GeneratedAt.UTC()
		for i := range out.Commits {
			c := &out.Commits[i]
			offsets = append(offsets, zone(c.AuthorDate), zone(c.CommitterDate), zone(c.LocalTime))
			c.AuthorDate, c.CommitterDate, c.LocalTime = c.AuthorDate.UTC(), c.CommitterDate.UTC(), c.LocalTime.UTC()
		}
		return &out, offsets
	}
	na, oa := normalise(a)
	nb, ob := normalise(b)
	if !reflect.DeepEqual(oa, ob) {
		return "a time lost its instant or the offset it was recorded in"
	}
	if len(na.Commits) != len(nb.Commits) {
		return fmt.Sprintf("%d records written, %d read back", len(na.Commits), len(nb.Commits))
	}
	for i := range na.Commits {
		if !reflect.DeepEqual(na.Commits[i], nb.Commits[i]) {
			return fmt.Sprintf("record %d (%s) differs:\n  written %+v\n  read    %+v",
				i, na.Commits[i].Hash, na.Commits[i], nb.Commits[i])
		}
	}
	na.Commits, nb.Commits = nil, nil
	if !reflect.DeepEqual(na, nb) {
		return fmt.Sprintf("the history's own fields differ:\n  written %+v\n  read    %+v", na, nb)
	}
	return ""
}

// TestCollectArtifactRoundTrip writes every fixture's artifact and reads it
// back (WP-0012 clause 6).
func TestCollectArtifactRoundTrip(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	for _, fixture := range collectedFixtures(t, repo) {
		written := collectFixture(t, newCollector(), filepath.Join(repo.root, "testdata", "fixtures", fixture))
		path := filepath.Join(t.TempDir(), "history.json")
		if err := collect.WriteHistory(written, path); err != nil {
			fatal(t, 20, "writing the %s artifact: %v", fixture, err)
		}
		read, err := collect.ReadHistory(path)
		if err != nil {
			fatal(t, 20, "reading the %s artifact back: %v", fixture, err)
		}
		if diff := recordDifference(written, read); diff != "" {
			report(t, 20, "the %s artifact does not read back as it was written: %s", fixture, diff)
		}
	}
}

// TestCollectArtifactRoundTripRejectsALostValue is the failure demonstration
// ADR-0064 clause 6 requires: a value or a local offset lost on the way back
// is found.
func TestCollectArtifactRoundTripRejectsALostValue(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	written := collectFixture(t, newCollector(), filepath.Join(repo.root, "testdata", "fixtures", "basic"))
	if len(written.Commits) == 0 {
		fatal(t, 64, "the basic fixture has no commits, so nothing can be lost")
	}
	for name, lose := range map[string]func(*model.History){
		"effective lines":   func(h *model.History) { h.Commits[0].EffectiveLines++ },
		"an excluded path":  func(h *model.History) { h.Commits[0].Files[0].Excluded = !h.Commits[0].Files[0].Excluded },
		"the local offset":  func(h *model.History) { h.Commits[0].LocalTime = h.Commits[0].LocalTime.UTC() },
		"the attributes":    func(h *model.History) { h.Attributes += "x" },
		"a record entirely": func(h *model.History) { h.Commits = h.Commits[1:] },
	} {
		read := *written
		read.Commits = append([]model.Commit(nil), written.Commits...)
		read.Commits[0].Files = append([]model.FileChange(nil), written.Commits[0].Files...)
		if len(read.Commits[0].Files) == 0 {
			fatal(t, 64, "the basic fixture's first record has no files")
		}
		lose(&read)
		if recordDifference(written, &read) == "" {
			report(t, 64, "the round trip accepted a history that lost %s", name)
		}
	}
	if diff := recordDifference(written, written); diff != "" {
		report(t, 20, "the round trip refused a history identical to itself: %s", diff)
	}
}

// ------------------------------------------------- a consumer of the records

// recordConsumerFixtures are the fixtures the consumer runs on: one with the
// attributes and generated paths the path filter must reproduce, and one
// without.
func recordConsumerFixtures() []string { return []string{"noise", "basic"} }

// recordConsumerFamilies runs the families that consume commit records alone
// (ADR-0024 clause 5) from an artifact, and returns each family's metrics as
// the report writes them. pathFilter builds the path filter the families are
// given.
func recordConsumerFamilies(t *testing.T, history *model.History, repoPath string,
	pathFilter func(config.Analysis) *filter.PathFilter) map[string]string {
	t.Helper()
	cfg := config.Default()
	cfg.CardinalityLimits = core.LimitValues()
	in := core.Input{
		Context:    context.Background(),
		RepoPath:   repoPath,
		Repository: history.Repository,
		Config:     cfg,
		Filtered:   filter.Summarize(history.Commits),
		Resolver:   identity.NewResolver(cfg, history.Commits),
		PathFilter: pathFilter(cfg),
	}
	couplingFamily, _ := coupling.Build(core.ScopedCommits(in, in.LineScoped()))
	families := map[string]any{
		"temporal":    temporal.Build(in, in.Analyzed()).Metrics,
		"commit-size": commitsize.Build(in, in.LineScoped()).Metrics,
		"messages":    messages.Build(in.Analyzed()).Metrics,
		"coupling":    couplingFamily.Metrics,
	}
	out := map[string]string{}
	for name, metrics := range families {
		data, err := json.Marshal(metrics)
		if err != nil {
			fatal(t, 20, "encoding the %s family: %v", name, err)
		}
		out[name] = canonicalJSON(t, name, data)
	}
	return out
}

// canonicalJSON re-encodes a JSON value with its object keys sorted, so two
// encodings of one value compare equal whatever order their fields were
// written in.
func canonicalJSON(t *testing.T, name string, data []byte) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		fatal(t, 20, "reading the %s family: %v", name, err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		fatal(t, 20, "encoding the %s family: %v", name, err)
	}
	return string(canonical)
}

// reportFamilies returns the metrics of the named families in a report.
func reportFamilies(t *testing.T, produced string, names map[string]string) map[string]string {
	t.Helper()
	var document struct {
		Families map[string]struct {
			Metrics json.RawMessage `json:"metrics"`
		} `json:"families"`
	}
	if err := json.Unmarshal([]byte(produced), &document); err != nil {
		fatal(t, 20, "reading the report: %v", err)
	}
	out := map[string]string{}
	for name := range names {
		family, ok := document.Families[name]
		if !ok {
			fatal(t, 20, "the report carries no %s family", name)
		}
		out[name] = canonicalJSON(t, name, family.Metrics)
	}
	return out
}

// copyTree copies a directory, so that the copy can be made unreadable
// without touching the fixture every other checker reads.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer source.Close()
		destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(destination, source); err != nil {
			destination.Close()
			return err
		}
		return destination.Close()
	})
	if err != nil {
		fatal(t, 64, "copying %s: %v", from, err)
	}
}

// consumerRun analyses a copy of a fixture, collects it into an artifact, and
// then makes the repository unreadable by moving it away. It returns the
// report, the artifact read back, and the path the repository was at.
func consumerRun(t *testing.T, repo repository, fixture string) (string, *model.History, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), fixture)
	copyTree(t, filepath.Join(repo.root, "testdata", "fixtures", fixture), dir)
	produced := analyseWith(t, pipeline.Options{RepoPath: dir}, core.FixedClock(checkTime()))

	artifact := filepath.Join(t.TempDir(), "history.json")
	if err := collect.WriteHistory(collectFixture(t, newCollector(), dir), artifact); err != nil {
		fatal(t, 20, "writing the %s artifact: %v", fixture, err)
	}
	if err := os.Rename(dir, dir+"-gone"); err != nil {
		fatal(t, 64, "making the %s repository unreadable: %v", fixture, err)
	}
	if _, err := os.Stat(dir); err == nil {
		fatal(t, 64, "the %s repository is still readable, so running without it proves nothing", fixture)
	}
	history, err := collect.ReadHistory(artifact)
	if err != nil {
		fatal(t, 20, "reading the %s artifact: %v", fixture, err)
	}
	return produced, history, dir
}

// fromArtifact is the path filter a consumer of the artifact builds: from the
// attributes the artifact carries.
func fromArtifact(t *testing.T, history *model.History) func(config.Analysis) *filter.PathFilter {
	return func(cfg config.Analysis) *filter.PathFilter {
		pf, err := filter.NewPathFilterFromAttributes(cfg, []byte(history.Attributes))
		if err != nil {
			fatal(t, 20, "building the path filter from the artifact: %v", err)
		}
		return pf
	}
}

// TestCollectArtifactServesACommitRecordConsumer runs the families that need
// only commit records from the artifact, with the repository gone, and
// requires the values the full analysis produced (WP-0012 clause 6).
func TestCollectArtifactServesACommitRecordConsumer(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	for _, fixture := range recordConsumerFixtures() {
		produced, history, gone := consumerRun(t, repo, fixture)
		got := recordConsumerFamilies(t, history, gone, fromArtifact(t, history))
		want := reportFamilies(t, produced, got)
		for name := range got {
			if got[name] != want[name] {
				report(t, 20, "the %s family computed from the %s artifact without the repository differs from "+
					"the analysis:\n  analysis %s\n  artifact %s", name, fixture, want[name], got[name])
			}
		}
	}
}

// TestCollectArtifactRejectsAConsumerThatReadsTheRepository is the failure
// demonstration: a consumer that reaches for the repository instead of the
// artifact — here, for the attributes — gets other values, and the
// comparison above says so.
func TestCollectArtifactRejectsAConsumerThatReadsTheRepository(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	produced, history, gone := consumerRun(t, repo, "noise")
	if history.Attributes == "" {
		fatal(t, 64, "the noise fixture's artifact carries no attributes, so reading them elsewhere changes nothing")
	}
	fromRepository := func(cfg config.Analysis) *filter.PathFilter {
		pf, err := filter.NewPathFilter(core.SystemFilesystem(), cfg, gone)
		if err != nil {
			fatal(t, 64, "building the path filter from the repository: %v", err)
		}
		return pf
	}
	got := recordConsumerFamilies(t, history, gone, fromRepository)
	want := reportFamilies(t, produced, got)
	differs := false
	for name := range got {
		if got[name] != want[name] {
			differs = true
		}
	}
	if !differs {
		report(t, 64, "a consumer that read the attributes from the missing repository matched the analysis, so "+
			"the check cannot tell it from one that uses the artifact")
	}
}

// --------------------------------------------------------- family namespace

const modulePath = "github.com/sinanganiz/commitography"

// reportTypeNames returns the names of the core types a report is built from:
// every type reachable from core.Report that core defines, with generic
// arguments dropped, and the functions that construct a family or a section.
// Code that names none of them cannot write into the report.
func reportTypeNames() map[string]bool {
	names := map[string]bool{
		"Computed": true, "Skipped": true, "EmbedConfiguration": true,
		"DocumentVersion": true, "CurrentSectionVersions": true,
	}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		if seen[t] {
			return
		}
		seen[t] = true
		if t.PkgPath() == modulePath+"/internal/core" && t.Name() != "" {
			name, _, _ := strings.Cut(t.Name(), "[")
			names[name] = true
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				walk(t.Field(i).Type)
			}
		}
	}
	walk(reflect.TypeOf(core.Report{}))
	return names
}

// familyWrites returns what a parsed file does that could write into a family
// namespace: importing a metric family, or naming a core type or constructor
// a report is built from.
func familyWrites(fset *token.FileSet, parsed *ast.File, reportNames map[string]bool) []string {
	var out []string
	local := map[string]string{}
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if strings.HasPrefix(path, modulePath+"/internal/metrics") {
			out = append(out, fmt.Sprintf("%s imports the metric family %s", fset.Position(spec.Pos()), path))
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		local[name] = path
	}
	ast.Inspect(parsed, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && local[pkg.Name] == modulePath+"/internal/core" &&
			reportNames[selector.Sel.Name] {
			out = append(out, fmt.Sprintf("%s names core.%s, which a report is built from",
				fset.Position(selector.Pos()), selector.Sel.Name))
		}
		return true
	})
	return out
}

// coreTypesIn returns every core type reachable from t: the artifact must
// hold none, because core's types are the report's.
func coreTypesIn(t reflect.Type) []string {
	var out []string
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		if seen[t] {
			return
		}
		seen[t] = true
		if t.PkgPath() == modulePath+"/internal/core" {
			out = append(out, t.String())
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				walk(t.Field(i).Type)
			}
		}
	}
	walk(t)
	return out
}

// TestCollectArtifactWritesNoFamilyNamespace holds the collect stage to
// computing no metric (ADR-0020 clause 2, WP-0012 clause 2): none of its
// source names a metric family or a type a report is built from, and nothing
// its artifact holds is one.
func TestCollectArtifactWritesNoFamilyNamespace(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	names := reportTypeNames()
	if !names["Families"] || !names["TemporalMetrics"] {
		fatal(t, 64, "the report's types were not found, so no family write could be recognised")
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, "internal/pipeline/collect/") || !strings.HasSuffix(file, ".go") ||
			strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 20, file), 0)
		if err != nil {
			fatal(t, 20, "cannot parse %s: %v", file, err)
		}
		scanned++
		for _, finding := range familyWrites(fset, parsed, names) {
			report(t, 20, "the collect stage computes no metric: %s", finding)
		}
	}
	if scanned == 0 {
		fatal(t, 64, "no source of the collect stage was found, so nothing was checked")
	}
	for _, found := range coreTypesIn(reflect.TypeOf(model.History{})) {
		report(t, 20, "the collect artifact holds %s, which is one of the report's types", found)
	}
}

// TestCollectArtifactNamespaceCheckRejectsAFamilyWrite is the failure
// demonstration: a stage that runs a family and places its output in a report
// is found, and one that only raises an error is not.
func TestCollectArtifactNamespaceCheckRejectsAFamilyWrite(t *testing.T) {
	t.Parallel()
	names := reportTypeNames()
	parse := func(source string) []string {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, "collect.go", source, 0)
		if err != nil {
			fatal(t, 64, "parsing a demonstration source: %v", err)
		}
		return familyWrites(fset, parsed, names)
	}
	writes := parse(`package collect
import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/metrics/temporal"
)
func f(r *core.Report) { r.Families.Temporal = temporal.Build(core.Input{}, nil) }
`)
	if len(writes) < 2 {
		report(t, 64, "the namespace check found %d of the two family writes in a demonstration source: %v",
			len(writes), writes)
	}
	if clean := parse(`package collect
import "github.com/sinanganiz/commitography/internal/core"
func f() error { return core.Internalf(nil, "reading %s", core.ReasonShallowClone) }
`); len(clean) != 0 {
		report(t, 20, "the namespace check refused a source that writes nothing into a report: %v", clean)
	}
	type leaky struct{ Report core.Report }
	if len(coreTypesIn(reflect.TypeOf(leaky{}))) == 0 {
		report(t, 64, "the artifact check accepted a type holding a report")
	}
}

// ------------------------------------------------------------- not exported

// exportPaths are the places an artifact leaves the machine from: the
// command's output, the server's routes, and the renderer.
func exportPaths() []string {
	return []string{"cmd/", "internal/server/", "internal/pipeline/render/"}
}

// artifactReads returns where a parsed file reads or names the collect
// artifact: its writer and reader, or the record types it is made of.
func artifactReads(fset *token.FileSet, parsed *ast.File) []string {
	forbidden := map[string]map[string]bool{
		modulePath + "/internal/pipeline/collect": {"WriteHistory": true, "ReadHistory": true},
		modulePath + "/internal/core/model":       {"History": true, "Commit": true, "FileChange": true},
	}
	local := map[string]string{}
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		local[name] = path
	}
	var out []string
	ast.Inspect(parsed, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && forbidden[local[pkg.Name]][selector.Sel.Name] {
			out = append(out, fmt.Sprintf("%s names %s.%s", fset.Position(selector.Pos()), pkg.Name,
				selector.Sel.Name))
		}
		return true
	})
	return out
}

// artifactTypesIn returns the collect artifact's types reachable from t.
func artifactTypesIn(t reflect.Type) []string {
	artifact := map[reflect.Type]bool{
		reflect.TypeOf(model.History{}):    true,
		reflect.TypeOf(model.Commit{}):     true,
		reflect.TypeOf(model.FileChange{}): true,
	}
	var out []string
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		if seen[t] {
			return
		}
		seen[t] = true
		if artifact[t] {
			out = append(out, t.String())
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				walk(t.Field(i).Type)
			}
		}
	}
	walk(t)
	return out
}

// TestCollectArtifactIsReadByNoExport requires that no route, output path or
// exported artifact reads the collect artifact (WP-0012 clause 8): no source
// that exports anything names its reader, its writer or its records, and
// neither the report nor the result the command and the server receive can
// hold one.
func TestCollectArtifactIsReadByNoExport(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	scanned := 0
	for _, file := range repo.tracked {
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		exported := false
		for _, prefix := range exportPaths() {
			if strings.HasPrefix(file, prefix) {
				exported = true
			}
		}
		if !exported {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 33, file), 0)
		if err != nil {
			fatal(t, 33, "cannot parse %s: %v", file, err)
		}
		scanned++
		for _, finding := range artifactReads(fset, parsed) {
			report(t, 33, "the collect artifact carries raw addresses and is never exported: %s", finding)
		}
	}
	if scanned == 0 {
		fatal(t, 64, "no source under %v was found, so nothing was checked", exportPaths())
	}
	for _, exported := range []reflect.Type{reflect.TypeOf(core.Report{}), reflect.TypeOf(pipeline.Result{})} {
		for _, found := range artifactTypesIn(exported) {
			report(t, 33, "%s can hold %s, so the collect artifact could be exported through it", exported, found)
		}
	}
}

// TestCollectArtifactExportCheckRejectsAReader is the failure demonstration:
// a route that reads the artifact, and a result type that carries its
// records, are both found.
func TestCollectArtifactExportCheckRejectsAReader(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "handler.go", `package server
import (
	c "github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/core/model"
)
func serve(path string) []model.Commit { h, _ := c.ReadHistory(path); return h.Commits }
`, 0)
	if err != nil {
		fatal(t, 64, "parsing a demonstration source: %v", err)
	}
	if found := artifactReads(fset, parsed); len(found) < 2 {
		report(t, 64, "the export check found %d of the two reads in a demonstration route: %v", len(found), found)
	}
	type leaky struct{ Records []model.Commit }
	if len(artifactTypesIn(reflect.TypeOf(leaky{}))) == 0 {
		report(t, 64, "the export check accepted a result type that carries commit records")
	}
}

// -------------------------------------------------------------- working tree

// recordingFilesystem is core.Filesystem with every path it is asked for
// written down.
type recordingFilesystem struct {
	inner core.Filesystem
	mu    sync.Mutex
	paths []string
}

func (r *recordingFilesystem) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, name)
}

func (r *recordingFilesystem) Open(name string) (fs.File, error) {
	r.record(name)
	return r.inner.Open(name)
}

func (r *recordingFilesystem) ReadFile(name string) ([]byte, error) {
	r.record(name)
	return r.inner.ReadFile(name)
}

func (r *recordingFilesystem) Stat(name string) (fs.FileInfo, error) {
	r.record(name)
	return r.inner.Stat(name)
}

// workingTreeReads returns the paths that lie outside the git directory: in
// the working tree, or anywhere else a stage has no business reading.
func workingTreeReads(paths []string, gitDir string) []string {
	var out []string
	for _, path := range paths {
		rel, err := filepath.Rel(filepath.Clean(gitDir), filepath.Clean(path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			out = append(out, path)
		}
	}
	return out
}

// TestCollectArtifactIsBuiltWithoutTheWorkingTree requires the collect stage
// to read no file from the working tree (WP-0012 clause 10c, ADR-0020
// clause 3). Every file it reads goes through the filesystem it is given, and
// every path it asked for must lie inside the git directory. Its source may
// reach the filesystem directly only to write and read its own artifact, and
// never through the path filter that reads a directory.
//
// What git itself reads while collect runs it is git's: git consults the
// working tree's attributes for binary detection and its mailmap, as its own
// rules say, and no option of the supported git versions directs it elsewhere.
func TestCollectArtifactIsBuiltWithoutTheWorkingTree(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	for _, fixture := range []string{"noise", "basic"} {
		dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
		recorder := &recordingFilesystem{inner: core.SystemFilesystem()}
		collectFixture(t, collect.New(core.FixedClock(checkTime()), recorder), dir)
		gitDir, err := git.Output(context.Background(), git.At(dir, "rev-parse", "--absolute-git-dir"))
		if err != nil {
			fatal(t, 20, "locating the %s fixture's git directory: %v", fixture, err)
		}
		if len(recorder.paths) == 0 {
			fatal(t, 64, "the collect stage asked the filesystem it was given for nothing, so the recorder is not "+
				"where the stage reads")
		}
		for _, path := range workingTreeReads(recorder.paths, gitDir) {
			report(t, 20, "the collect stage read %s, outside the %s fixture's git directory", path, fixture)
		}
	}

	fset := token.NewFileSet()
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, "internal/pipeline/collect/") || !strings.HasSuffix(file, ".go") ||
			strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 20, file), 0)
		if err != nil {
			fatal(t, 20, "cannot parse %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			selector, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch {
			case pkg.Name == "os" && file != "internal/pipeline/collect/writer.go":
				report(t, 20, "%s reaches the filesystem directly with os.%s; the stage reads through the "+
					"filesystem it is given", fset.Position(selector.Pos()), selector.Sel.Name)
			case pkg.Name == "filter" && selector.Sel.Name == "NewPathFilter":
				report(t, 20, "%s builds a path filter from a directory; the stage reads the analysed commit's "+
					"attributes through git", fset.Position(selector.Pos()))
			}
			return true
		})
	}
}

// TestCollectArtifactWorkingTreeCheckRejectsAWorkingTreeRead is the failure
// demonstration for the path half: a path in the working tree is found, and
// one in the git directory is not.
func TestCollectArtifactWorkingTreeCheckRejectsAWorkingTreeRead(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repo")
	gitDir := filepath.Join(root, ".git")
	found := workingTreeReads([]string{
		filepath.Join(gitDir, "shallow"),
		filepath.Join(gitDir, "info", "grafts"),
		filepath.Join(root, ".gitattributes"),
		filepath.Join(root, ".git-lookalike", "x"),
	}, gitDir)
	if len(found) != 2 {
		report(t, 64, "the working-tree check found %v, want the two paths outside the git directory", found)
	}
}
