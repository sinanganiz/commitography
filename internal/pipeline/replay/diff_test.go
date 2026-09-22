package replay

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// alignText aligns two texts given one line per element, each written with
// its terminator.
func alignText(older, newer []string) []int32 {
	return align(asLines(older), asLines(newer))
}

func asLines(text []string) [][]byte {
	out := make([][]byte, len(text))
	for i, line := range text {
		out[i] = []byte(line + "\n")
	}
	return out
}

// checkCommon fails unless matched pairs each line of newer with an equal line
// of older, in increasing order of both: a common subsequence. It returns how
// many lines it pairs.
func checkCommon(t *testing.T, older, newer [][]byte, matched []int32) int {
	t.Helper()
	if len(matched) != len(newer) {
		t.Fatalf("the alignment has %d entries for %d lines", len(matched), len(newer))
	}
	last, count := -1, 0
	for j, i := range matched {
		if i < 0 {
			continue
		}
		if int(i) <= last || int(i) >= len(older) {
			t.Fatalf("line %d is aligned with line %d, out of order or out of range", j, i)
		}
		if string(older[i]) != string(newer[j]) {
			t.Fatalf("line %d %q is aligned with line %d %q, which differs", j, newer[j], i, older[i])
		}
		last, count = int(i), count+1
	}
	return count
}

func TestReplayLinesKeepTheirTerminators(t *testing.T) {
	t.Parallel()
	for content, want := range map[string][]string{
		"":             nil,
		"a":            {"a"},
		"a\n":          {"a\n"},
		"a\nb":         {"a\n", "b"},
		"a\r\nb\n\n":   {"a\r\n", "b\n", "\n"},
		"\n\n":         {"\n", "\n"},
		"x\x00y\nz\n":  {"x\x00y\n", "z\n"},
		"no newline\n": {"no newline\n"},
	} {
		var got []string
		for _, line := range splitLines([]byte(content)) {
			got = append(got, string(line))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("splitLines(%q) = %q, want %q", content, got, want)
		}
	}
}

// The alignments a reader can check by hand.
func TestReplayAlignsHandWrittenCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		older, newer []string
		want         []int32
	}{
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, []int32{0, 1, 2}},
		{"a line inserted", []string{"a", "c"}, []string{"a", "b", "c"}, []int32{0, -1, 1}},
		{"a line removed", []string{"a", "b", "c"}, []string{"a", "c"}, []int32{0, 2}},
		{"a line replaced", []string{"a", "b", "c"}, []string{"a", "B", "c"}, []int32{0, -1, 2}},
		{"everything new", nil, []string{"a", "b"}, []int32{-1, -1}},
		{"everything removed", []string{"a", "b"}, nil, []int32{}},
		{"lines appended", []string{"a"}, []string{"a", "b", "c"}, []int32{0, -1, -1}},
		{"lines prepended", []string{"c"}, []string{"a", "b", "c"}, []int32{-1, -1, 0}},
		{"repeated lines", []string{"x", "y", "x", "y"}, []string{"x", "y", "x", "y", "x", "y"},
			[]int32{0, 1, 2, 3, -1, -1}},
		// Where two alignments are as long, an old line is passed over first,
		// so the line that stays in place is the later one.
		{"two lines swapped", []string{"a", "b", "c", "d"}, []string{"a", "c", "b", "d"}, []int32{0, 2, -1, 3}},
		{"a block moved", []string{"1", "2", "3", "4", "5", "6"},
			[]string{"4", "5", "6", "1", "2", "3"}, []int32{3, 4, 5, -1, -1, -1}},
	}
	for _, tc := range cases {
		got := alignText(tc.older, tc.newer)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: aligned as %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A last line that gains its terminator is a changed line, as it is to git.
func TestReplayAGainedTerminatorChangesTheLine(t *testing.T) {
	t.Parallel()
	got := align(splitLines([]byte("a\nb")), splitLines([]byte("a\nb\n")))
	if want := []int32{0, -1}; !reflect.DeepEqual(got, want) {
		t.Errorf("aligned as %v, want %v", got, want)
	}
}

// longestCommon is the length of a longest common subsequence, by the
// textbook recursion, for checking small cases.
func longestCommon(a, b [][]byte) int {
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if string(a[i]) == string(b[j]) {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table[0][0]
}

// Within the exact bound, the alignment keeps as many lines as any alignment
// could.
func TestReplayAlignmentIsLongestWithinTheExactBound(t *testing.T) {
	t.Parallel()
	random := rand.New(rand.NewSource(13))
	text := func(n int) [][]byte {
		out := make([][]byte, n)
		for i := range out {
			out[i] = []byte(fmt.Sprintf("%c\n", 'a'+random.Intn(4)))
		}
		return out
	}
	for trial := 0; trial < 500; trial++ {
		older, newer := text(random.Intn(14)), text(random.Intn(14))
		matched := align(older, newer)
		if got, want := checkCommon(t, older, newer, matched), longestCommon(older, newer); got != want {
			t.Fatalf("trial %d: aligned %d lines of %q and %q, and %d is possible", trial, got, older, newer, want)
		}
	}
}

// A gap beyond the exact bound is anchored on the lines unique to both sides:
// a long file with scattered edits keeps every unedited line.
func TestReplayALargeGapIsAnchored(t *testing.T) {
	t.Parallel()
	const n = 5000
	older := make([]string, n)
	newer := make([]string, n)
	edited := 0
	for i := range older {
		older[i] = fmt.Sprintf("line %d", i)
		newer[i] = older[i]
		if i%97 == 3 {
			newer[i] = fmt.Sprintf("edited %d", i)
			edited++
		}
	}
	a, b := asLines(older), asLines(newer)
	matched := align(a, b)
	if got := checkCommon(t, a, b, matched); got != n-edited {
		t.Errorf("aligned %d lines, want the %d that were not edited", got, n-edited)
	}
}

// A gap beyond the exact bound with no line unique to both sides has nothing
// to anchor on and is left unaligned, in bounded work, rather than aligned at a
// cost a crafted file could make arbitrary.
func TestReplayAGapWithoutAnchorsIsLeftUnaligned(t *testing.T) {
	t.Parallel()
	older := strings.Split(strings.Repeat("x,y,", 3000), ",")
	newer := strings.Split(strings.Repeat("y,x,", 3000), ",")
	a, b := asLines(older), asLines(newer)
	matched := align(a, b)
	checkCommon(t, a, b, matched)
}
