package replay

import "bytes"

// The line difference replay derives ownership through (ADR-0073 clause 7):
// one fixed algorithm, computed here, independent of git's diff configuration
// and of any diff git would produce. Changing it changes which lines keep their
// owner, so it is a change of meaning for every family that reads replay state.
//
// A line is its bytes up to and including its terminator; a last line without
// one is a line too, and differs from the same text with one, as git counts
// lines. Two versions are aligned as follows, and in no other way:
//
//  1. The lines both versions begin with, and then the lines both end with,
//     are the same lines.
//  2. What remains between them is a gap. A gap of at most lcsCells line pairs
//     is aligned by a longest common subsequence, computed exactly; where
//     several are longest, a line of the old version is passed over before a
//     line of the new one.
//  3. A larger gap is anchored on the lines that occur exactly once in each
//     version's part of it, keeping the longest run of them that appears in
//     the same order in both. The anchors are the same lines, and the stretches
//     between them are gaps in their own right, from step 1.
//  4. A gap with no anchor, or one reached once the work budget is spent, has
//     no line in common. The budget grows with the lengths of the two versions,
//     so a file crafted to make alignment slow costs a bounded amount of work
//     and loses only the attribution of its unaligned lines.
//
// A line of the new version that is aligned with a line of the old one keeps
// that line's owner. Every other line of the new version is written by the
// commit that produced it.

const (
	// lcsCells bounds the gap aligned exactly: 2^20 line pairs, a table of 4
	// MiB, which a thousand lines against a thousand fills.
	lcsCells = 1 << 20

	// workPerLine and workBase set the budget of one alignment: the work of
	// scanning each line workPerLine times over, plus four exact gaps of the
	// largest size.
	workPerLine = 16
	workBase    = 4 * lcsCells
)

// splitLines returns content's lines, each with its terminator.
func splitLines(content []byte) [][]byte {
	if len(content) == 0 {
		return nil
	}
	lines := make([][]byte, 0, bytes.Count(content, []byte{'\n'})+1)
	for len(content) > 0 {
		end := bytes.IndexByte(content, '\n')
		if end < 0 {
			lines = append(lines, content)
			break
		}
		lines = append(lines, content[:end+1])
		content = content[end+1:]
	}
	return lines
}

// aligner holds one alignment's state.
type aligner struct {
	a, b    []int32 // the two versions, each line as the number of its text
	matched []int32 // for each line of b, the line of a it is, or -1
	budget  int

	// counts of each text in the part of a and of b being anchored, kept at
	// zero between uses; table is the exact alignment's scratch space.
	countA, countB []int32
	table          []int32
}

// align returns, for each line of newer, the index of the line of older it is
// the same line as, or -1 for a line newer introduces.
func align(older, newer [][]byte) []int32 {
	numbers := make(map[string]int32, len(older)+len(newer))
	number := func(lines [][]byte) []int32 {
		out := make([]int32, len(lines))
		for i, line := range lines {
			n, ok := numbers[string(line)]
			if !ok {
				n = int32(len(numbers))
				numbers[string(line)] = n
			}
			out[i] = n
		}
		return out
	}
	x := &aligner{a: number(older), b: number(newer)}
	x.matched = make([]int32, len(newer))
	for i := range x.matched {
		x.matched[i] = -1
	}
	x.budget = workPerLine*(len(older)+len(newer)) + workBase
	x.countA = make([]int32, len(numbers))
	x.countB = make([]int32, len(numbers))
	x.gap(0, len(x.a), 0, len(x.b))
	return x.matched
}

// gap aligns a[aLo:aHi] with b[bLo:bHi].
func (x *aligner) gap(aLo, aHi, bLo, bHi int) {
	for aLo < aHi && bLo < bHi && x.a[aLo] == x.b[bLo] {
		x.matched[bLo] = int32(aLo)
		aLo, bLo = aLo+1, bLo+1
	}
	for aLo < aHi && bLo < bHi && x.a[aHi-1] == x.b[bHi-1] {
		aHi, bHi = aHi-1, bHi-1
		x.matched[bHi] = int32(aHi)
	}
	n, m := aHi-aLo, bHi-bLo
	if n == 0 || m == 0 {
		return
	}
	if n*m <= lcsCells {
		if x.spend(n * m) {
			x.exact(aLo, aHi, bLo, bHi)
		}
		return
	}
	if !x.spend(2 * (n + m)) {
		return
	}
	anchors := x.anchors(aLo, aHi, bLo, bHi)
	for _, anchor := range anchors {
		x.gap(aLo, anchor.a, bLo, anchor.b)
		x.matched[anchor.b] = int32(anchor.a)
		aLo, bLo = anchor.a+1, anchor.b+1
	}
	if len(anchors) > 0 {
		x.gap(aLo, aHi, bLo, bHi)
	}
}

