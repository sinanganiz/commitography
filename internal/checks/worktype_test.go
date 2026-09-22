package checks

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/pipeline/replay"
)

// The work-type classification inputs replay records (WP-0014, ADR-0074):
// the purpose-built fixture's events, the window's absence from replay state,
// and the absence of blame from the path that records them.

// worktypeFixture is the fixture whose every line-level change is known by
// construction. testdata/build-fixtures.sh writes each commit's expected
// events beside it, and worktypeFixtureEvents holds the same list.
const worktypeFixture = "worktype-events"

// worktypeFixtureEvents is the events of each commit of the fixture, by
// subject, as "<kind> <editor> <- <previous owner> <age>" or
// "addition <editor>". A commit produces them only when it is counted, which
// for the merge means only when merges are counted.
func worktypeFixtureEvents() map[string][]string {
	repeated := func(n int, event string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = event
		}
		return out
	}
	return map[string][]string{
		"feat: write the first draft": repeated(8, "addition Ada Lovelace"),
		"fix: reword bravo":           {"replacement Ada Lovelace <- Ada Lovelace 2"},
		"fix: reword charlie":         {"replacement Grace Hopper <- Ada Lovelace 4"},
		"fix: reword delta":           {"replacement Ada L. <- Ada Lovelace 6"},
		"fix: reword delta again":     {"replacement Ada Lovelace <- Ada L. 2"},
		"feat: add india":             {"addition Alan Turing"},
		"refactor: drop golf":         {"deletion Grace Hopper <- Ada Lovelace 12"},
		"fix: reword echo":            {"replacement Alan Turing <- Ada Lovelace 29"},
		"fix: reword foxtrot":         {"replacement Grace Hopper <- Ada Lovelace 30"},
		"fix: reword alpha":           {"replacement Alan Turing <- Ada Lovelace 45"},
		"refactor: split two lines into three": {"replacement Ada L. <- Ada Lovelace 45",
			"replacement Ada L. <- Grace Hopper 43", "addition Ada L."},
		"refactor: fold three lines into one": {"replacement Grace Hopper <- Ada Lovelace 42",
			"deletion Grace Hopper <- Alan Turing 21", "deletion Grace Hopper <- Grace Hopper 20"},
		"docs: add notes": repeated(6, "addition Ada Lovelace"),
		"docs: reword note two and drop note five": {"replacement Grace Hopper <- Ada Lovelace 2",
			"deletion Grace Hopper <- Ada Lovelace 2"},
		"docs: reword note two": {"replacement Alan Turing <- Ada Lovelace 3"},
		// The resolution replaces the first parent's line, and note four,
		// which both parents hold, is deleted. Note five, which the side
		// branch removed, is not the merge's deletion.
		"Merge branch 'side'": {"replacement Ada Lovelace <- Alan Turing 2",
			"deletion Ada Lovelace <- Ada Lovelace 5"},
	}
}

// replayTo replays a collected history to one of its commits, as the pipeline
// replays it to the analysed commit.
func replayTo(t *testing.T, dir string, cfg config.Analysis, history *model.History, commit string) *core.ReplayState {
	t.Helper()
	paths, err := filter.NewPathFilterFromAttributes(cfg, []byte(history.Attributes))
	if err != nil {
		fatal(t, 74, "building the path filter for %s: %v", filepath.Base(dir), err)
	}
	to := *history
	to.Repository.HeadCommit = commit
	state, _, err := replay.Run(replay.Options{Context: context.Background(), RepoPath: dir, History: &to,
		PathFilter: paths, MaxFileBytes: config.DefaultMaxFileBytes})
	if err != nil {
		fatal(t, 74, "replaying %s to %.12s: %v", filepath.Base(dir), commit, err)
	}
	if state.Worktype == nil {
		fatal(t, 74, "replaying %s to %.12s recorded no classification inputs: %s", filepath.Base(dir), commit,
			state.Unavailable)
	}
	return state
}

