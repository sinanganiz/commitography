package replay

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
)

// worktypeOf replays a history to head, with the given single-file-size limit
// (0 for none), and returns the classification inputs it recorded.
func worktypeOf(t *testing.T, h *history, head string, limit int) *core.WorktypeInputs {
	t.Helper()
	paths, err := filter.NewPathFilterFromAttributes(config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := filter.NewBinaryRule(nil)
	if err != nil {
		t.Fatal(err)
	}
	w := &walker{
		ctx:      context.Background(),
		opts:     Options{History: h.records(head), PathFilter: paths},
		objects:  &fakeObjects{contents: h.contents, limit: limit},
		binary:   rule,
		cache:    newContentCache(cacheBytes),
		identity: map[string]uint32{},
	}
	ownership, unavailable, err := w.walk(head)
	if err != nil || ownership == nil {
		t.Fatalf("replaying to %s: map %v, reason %q, error %v", head, ownership, unavailable, err)
	}
	return w.worktypeInputs()
}

// eventsOf renders recorded inputs as one line per event, sorted:
// "replacement grace<-ada 4" for a replacement of a line of ada's four days
// old, "deletion ..." likewise, and "addition ada".
func eventsOf(in *core.WorktypeInputs) []string {
	var out []string
	for _, p := range in.Pairs {
		for kind, bars := range map[string][]core.AgeCount{"replacement": p.Replaced, "deletion": p.Deleted} {
			for _, bar := range bars {
				for n := 0; n < bar.Lines; n++ {
					out = append(out, fmt.Sprintf("%s %s<-%s %d", kind, p.Editor, p.Owner, bar.Days))
				}
			}
		}
	}
	for _, a := range in.Additions {
		for n := 0; n < a.Lines; n++ {
			out = append(out, "addition "+a.Editor)
		}
	}
	sort.Strings(out)
	return out
}

// times repeats an event.
func times(n int, event string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = event
	}
	return out
}

// expectEvents fails unless the recorded events are exactly want, in any
// order.
func expectEvents(t *testing.T, what string, got *core.WorktypeInputs, want ...[]string) {
	t.Helper()
	var all []string
	for _, w := range want {
		all = append(all, w...)
	}
	sort.Strings(all)
	if events := eventsOf(got); !reflect.DeepEqual(events, all) {
		t.Errorf("%s: recorded\n  %s\nwant\n  %s", what, strings.Join(events, "\n  "), strings.Join(all, "\n  "))
	}
}

// Rewriting one's own line is one replacement, carrying the line's age, and no
// addition (ADR-0074 clause 4).
func TestWorkTypeRewritingOnesOwnLineIsOneReplacement(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f", content: text("a", "b", "c")})
	h.commit("c2", "ada", 4, "c1", edit{path: "f", content: text("a", "B", "c")})
	expectEvents(t, "a rewrite of one's own line", worktypeOf(t, h, "c2", 0),
		times(3, "addition ada"), []string{"replacement ada<-ada 3"})
}

// Within each changed block, the first removed lines pair with the first added
// lines by position (ADR-0074 clause 2). A block of two removed and three
// added lines is two replacements and an addition; one of three removed and
// one added is a replacement and two deletions. The three removed lines of the
// second block have three owners and ages, so pairing the added line with any
// removed line but the first records other events.
func TestWorkTypePairsEachBlockByPosition(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f", content: text("1", "2", "3", "4", "5", "6", "7", "8", "9", "10")})
	h.commit("c2", "alan", 2, "c1", edit{path: "f", content: text("1", "2", "3'", "4", "5'", "6", "7", "8", "9", "10")})
	h.commit("c3", "bob", 3, "c2", edit{path: "f", content: text("1", "2", "3'", "4", "5'", "6'", "7", "8", "9", "10")})
	h.commit("c4", "grace", 5, "c3", edit{path: "f", content: text("1", "a", "b", "c", "4", "d", "8", "e", "9")})
	expectEvents(t, "four blocks in one commit", worktypeOf(t, h, "c4", 0),
		times(10, "addition ada"),
		times(2, "replacement alan<-ada 1"),
		[]string{"replacement bob<-ada 2"},
		// 2 and 3' become a, b and c.
		[]string{"replacement grace<-ada 4", "replacement grace<-alan 3", "addition grace"},
		// 5', 6' and 7 become d: 5' is replaced, 6' and 7 deleted.
		[]string{"replacement grace<-alan 3", "deletion grace<-bob 2", "deletion grace<-ada 4"},
		// e is added between 8 and 9, and 10 is removed after 9.
		[]string{"addition grace", "deletion grace<-ada 4"})
}

