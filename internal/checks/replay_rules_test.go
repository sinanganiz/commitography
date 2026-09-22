package checks

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
	"github.com/sinanganiz/commitography/internal/pipeline/replay"
)

// The rules the replay stage is held to (WP-0013): the walk is sequential,
// replay reads no working tree and aggregation no repository, the serialised
// map carries no raw address, and its memory stays within a recorded budget.

// stageSources parses the non-test sources of a package directory.
func stageSources(t *testing.T, repo repository, fset *token.FileSet, dir string) []*ast.File {
	t.Helper()
	var out []*ast.File
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, dir+"/") || strings.Contains(strings.TrimPrefix(file, dir+"/"), "/") ||
			!strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, repo.read(t, 52, file), parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 52, "cannot parse %s: %v", file, err)
		}
		out = append(out, parsed)
	}
	if len(out) == 0 {
		fatal(t, 64, "%s has no source to check", dir)
	}
	return out
}

// concurrencyIn returns where a source could start a goroutine: a go
// statement, a call to a method named Go, as a wait group or an error group
// starts one, or an import of a package whose purpose is concurrency.
func concurrencyIn(fset *token.FileSet, parsed *ast.File) []string {
	var out []string
	for _, spec := range parsed.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); path == "sync" || path == "golang.org/x/sync/errgroup" {
			out = append(out, fmt.Sprintf("%s imports %s", fset.Position(spec.Pos()), path))
		}
	}
	ast.Inspect(parsed, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			out = append(out, fmt.Sprintf("%s starts a goroutine", fset.Position(node.Pos())))
		case *ast.CallExpr:
			if selector, ok := node.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Go" {
				out = append(out, fmt.Sprintf("%s starts a goroutine through %s", fset.Position(node.Pos()),
					selector.Sel.Name))
			}
		}
		return true
	})
	return out
}

// TestReplayWalkStartsNoGoroutine enforces ADR-0052 clause 2 and ADR-0073
// clause 9: the chronological walk is sequential. No source of the replay
// stage starts a goroutine or imports a package for starting and joining
// them. The git subprocess the stage reads objects through is I/O, owned and
// waited for by the git package (ADR-0044), and not part of the walk.
func TestReplayWalkStartsNoGoroutine(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	for _, parsed := range stageSources(t, repo, fset, "internal/pipeline/replay") {
		for _, finding := range concurrencyIn(fset, parsed) {
			report(t, 52, "the replay walk is sequential, and %s", finding)
		}
	}
}

// TestReplayWalkCheckRejectsAGoroutine is the failure demonstration ADR-0064
// clause 6 requires: each way of starting a goroutine is found.
func TestReplayWalkCheckRejectsAGoroutine(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	for name, source := range map[string]string{
		"a go statement": "package replay\nfunc walk(f func()) { go f() }\n",
		"a wait group":   "package replay\nimport \"sync\"\nfunc walk(f func()) { var wg sync.WaitGroup; wg.Go(f); wg.Wait() }\n",
		"an error group": "package replay\nimport \"golang.org/x/sync/errgroup\"\nfunc walk(g *errgroup.Group, f func() error) { g.Go(f) }\n",
	} {
		parsed, err := parser.ParseFile(fset, "walk.go", source, parser.SkipObjectResolution)
		if err != nil {
			fatal(t, 64, "parsing the demonstration of %s: %v", name, err)
		}
		if len(concurrencyIn(fset, parsed)) == 0 {
			report(t, 64, "the sequential walk check accepted %s", name)
		}
	}
	parsed, err := parser.ParseFile(fset, "walk.go", "package replay\nfunc walk(f func()) { f() }\n",
		parser.SkipObjectResolution)
	if err != nil {
		fatal(t, 64, "parsing the sequential demonstration: %v", err)
	}
	if found := concurrencyIn(fset, parsed); len(found) != 0 {
		report(t, 52, "the sequential walk check refused a sequential walk: %v", found)
	}
}