// collectWith collects a fixture under the given analysis plane.
func collectWith(t *testing.T, dir string, cfg config.Analysis) *model.History {
	t.Helper()
	history, err := newCollector().Collect(collect.Options{RepoPath: dir, Analysis: &cfg,
		Context: context.Background()})
	if err != nil {
		fatal(t, 74, "collecting %s: %v", filepath.Base(dir), err)
	}
	return history
}

// renderEvents renders recorded inputs as one line per event, sorted, naming
// identities as names gives them. A digest names does not know fails.
func renderEvents(t *testing.T, in *core.WorktypeInputs, names map[string]string) []string {
	t.Helper()
	name := func(id string) string {
		n, ok := names[id]
		if !ok {
			fatal(t, 33, "the classification inputs name %q, which is no identity digest of the history", id)
		}
		return n
	}
	var out []string
	for _, p := range in.Pairs {
		for kind, bars := range map[string][]core.AgeCount{"replacement": p.Replaced, "deletion": p.Deleted} {
			for _, bar := range bars {
				for n := 0; n < bar.Lines; n++ {
					out = append(out, fmt.Sprintf("%s %s <- %s %d", kind, name(p.Editor), name(p.Owner), bar.Days))
				}
			}
		}
	}
	for _, a := range in.Additions {
		for n := 0; n < a.Lines; n++ {
			out = append(out, "addition "+name(a.Editor))
		}
	}
	sort.Strings(out)
	return out
}

// multisetDifference returns the lines of a not matched by a line of b, each
// line of b matching one line of a.
func multisetDifference(a, b []string) []string {
	left := map[string]int{}
	for _, line := range b {
		left[line]++
	}
	var out []string
	for _, line := range a {
		if left[line] > 0 {
			left[line]--
			continue
		}
		out = append(out, line)
	}
	return out
}

// ancestry returns a commit and every ancestor of it, by hash.
func ancestry(byHash map[string]model.Commit, hash string) map[string]bool {
	seen := map[string]bool{hash: true}
	stack := []string{hash}
	for len(stack) > 0 {
		c := byHash[stack[len(stack)-1]]
		stack = stack[:len(stack)-1]
		for _, p := range c.Parents {
			if !seen[p] {
				seen[p] = true
				stack = append(stack, p)
			}
		}
	}
	return seen
}