// A file a commit adds is all additions and one it deletes all deletions; a
// file renamed with an edit counts the lines the edit changed, and one renamed
// unchanged counts none.
func TestWorkTypeFilesAddedDeletedAndRenamed(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f", content: text("a", "b")},
		edit{path: "g", content: text("x", "y", "z")})
	h.commit("c2", "grace", 3, "c1", edit{path: "g", remove: true},
		edit{path: "h", from: "f", content: text("a", "B")}, edit{path: "n", content: text("n1")})
	h.commit("c3", "alan", 4, "c2", edit{path: "k", from: "h", content: text("a", "B")})
	expectEvents(t, "added, deleted and renamed files", worktypeOf(t, h, "c3", 0),
		times(5, "addition ada"),
		times(3, "deletion grace<-ada 2"),
		[]string{"replacement grace<-ada 2", "addition grace"})
}

// Only analysed, non-bulk commits produce events (ADR-0073 clause 6,
// docs/metrics.md section 8). The others are replayed for ownership all the
// same, so a later event carries the owner they gave a line.
func TestWorkTypeCountsOnlyAnalysedNonBulkCommits(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f", content: text("a", "b", "c")})
	h.commit("c2", "bot", 2, "c1", edit{path: "f", content: text("a", "B", "c")})
	h.commit("c3", "ada", 3, "c2", edit{path: "f", content: text("a", "B", "C")})
	h.commit("c4", "grace", 6, "c3", edit{path: "f", content: text("a2", "B2", "C2")})
	h.commits[1].Excluded, h.commits[1].AuthorExcluded = true, true
	h.commits[2].IsBulk = true
	expectEvents(t, "an excluded and a bulk commit among analysed ones", worktypeOf(t, h, "c4", 0),
		times(3, "addition ada"),
		[]string{"replacement grace<-ada 5", "replacement grace<-bot 4", "replacement grace<-ada 3"})
}

// An age is a difference of local dates, each in the offset its commit
// recorded, at day resolution, and it is negative where the editing commit is
// dated before the line it changes (ADR-0074 clause 5).
func TestWorkTypeAgesAreDaysBetweenLocalDates(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 0, "", edit{path: "f", content: text("x")})
	h.commit("c2", "grace", 0, "c1", edit{path: "f", content: text("y")})
	h.commit("c3", "alan", 0, "c2", edit{path: "f", content: text("z")})
	// 23:30 on the 10th at -05:00 is the 11th in UTC; 01:00 on the 11th at
	// +02:00 is the 10th in UTC. By local date, c2 is a day after c1.
	h.commits[0].LocalTime = time.Date(2025, 1, 10, 23, 30, 0, 0, time.FixedZone("", -5*3600))
	h.commits[1].LocalTime = time.Date(2025, 1, 11, 1, 0, 0, 0, time.FixedZone("", 2*3600))
	h.commits[2].LocalTime = time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC)
	expectEvents(t, "ages across offsets", worktypeOf(t, h, "c3", 0),
		[]string{"addition ada", "replacement grace<-ada 1", "replacement alan<-grace -6"})
}

// The merge rule of ADR-0074 clause 6 and the package's settled details: a
// merge's blocks are taken against its first parent; an addition or
// replacement is recorded only for a line of the merge's own, matching no
// parent, and a deletion only for a line every parent holds that the merge
// removes. A line one side removed, a line taken from the other parent and a
// file taken whole from it are the parents' own commits' events. A merge that
// is not counted records nothing.
func TestWorkTypeMergeRecordsOnlyItsOwnChanges(t *testing.T) {
	t.Parallel()
	build := func(counted bool) *history {
		h := newHistory()
		h.commit("root", "ada", 1, "", edit{path: "f", content: text("one", "two", "three", "four", "five", "six")},
			edit{path: "g", content: text("x", "k")}, edit{path: "h", content: text("h1")})
		h.commit("side", "grace", 3, "root", edit{path: "f", content: text("one", "two-g", "three", "four", "six")},
			edit{path: "g", content: text("x", "y", "k")}, edit{path: "h", content: text("h1", "h2")})
		h.commit("main", "alan", 4, "root", edit{path: "f", content: text("one", "two-a", "three", "four", "five",
			"six")})
		h.merge("merge", "ada", 6, []string{"main", "side"}, map[string]string{
			// two-resolved matches neither parent, four is in both and goes,
			// five went on the side only, and evil is new.
			"f": text("one", "two-resolved", "three", "six", "evil"),
			// x is in both parents and goes; y comes from the side, paired by
			// position with x, and is the side's.
			"g": text("y", "k"),
			// Taken whole from the side.
			"h": text("h1", "h2"),
		})
		if !counted {
			last := &h.commits[len(h.commits)-1]
			last.Excluded, last.MergeExcluded = true, true
		}
		return h
	}
	parents := [][]string{
		times(9, "addition ada"),
		{"replacement grace<-ada 2", "deletion grace<-ada 2", "addition grace", "addition grace"},
		{"replacement alan<-ada 3"},
	}
	expectEvents(t, "a merge that is not counted", worktypeOf(t, build(false), "merge", 0), parents...)
	expectEvents(t, "a counted merge", worktypeOf(t, build(true), "merge", 0), append(parents,
		// The resolution replaces the first parent's line, four goes from
		// both parents, evil is added, and x goes from both parents.
		[]string{"replacement ada<-alan 2", "deletion ada<-ada 5", "addition ada", "deletion ada<-ada 5"})...)
}