// fileAccessIn returns the imports through which a source could read a file:
// the filesystem packages and, where git is forbidden too, the git package.
func fileAccessIn(fset *token.FileSet, parsed *ast.File, orGit bool) []string {
	forbidden := map[string]bool{"os": true, "io/fs": true, "io/ioutil": true, "path/filepath": true}
	if orGit {
		forbidden[modulePath+"/internal/git"] = true
	}
	var out []string
	for _, spec := range parsed.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); forbidden[path] {
			out = append(out, fmt.Sprintf("%s imports %s", fset.Position(spec.Pos()), path))
		}
	}
	return out
}

// TestReplayReadsNoWorkingTree enforces ADR-0073 clause 8 and ADR-0020
// clause 3 at the level of what each stage can reach. The replay stage reads
// file contents from git objects alone, so no source of it imports a package
// that opens a file. The aggregation stage reads neither the working tree nor
// the repository, so none of its sources imports a filesystem package or the
// git package.
func TestReplayReadsNoWorkingTree(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	fset := token.NewFileSet()
	for _, parsed := range stageSources(t, repo, fset, "internal/pipeline/replay") {
		for _, finding := range fileAccessIn(fset, parsed, false) {
			report(t, 73, "replay reads file contents from git objects, never the working tree, and %s", finding)
		}
	}
	for _, parsed := range stageSources(t, repo, fset, "internal/pipeline/aggregate") {
		for _, finding := range fileAccessIn(fset, parsed, true) {
			report(t, 20, "aggregation reads neither the working tree nor the repository, and %s", finding)
		}
	}
}

// TestReplayWorkingTreeCheckRejectsAFileRead is the failure demonstration.
func TestReplayWorkingTreeCheckRejectsAFileRead(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	parse := func(source string) *ast.File {
		parsed, err := parser.ParseFile(fset, "stage.go", source, 0)
		if err != nil {
			fatal(t, 64, "parsing a demonstration source: %v", err)
		}
		return parsed
	}
	reads := parse("package replay\nimport \"os\"\nfunc read(p string) ([]byte, error) { return os.ReadFile(p) }\n")
	if len(fileAccessIn(fset, reads, false)) == 0 {
		report(t, 64, "the working-tree check accepted a stage that opens a file")
	}
	lists := parse("package aggregate\nimport \"" + modulePath + "/internal/git\"\nvar _ = git.Records\n")
	if len(fileAccessIn(fset, lists, true)) == 0 {
		report(t, 64, "the working-tree check accepted an aggregation stage that runs git")
	}
	if len(fileAccessIn(fset, lists, false)) != 0 {
		report(t, 73, "the working-tree check refused the replay stage its git package")
	}
}

// bareClone makes a bare repository from a fixture by copying its git
// directory, which needs no transport: the git package allows none
// (ADR-0065 clause 2). The copy is named as a bare clone is, <name>.git.
func bareClone(t *testing.T, dir string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), filepath.Base(dir)+".git")
	if err := os.CopyFS(bare, os.DirFS(filepath.Join(dir, ".git"))); err != nil {
		fatal(t, 73, "copying %s's git directory: %v", filepath.Base(dir), err)
	}
	if _, err := git.Output(context.Background(), git.At(bare, "config", "core.bare", "true")); err != nil {
		fatal(t, 73, "making the copy of %s bare: %v", filepath.Base(dir), err)
	}
	if out, err := git.Output(context.Background(), git.At(bare, "rev-parse", "--is-bare-repository")); err != nil ||
		out != "true" {
		fatal(t, 73, "the copy of %s is not a bare repository: %q, %v", filepath.Base(dir), out, err)
	}
	return bare
}