// TestWorkTypeFixtureEvents replays the purpose-built fixture to each of its
// commits and requires the classification inputs recorded there to be exactly
// the events of the counted commits in that commit's ancestry, as the fixture
// was built to produce (ADR-0074 clauses 1 to 7). It does so with merges not
// counted, the default, and counted. Beside the whole, it names the cases the
// package's definition of done lists: a rewrite of one's own recent line is
// one replacement and no addition, the lines around the default window are
// recorded at exactly their ages, and the merge deletes the line every parent
// holds and not the one a single side removed.
func TestWorkTypeFixtureEvents(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	dir := fixtureDir(t, repo, worktypeFixture)
	if window := config.Default().RecencyWindowDays; window != 30 {
		fatal(t, 74, "the %s fixture places lines 29 and 30 days old around the default recency window of 30 days, "+
			"and the default is now %d; move them", worktypeFixture, window)
	}
	expected := worktypeFixtureEvents()

	for _, merges := range []bool{false, true} {
		cfg := config.Default()
		cfg.CountMerges = merges
		history := collectWith(t, dir, cfg)
		byHash := map[string]model.Commit{}
		names := map[string]string{}
		subjects := map[string]bool{}
		for _, c := range history.Commits {
			byHash[c.Hash] = c
			names[c.IdentityID] = c.AuthorName
			subjects[c.Subject] = true
			if _, ok := expected[c.Subject]; !ok {
				fatal(t, 19, "commit %q of the %s fixture has no expected events", c.Subject, worktypeFixture)
			}
		}
		for subject := range expected {
			if !subjects[subject] {
				fatal(t, 19, "the expected events name commit %q, which the %s fixture does not hold", subject,
					worktypeFixture)
			}
		}

		recorded := map[string][]string{}
		for i := len(history.Commits) - 1; i >= 0; i-- { // oldest first
			c := history.Commits[i]
			got := renderEvents(t, replayTo(t, dir, cfg, history, c.Hash).Worktype, names)
			recorded[c.Hash] = got
			var want []string
			for hash := range ancestry(byHash, c.Hash) {
				if a := byHash[hash]; filter.CountsForLines(a) {
					want = append(want, expected[a.Subject]...)
				}
			}
			sort.Strings(want)
			missing, unexpected := multisetDifference(want, got), multisetDifference(got, want)
			if len(missing) > 0 || len(unexpected) > 0 {
				fatal(t, 74, "%s, merges counted %v, replayed to %q: the inputs lack\n  %s\nand hold beyond what "+
					"the fixture was built to produce\n  %s", worktypeFixture, merges, c.Subject,
					strings.Join(missing, "\n  "), strings.Join(unexpected, "\n  "))
			}
		}

		// own is the events a commit added to its first parent's inputs.
		own := func(subject string) []string {
			for _, c := range history.Commits {
				if c.Subject == subject {
					return multisetDifference(recorded[c.Hash], recorded[c.Parents[0]])
				}
			}
			return nil
		}
		if got := own("fix: reword bravo"); strings.Join(got, "; ") != "replacement Ada Lovelace <- Ada Lovelace 2" {
			report(t, 74, "rewriting one's own recent line recorded %q; it is one replacement and no addition "+
				"(clause 4)", got)
		}
		for subject, want := range map[string]string{
			"fix: reword foxtrot": "replacement Grace Hopper <- Ada Lovelace 30",
			"fix: reword echo":    "replacement Alan Turing <- Ada Lovelace 29",
		} {
			if got := own(subject); strings.Join(got, "; ") != want {
				report(t, 74, "%q recorded %q; the line it rewrote is recorded at exactly its age, %s", subject, got,
					want)
			}
		}
		// The merge's own events are what it adds to its first parent's inputs
		// beyond the side branch's commit.
		merge := own("Merge branch 'side'")
		merge = multisetDifference(merge, expected["docs: reword note two and drop note five"])
		deletions := 0
		for _, e := range merge {
			if strings.HasPrefix(e, "deletion Ada Lovelace <- ") {
				deletions++
			}
		}
		switch {
		case merges && (deletions != 1 || !contains(merge, "deletion Ada Lovelace <- Ada Lovelace 5")):
			report(t, 74, "the counted merge recorded %q; it deletes note four, which both parents hold, and not "+
				"note five, which one side removed", merge)
		case !merges && len(merge) != 0:
			report(t, 73, "a merge that is not counted recorded %q; it records nothing (ADR-0073 clause 6)", merge)
		}
	}
}

func contains(list []string, item string) bool {
	for _, l := range list {
		if l == item {
			return true
		}
	}
	return false
}

// encodeState is a replay state's serialised form, whose bytes are compared.
func encodeState(t *testing.T, state *core.ReplayState) string {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		fatal(t, 74, "serialising a replay state: %v", err)
	}
	return string(data)
}

// windowDependence returns where two serialised replay states, produced under
// two recency windows, differ, or the empty string where they are identical.
func windowDependence(a, b string) string {
	if a == b {
		return ""
	}
	at := 0
	for at < len(a) && at < len(b) && a[at] == b[at] {
		at++
	}
	excerpt := func(s string) string {
		return s[max(0, at-40):min(len(s), at+40)]
	}
	return fmt.Sprintf("they part at byte %d: %q against %q", at, excerpt(a), excerpt(b))
}