// spend takes work from the budget, and reports whether there was enough.
func (x *aligner) spend(work int) bool {
	if work > x.budget {
		x.budget = 0
		return false
	}
	x.budget -= work
	return true
}

// exact aligns a gap by a longest common subsequence. table[i*(m+1)+j] is the
// length of the longest common subsequence of a[aLo+i:aHi] and b[bLo+j:bHi].
func (x *aligner) exact(aLo, aHi, bLo, bHi int) {
	n, m := aHi-aLo, bHi-bLo
	size := (n + 1) * (m + 1)
	if cap(x.table) < size {
		x.table = make([]int32, size)
	}
	table := x.table[:size]
	width := m + 1
	for j := 0; j <= m; j++ {
		table[n*width+j] = 0
	}
	for i := n - 1; i >= 0; i-- {
		table[i*width+m] = 0
		for j := m - 1; j >= 0; j-- {
			switch {
			case x.a[aLo+i] == x.b[bLo+j]:
				table[i*width+j] = table[(i+1)*width+j+1] + 1
			case table[(i+1)*width+j] >= table[i*width+j+1]:
				table[i*width+j] = table[(i+1)*width+j]
			default:
				table[i*width+j] = table[i*width+j+1]
			}
		}
	}
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case x.a[aLo+i] == x.b[bLo+j]:
			x.matched[bLo+j] = int32(aLo + i)
			i, j = i+1, j+1
		case table[(i+1)*width+j] >= table[i*width+j+1]:
			i++
		default:
			j++
		}
	}
}

// pair is a line of a and a line of b taken as the same line.
type pair struct{ a, b int }

// anchors returns the lines that occur exactly once in a[aLo:aHi] and once in
// b[bLo:bHi], as the longest run that is in order in both, in that order.
func (x *aligner) anchors(aLo, aHi, bLo, bHi int) []pair {
	for i := aLo; i < aHi; i++ {
		x.countA[x.a[i]]++
	}
	for j := bLo; j < bHi; j++ {
		x.countB[x.b[j]]++
	}
	position := make(map[int32]int)
	for i := aLo; i < aHi; i++ {
		if n := x.a[i]; x.countA[n] == 1 && x.countB[n] == 1 {
			position[n] = i
		}
	}
	var candidates []pair
	for j := bLo; j < bHi; j++ {
		if i, ok := position[x.b[j]]; ok {
			candidates = append(candidates, pair{a: i, b: j})
		}
	}
	for i := aLo; i < aHi; i++ {
		x.countA[x.a[i]] = 0
	}
	for j := bLo; j < bHi; j++ {
		x.countB[x.b[j]] = 0
	}
	return increasing(candidates)
}

// increasing returns the longest run of candidates, which are in increasing
// order of b, whose lines of a increase too. Of several longest, it returns
// the one patience sorting ends on: each candidate goes on the leftmost pile
// whose top is at a later line of a, and the run is read back from the top of
// the last pile.
func increasing(candidates []pair) []pair {
	if len(candidates) == 0 {
		return nil
	}
	tops := make([]int, 0, len(candidates)) // index of each pile's top candidate
	previous := make([]int, len(candidates))
	for k, c := range candidates {
		lo, hi := 0, len(tops)
		for lo < hi {
			mid := (lo + hi) / 2
			if candidates[tops[mid]].a < c.a {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		previous[k] = -1
		if lo > 0 {
			previous[k] = tops[lo-1]
		}
		if lo == len(tops) {
			tops = append(tops, k)
		} else {
			tops[lo] = k
		}
	}
	run := make([]pair, len(tops))
	for k, at := len(tops)-1, tops[len(tops)-1]; k >= 0; k, at = k-1, previous[at] {
		run[k] = candidates[at]
	}
	return run
}