// TestReplayCompletesOnABareClone requires an analysis of a bare clone, which
// has no working tree at all, to complete and to produce the report the
// fixture's golden file holds (ADR-0073 clause 8).
func TestReplayCompletesOnABareClone(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	for _, fixture := range []string{"basic", "noise", "binary", "merged-side-branch"} {
		bare := bareClone(t, fixtureDir(t, repo, fixture))
		result, err := newAnalyzer().Run(context.Background(), pipeline.Options{RepoPath: bare}, nil)
		if err != nil {
			report(t, 73, "analysing a bare clone of %s: %v", fixture, err)
			continue
		}
		out := filepath.Join(t.TempDir(), render.ReportFile)
		if err := render.WriteReportJSON(result.Report, out); err != nil {
			fatal(t, 73, "writing the report of a bare clone of %s: %v", fixture, err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			fatal(t, 73, "reading the report of a bare clone of %s: %v", fixture, err)
		}
		got := normalise(string(data), filepath.Dir(bare))
		want := strings.ReplaceAll(repo.read(t, 73, goldenFile(fixture, false)), "\r\n", "\n")
		if got != want {
			report(t, 73, "a bare clone of %s gives a report other than its golden file's (- golden, + bare):\n%s",
				fixture, unifiedDiff(want, got))
		}
	}
}

// rawAddresses returns every address a history records, before and after
// .mailmap, lowercased.
func rawAddresses(run replayRun) []string {
	seen := map[string]bool{}
	for _, c := range run.history.Commits {
		for _, address := range []string{c.AuthorEmail, c.AuthorSourceEmail} {
			if address = strings.ToLower(strings.TrimSpace(address)); address != "" {
				seen[address] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for address := range seen {
		out = append(out, address)
	}
	return out
}

// addressIn returns an address found in a serialised map: one of the
// history's own, or anything shaped like one.
func addressIn(serialised string, addresses []string) string {
	lower := strings.ToLower(serialised)
	for _, address := range addresses {
		if strings.Contains(lower, address) {
			return address
		}
	}
	return emailPattern().FindString(serialised)
}

// TestReplayMapCarriesNoRawAddress requires the serialised ownership map to
// contain no raw address (ADR-0051 clause 3, ADR-0033 clause 3). That follows
// from the identity index rather than from a filter: an owner is an index
// into a table of identity digests. The serialised form also reads back as the
// map it was written from.
func TestReplayMapCarriesNoRawAddress(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	scanned := 0
	for _, fixture := range smallCollectedFixtures(t, repo) {
		run := replayFixture(t, fixtureDir(t, repo, fixture), config.DefaultMaxFileBytes)
		if run.state.Ownership == nil {
			continue
		}
		var buf bytes.Buffer
		if err := run.state.Ownership.Encode(&buf); err != nil {
			fatal(t, 51, "%s: serialising the map: %v", fixture, err)
		}
		addresses := rawAddresses(run)
		if len(addresses) == 0 {
			fatal(t, 64, "%s records no address, so none could be found", fixture)
		}
		if found := addressIn(buf.String(), addresses); found != "" {
			report(t, 51, "%s: the serialised ownership map contains the raw address %q", fixture, found)
		}
		read, err := core.DecodeOwnership(bytes.NewReader(buf.Bytes()))
		if err != nil {
			fatal(t, 51, "%s: reading the serialised map back: %v", fixture, err)
		}
		if !reflect.DeepEqual(read, run.state.Ownership) {
			report(t, 51, "%s: the serialised map does not read back as the map it was written from", fixture)
		}
		scanned++
	}
	if scanned == 0 {
		fatal(t, 64, "no fixture produced an ownership map, so nothing was scanned")
	}
}

// TestReplayAddressCheckRejectsAnAddress is the failure demonstration: a map
// whose table held addresses rather than digests is found.
func TestReplayAddressCheckRejectsAnAddress(t *testing.T) {
	t.Parallel()
	leaky := &core.Ownership{Identities: []string{"ada@example.com"}}
	var buf bytes.Buffer
	if err := leaky.Encode(&buf); err != nil {
		fatal(t, 64, "serialising the demonstration map: %v", err)
	}
	if addressIn(buf.String(), []string{"ada@example.com"}) == "" {
		report(t, 64, "the address check accepted a map carrying ada@example.com")
	}
	if addressIn(buf.String(), nil) == "" {
		report(t, 64, "the address check accepted an address it had not been told of")
	}
}

// budgetsFile holds the budget values (ADR-0050 clause 5, ADR-0054 clause 2).
const budgetsFile = "internal/checks/budgets.txt"

// loadBudgets reads the budget file: "<budget> <fixture> <value>" per line.
func loadBudgets(t *testing.T, repo repository) map[string]float64 {
	t.Helper()
	budgets := map[string]float64{}
	scanner := bufio.NewScanner(strings.NewReader(repo.read(t, 54, budgetsFile)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			fatal(t, 54, "%s: %q is not \"<budget> <fixture> <value>\"", budgetsFile, line)
		}
		value, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			fatal(t, 54, "%s: %q has no numeric value", budgetsFile, line)
		}
		budgets[fields[0]+" "+fields[1]] = value
	}
	return budgets
}

// overBudget returns why a measurement is over its budget, or the empty
// string.
func overBudget(budgets map[string]float64, name, fixture string, measured float64) string {
	budget, ok := budgets[name+" "+fixture]
	switch {
	case !ok:
		return fmt.Sprintf("%s has no %s budget for %s", budgetsFile, name, fixture)
	case measured > budget:
		return fmt.Sprintf("%s on %s is %.2f, over its budget of %.2f in %s", name, fixture, measured, budget,
			budgetsFile)
	}
	return ""
}

// mapBytes is the ownership map's own size, counted from its structure: the
// map, its identity table, every file's record with its path and object
// name, and every line's owner and day, each shared line slice once. Paths and
// object names are counted though the collect records share them.
func mapBytes(o *core.Ownership) int {
	size := int(reflect.TypeOf(*o).Size()) + len(o.Commit)
	size += cap(o.Identities) * int(reflect.TypeOf("").Size())
	for _, id := range o.Identities {
		size += len(id)
	}
	size += cap(o.Files) * int(reflect.TypeOf(core.OwnedFile{}).Size())
	line := int(reflect.TypeOf(core.OwnedLine{}).Size())
	seen := map[*core.OwnedLine]bool{}
	for _, f := range o.Files {
		size += len(f.Path) + len(f.Blob) + len(f.Degraded)
		if cap(f.Lines) > 0 && !seen[&f.Lines[:1][0]] {
			seen[&f.Lines[:1][0]] = true
			size += cap(f.Lines) * line
		}
	}
	return size
}

// TestReplayMemoryBudget measures the ownership map's memory per tracked text
// line and the most commit states replay holds at once, records both, and
// fails either over its budget (ADR-0050 clause 3, ADR-0051 clause 4,
// ADR-0073, ADR-0054 clause 1). Per-line memory is measured on the designated
// large fixture; retained states on it and on the two fixtures with branches.
func TestReplayMemoryBudget(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	budgets := loadBudgets(t, repo)
	large := largeFixture(t, repo)
	for _, fixture := range []string{large, "merges", "merged-side-branch"} {
		run := replayFixture(t, fixtureDir(t, repo, fixture), config.DefaultMaxFileBytes)
		if run.state.Ownership == nil {
			fatal(t, 51, "replay produced no ownership map for %s: %s", fixture, run.state.Unavailable)
		}
		t.Logf("%s: %d commits replayed, at most %d states held", fixture, run.stats.Commits, run.stats.PeakStates)
		if why := overBudget(budgets, "replay-peak-states", fixture, float64(run.stats.PeakStates)); why != "" {
			report(t, 54, "%s", why)
		}
		if fixture != large {
			continue
		}
		lines := run.state.Ownership.Lines()
		if lines == 0 {
			fatal(t, 64, "%s holds no line, so no per-line memory can be measured", fixture)
		}
		size := mapBytes(run.state.Ownership)
		perLine := float64(size) / float64(lines)
		t.Logf("%s: the map holds %d lines in %d bytes, %.2f bytes per line", fixture, lines, size, perLine)
		if why := overBudget(budgets, "replay-bytes-per-line", fixture, perLine); why != "" {
			report(t, 54, "%s", why)
		}
	}
}

// TestReplayMemoryBudgetRejectsAnExcess is the failure demonstration: a
// measurement over its budget, and one with no budget, are both refused.
func TestReplayMemoryBudgetRejectsAnExcess(t *testing.T) {
	t.Parallel()
	budgets := map[string]float64{"replay-bytes-per-line large-history": 12}
	if overBudget(budgets, "replay-bytes-per-line", "large-history", 12.5) == "" {
		report(t, 64, "the budget check accepted a measurement over its budget")
	}
	if overBudget(budgets, "replay-peak-states", "large-history", 1) == "" {
		report(t, 64, "the budget check accepted a measurement with no budget")
	}
	if why := overBudget(budgets, "replay-bytes-per-line", "large-history", 12); why != "" {
		report(t, 54, "the budget check refused a measurement at its budget: %s", why)
	}
}

// TestReplayIgnoresMergeCounting requires replay to traverse merges whatever
// the merge-counting setting (ADR-0073 clause 6): the maps with merges counted
// and not counted are identical.
func TestReplayIgnoresMergeCounting(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := fixtureDir(t, repo, "merged-side-branch")
	var maps []string
	for _, counted := range []bool{false, true} {
		cfg := config.Default()
		cfg.CountMerges = counted
		history, err := newCollector().Collect(collect.Options{RepoPath: dir, Analysis: &cfg,
			Context: context.Background()})
		if err != nil {
			fatal(t, 73, "collecting with merges counted %v: %v", counted, err)
		}
		paths, err := filter.NewPathFilterFromAttributes(cfg, []byte(history.Attributes))
		if err != nil {
			fatal(t, 73, "building the path filter: %v", err)
		}
		state, _, err := replay.Run(replay.Options{Context: context.Background(), RepoPath: dir, History: history,
			PathFilter: paths, MaxFileBytes: config.DefaultMaxFileBytes})
		if err != nil || state.Ownership == nil {
			fatal(t, 73, "replaying with merges counted %v: %v", counted, err)
		}
		var buf bytes.Buffer
		if err := state.Ownership.Encode(&buf); err != nil {
			fatal(t, 73, "serialising the map: %v", err)
		}
		maps = append(maps, buf.String())
	}
	if maps[0] != maps[1] {
		report(t, 73, "the ownership map differs with merges counted and not counted")
	}
}

// TestReplayDegradesAFileOverTheCap requires a text file larger than the
// single-file-size limit to be marked degraded, with no lines, while the walk
// completes and every other file keeps its lines (ADR-0072 clause 5, ADR-0048
// clause 3). The noise fixture's data table is some 180 KB of text.
func TestReplayDegradesAFileOverTheCap(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := fixtureDir(t, repo, "noise")
	capped := replayFixture(t, dir, 4096)
	full := replayFixture(t, dir, config.DefaultMaxFileBytes)
	if capped.state.Ownership == nil {
		fatal(t, 48, "replay under a 4 KiB cap produced no map: %s", capped.state.Unavailable)
	}
	degraded := 0
	for i, f := range capped.state.Ownership.Files {
		whole := full.state.Ownership.Files[i]
		switch {
		case f.Path != whole.Path:
			fatal(t, 48, "the capped and uncapped maps hold different files")
		case f.Degraded != "":
			degraded++
			if f.Degraded != core.ReasonLimitReachedSize || f.Lines != nil || f.Binary {
				report(t, 48, "%s is over the cap and held as %+v; want it degraded with %s and no lines",
					f.Path, f, core.ReasonLimitReachedSize)
			}
		case !reflect.DeepEqual(f, whole):
			report(t, 48, "%s is not marked degraded, and its ownership moved with the cap", f.Path)
		}
	}
	if degraded == 0 {
		fatal(t, 64, "no file of the noise fixture is over a 4 KiB cap, so nothing was degraded")
	}
	if capped.state.TextFileCount != full.state.TextFileCount {
		report(t, 48, "the cap changed the tracked text file count from %d to %d; a text file over it is still "+
			"a text file", full.state.TextFileCount, capped.state.TextFileCount)
	}
}