// TestWorkTypeReplayStateIgnoresTheWindow replays every small fixture under
// the default recency window and under a window of 7 days, and requires the
// two replay states to be byte-identical: replay records ages and applies no
// window, which the worktype family applies when it is aggregated (ADR-0074
// clauses 8 and 9).
func TestWorkTypeReplayStateIgnoresTheWindow(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	events := 0
	for _, fixture := range smallCollectedFixtures(t, repo) {
		dir := fixtureDir(t, repo, fixture)
		var encoded []string
		for _, window := range []int{config.Default().RecencyWindowDays, 7} {
			cfg := config.Default()
			cfg.RecencyWindowDays = window
			history := collectWith(t, dir, cfg)
			state := replayTo(t, dir, cfg, history, history.Repository.HeadCommit)
			events += len(state.Worktype.Pairs) + len(state.Worktype.Additions)
			encoded = append(encoded, encodeState(t, state))
		}
		if why := windowDependence(encoded[0], encoded[1]); why != "" {
			report(t, 74, "%s: the replay state under a recency window of %d days differs from the one under 7 "+
				"days, and replay applies no window (clause 9): %s", fixture, config.Default().RecencyWindowDays,
				why)
		}
	}
	if events == 0 {
		fatal(t, 64, "no fixture recorded any classification input, so the window's absence was not shown")
	}
}

// TestWorkTypeWindowCheckRejectsAWindowedState is the failure demonstration
// ADR-0064 clause 6 requires: a replay state from which a window had dropped
// the ages at or past it, as a replay applying the window would, differs
// between two windows, and the check says where.
func TestWorkTypeWindowCheckRejectsAWindowedState(t *testing.T) {
	t.Parallel()
	windowed := func(window int32) *core.ReplayState {
		pair := core.WorktypePair{Editor: "0123456789abcdef", Owner: "fedcba9876543210"}
		for _, days := range []int32{3, 20, 45} {
			if days < window {
				pair.Replaced = append(pair.Replaced, core.AgeCount{Days: days, Lines: 1})
			}
		}
		return &core.ReplayState{Worktype: &core.WorktypeInputs{Pairs: []core.WorktypePair{pair}}}
	}
	if windowDependence(encodeState(t, windowed(30)), encodeState(t, windowed(7))) == "" {
		report(t, 64, "the window check accepted two replay states a window had made differ")
	}
	if why := windowDependence(encodeState(t, windowed(30)), encodeState(t, windowed(30))); why != "" {
		report(t, 74, "the window check refused two identical replay states: %s", why)
	}
}

// replayStage is the stage whose tracked files may not name blame.
const replayStage = "internal/pipeline/replay"

// blameIn returns every line of a file that names blame, in any case.
func blameIn(name, content string) []string {
	var out []string
	for i, line := range lines(content) {
		if strings.Contains(strings.ToLower(line), "blame") {
			out = append(out, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
		}
	}
	return out
}

// TestWorkTypeInvokesNoBlame enforces ADR-0020 clause 4 on the path that
// records the classification inputs: no file tracked under the replay stage,
// source or test, names blame. The git package runs what its arguments name,
// so an invocation of blame has to name it; the map, not blame, is the source.
func TestWorkTypeInvokesNoBlame(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	scanned := 0
	for _, file := range repo.tracked {
		if !strings.HasPrefix(file, replayStage+"/") {
			continue
		}
		scanned++
		for _, finding := range blameIn(file, repo.read(t, 20, file)) {
			report(t, 20, "work-type classification consults the ownership map, never blame, and %s", finding)
		}
	}
	if scanned == 0 {
		fatal(t, 64, "no file is tracked under %s, so nothing was scanned", replayStage)
	}
}

// TestWorkTypeBlameCheckRejectsABlameInvocation is the failure demonstration.
func TestWorkTypeBlameCheckRejectsABlameInvocation(t *testing.T) {
	t.Parallel()
	invokes := "package replay\nfunc owners(dir string) { git.Output(ctx, git.At(dir, \"blame\", \"--incremental\")) }\n"
	if len(blameIn("walk.go", invokes)) == 0 {
		report(t, 64, "the blame check accepted a source that invokes blame")
	}
	if found := blameIn("walk.go", "package replay\nfunc owners() {}\n"); len(found) != 0 {
		report(t, 20, "the blame check refused a source that does not name blame: %v", found)
	}
}