// A change to or from a version whose lines replay cannot tell apart records
// no events: one over the single-file-size limit, and one derived from it,
// whose lines replay gives to its commit's author whole. The inputs are then
// marked limit_reached_size, and every other file is counted as before
// (ADR-0074 clause 7).
func TestWorkTypeUnreadableVersionsRecordNoEvents(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("big line\n", 20)
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "big.txt", content: big}, edit{path: "small.txt", content: text("s1")})
	h.commit("c2", "grace", 2, "c1", edit{path: "big.txt", content: text("small again")})
	h.commit("c3", "alan", 3, "c2", edit{path: "big.txt", content: text("small again", "more")},
		edit{path: "small.txt", content: text("s1", "s2")})
	for head, want := range map[string][]string{
		"c1": {"addition ada"},
		"c2": {"addition ada"},
		"c3": {"addition ada", "addition alan"},
	} {
		inputs := worktypeOf(t, h, head, 64)
		expectEvents(t, "to "+head+" with a file over the limit", inputs, want)
		if inputs.Degraded != core.ReasonLimitReachedSize {
			t.Errorf("to %s the inputs are marked %q, want %s", head, inputs.Degraded, core.ReasonLimitReachedSize)
		}
	}
	if inputs := worktypeOf(t, h, "c3", 0); inputs.Degraded != "" {
		t.Errorf("with no limit the inputs are marked %q, want no mark", inputs.Degraded)
	}
}

// The mark says counted changes were left out, so it is not set where only
// uncounted commits changed such a file, or where a counted one only moved it.
func TestWorkTypeUnreadableVersionsMarkOnlyWhatWasLeftOut(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("big line\n", 20)
	h := newHistory()
	h.commit("c1", "bot", 1, "", edit{path: "big.txt", content: big}, edit{path: "small.txt", content: text("s")})
	h.commit("c2", "bot", 2, "c1", edit{path: "big.txt", content: text("smaller")})
	h.commit("c3", "ada", 3, "c2", edit{path: "moved.txt", from: "big.txt", content: text("smaller")},
		edit{path: "small.txt", content: text("s", "t")})
	for i := 0; i < 2; i++ {
		h.commits[i].Excluded, h.commits[i].AuthorExcluded = true, true
	}
	inputs := worktypeOf(t, h, "c3", 64)
	expectEvents(t, "a file over the limit that no counted commit changed", inputs, []string{"addition ada"})
	if inputs.Degraded != "" {
		t.Errorf("the inputs are marked %q, and no counted change was left out", inputs.Degraded)
	}
}

// A binary version has no lines, so a change to, from or between binary
// versions records nothing, and marks nothing.
func TestWorkTypeBinaryVersionsRecordNoEvents(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "doc.txt", content: text("a", "b")},
		edit{path: "logo.png", content: "PNG\x00\x01"})
	h.commit("c2", "grace", 2, "c1", edit{path: "doc.txt", content: "now\x00binary"},
		edit{path: "logo.png", content: "PNG\x00\x02"})
	h.commit("c3", "alan", 3, "c2", edit{path: "doc.txt", content: text("text again")})
	inputs := worktypeOf(t, h, "c3", 0)
	expectEvents(t, "changes to and from binary versions", inputs, times(2, "addition ada"))
	if inputs.Degraded != "" {
		t.Errorf("the inputs are marked %q; a binary file is not a limit reached", inputs.Degraded)
	}
}
