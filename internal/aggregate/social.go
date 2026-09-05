package aggregate

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/model"
)

const (
	// couplingMinSupport and couplingMinConfidence keep the coupling list to
	// pairs that genuinely travel together rather than pairs that coincided
	// once or twice.
	couplingMinSupport    = 5
	couplingMinConfidence = 0.5
	couplingLimit         = 50

	// couplingMaxPairs bounds memory on repositories with very wide commits.
	// Past it, single-support pairs are dropped: they can never reach the
	// reporting threshold anyway.
	couplingMaxPairs = 5_000_000

	// churnWindowDays and churnMinCommits define a hotspot: a file rewritten
	// repeatedly inside one month.
	churnWindowDays = 30
	churnMinCommits = 5
	churnLimit      = 25

	// directoryMinWork is the activity a directory needs before its bus factor
	// is worth reporting. Below it the number is noise.
	directoryMinWork = 10
)

// DirectoryBusFactor is the bus factor of one directory.
type DirectoryBusFactor struct {
	Path         string `json:"path"`
	BusFactor    int    `json:"busFactor"`
	Contributors int    `json:"contributors"`
	Commits      int    `json:"commits"`
}

// CoupledPair is a pair of files that keep changing together.
type CoupledPair struct {
	A          string  `json:"a"`
	B          string  `json:"b"`
	Support    int     `json:"support"`
	Confidence float64 `json:"confidence"`
	// Expected marks pairs sharing a basename, such as a file and its test, so
	// the UI can de-emphasize coupling nobody needs to be told about.
	Expected bool `json:"expected"`
}

// ChurnHotspot is a file repeatedly rewritten inside a short window.
type ChurnHotspot struct {
	Path               string    `json:"path"`
	MaxCommitsInWindow int       `json:"maxCommitsInWindow"`
	WindowStart        time.Time `json:"windowStart"`
	TotalCommits       int       `json:"totalCommits"`
	Added              int       `json:"added"`
	Deleted            int       `json:"deleted"`
}

// KnowledgeShare reports how concentrated a directory's history is. The
// contributor responsible is named only when --per-author was requested.
type KnowledgeShare struct {
	Path           string  `json:"path"`
	LargestShare   float64 `json:"largestShare"`
	Contributors   int     `json:"contributors"`
	Commits        int     `json:"commits"`
	TopContributor string  `json:"topContributor,omitempty"`
}

// SocialMetrics describes where knowledge sits and what moves together.
type SocialMetrics struct {
	BusFactor              int                  `json:"busFactor"`
	DirectoryBusFactor     []DirectoryBusFactor `json:"directoryBusFactor"`
	Coupling               []CoupledPair        `json:"coupling"`
	Churn                  []ChurnHotspot       `json:"churn"`
	KnowledgeConcentration []KnowledgeShare     `json:"knowledgeConcentration"`
}

// scopedCommit is one commit reduced to what the social metrics need: its
// identity, its timestamp, and the files that survived path exclusion.
type scopedCommit struct {
	identityID string
	when       time.Time
	files      []model.FileChange
}

func buildSocial(in Input, commits []model.Commit) (SocialMetrics, []string) {
	var warnings []string
	m := SocialMetrics{
		DirectoryBusFactor:     []DirectoryBusFactor{},
		Coupling:               []CoupledPair{},
		Churn:                  []ChurnHotspot{},
		KnowledgeConcentration: []KnowledgeShare{},
	}

	scoped := make([]scopedCommit, 0, len(commits))
	for _, c := range commits {
		files := filter.IncludedFiles(c, in.PathFilter)
		if len(files) == 0 {
			continue
		}
		scoped = append(scoped, scopedCommit{
			identityID: c.IdentityID,
			when:       in.date(c),
			files:      files,
		})
	}

	repoCounts, dirCounts := contributionCounts(scoped)
	m.BusFactor = busFactor(repoCounts)
	m.DirectoryBusFactor = directoryBusFactors(dirCounts)
	m.KnowledgeConcentration = knowledgeConcentration(in, dirCounts)

	coupling, couplingWarnings := buildCoupling(scoped)
	m.Coupling = coupling
	warnings = append(warnings, couplingWarnings...)

	m.Churn = buildChurn(scoped)

	return m, warnings
}

// contributionCounts tallies, per identity, the commits touching the repository
// as a whole and each directory at depth 1 and 2.
func contributionCounts(scoped []scopedCommit) (repo map[string]int, dirs map[string]map[string]int) {
	repo = map[string]int{}
	dirs = map[string]map[string]int{}
	for _, sc := range scoped {
		repo[sc.identityID]++
		for _, dir := range scopesOf(sc.files) {
			if dirs[dir] == nil {
				dirs[dir] = map[string]int{}
			}
			dirs[dir][sc.identityID]++
		}
	}
	return repo, dirs
}

func directoryBusFactors(dirCounts map[string]map[string]int) []DirectoryBusFactor {
	out := []DirectoryBusFactor{}
	for dir, counts := range dirCounts {
		total := 0
		for _, n := range counts {
			total += n
		}
		if total < directoryMinWork {
			continue
		}
		out = append(out, DirectoryBusFactor{
			Path:         dir,
			BusFactor:    busFactor(counts),
			Contributors: len(counts),
			Commits:      total,
		})
	}
	// Most fragile first: the directories a team should look at.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.BusFactor != b.BusFactor {
			return a.BusFactor < b.BusFactor
		}
		if a.Commits != b.Commits {
			return a.Commits > b.Commits
		}
		return a.Path < b.Path
	})
	return out
}

func knowledgeConcentration(in Input, dirCounts map[string]map[string]int) []KnowledgeShare {
	out := []KnowledgeShare{}
	for dir, counts := range dirCounts {
		if strings.Contains(dir, "/") { // depth-1 directories only
			continue
		}
		total, largest, leader := 0, 0, ""
		for id, n := range counts {
			total += n
			if n > largest || (n == largest && (leader == "" || id < leader)) {
				largest, leader = n, id
			}
		}
		if total < directoryMinWork {
			continue
		}
		share := KnowledgeShare{
			Path:         dir,
			LargestShare: round(float64(largest)/float64(total), 4),
			Contributors: len(counts),
			Commits:      total,
		}
		if in.PerAuthor {
			share.TopContributor = displayName(in, leader)
		}
		out = append(out, share)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.LargestShare != b.LargestShare {
			return a.LargestShare > b.LargestShare
		}
		return a.Path < b.Path
	})
	return out
}

// busFactor is the smallest number of contributors accounting for at least half
// the work in a scope. It answers "how many people could leave before this code
// has no owner", not "who is most productive". Ties break on identity so the
// figure never changes between runs.
func busFactor(counts map[string]int) int {
	if len(counts) == 0 {
		return 0
	}
	type row struct {
		id string
		n  int
	}
	rows := make([]row, 0, len(counts))
	total := 0
	for id, n := range counts {
		rows = append(rows, row{id, n})
		total += n
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].id < rows[j].id
	})

	running := 0
	for i, r := range rows {
		running += r.n
		if float64(running) >= float64(total)/2 {
			return i + 1
		}
	}
	return len(rows)
}

// scopesOf returns the depth-1 and depth-2 directories a commit touched, each
// counted at most once per commit.
func scopesOf(files []model.FileChange) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		dir := path.Dir(f.Path)
		if dir == "." || dir == "/" || dir == "" {
			continue
		}
		parts := strings.Split(dir, "/")
		for depth := 1; depth <= 2 && depth <= len(parts); depth++ {
			scope := strings.Join(parts[:depth], "/")
			if !seen[scope] {
				seen[scope] = true
				out = append(out, scope)
			}
		}
	}
	return out
}

// buildCoupling finds file pairs that keep changing together. Confidence is
// measured against the rarer of the two files, so a pair is only reported when
// the smaller partner nearly always brings the larger one along.
func buildCoupling(scoped []scopedCommit) ([]CoupledPair, []string) {
	var warnings []string

	changes := map[string]int{}
	support := map[[2]string]int{}
	droppedSingles := false

	for _, sc := range scoped {
		paths := uniquePaths(sc.files)
		if len(paths) < 2 || len(paths) > filter.CouplingMaxFilesPerCommit {
			// A commit touching one file couples nothing; a commit touching
			// hundreds couples everything to everything, which is noise.
			for _, p := range paths {
				changes[p]++
			}
			continue
		}
		for _, p := range paths {
			changes[p]++
		}
		for i := 0; i < len(paths); i++ {
			for j := i + 1; j < len(paths); j++ {
				support[pairKey(paths[i], paths[j])]++
			}
		}

		if len(support) > couplingMaxPairs {
			for key, n := range support {
				if n == 1 {
					delete(support, key)
				}
			}
			droppedSingles = true
		}
	}

	if droppedSingles {
		warnings = append(warnings, fmt.Sprintf(
			"change coupling exceeded %d tracked file pairs; pairs seen in only one commit were discarded",
			couplingMaxPairs))
	}

	out := []CoupledPair{}
	for key, n := range support {
		if n < couplingMinSupport {
			continue
		}
		smaller := changes[key[0]]
		if changes[key[1]] < smaller {
			smaller = changes[key[1]]
		}
		if smaller == 0 {
			continue
		}
		confidence := float64(n) / float64(smaller)
		if confidence < couplingMinConfidence {
			continue
		}
		out = append(out, CoupledPair{
			A:          key[0],
			B:          key[1],
			Support:    n,
			Confidence: round(confidence, 4),
			Expected:   sharesBaseName(key[0], key[1]),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Support != b.Support {
			return a.Support > b.Support
		}
		if a.Confidence != b.Confidence {
			return a.Confidence > b.Confidence
		}
		if a.A != b.A {
			return a.A < b.A
		}
		return a.B < b.B
	})
	if len(out) > couplingLimit {
		out = out[:couplingLimit]
	}
	return out, warnings
}

// buildChurn finds files touched at least churnMinCommits times inside any
// rolling 30-day window. A file being rewritten that often is either the heart
// of the system or a place nobody has got right yet.
func buildChurn(scoped []scopedCommit) []ChurnHotspot {
	type touch struct {
		when    time.Time
		added   int
		deleted int
	}
	byPath := map[string][]touch{}
	for _, sc := range scoped {
		for _, f := range sc.files {
			byPath[f.Path] = append(byPath[f.Path], touch{sc.when, f.Added, f.Deleted})
		}
	}

	out := []ChurnHotspot{}
	window := time.Duration(churnWindowDays) * 24 * time.Hour

	for p, touches := range byPath {
		if len(touches) < churnMinCommits {
			continue
		}
		sort.Slice(touches, func(i, j int) bool { return touches[i].when.Before(touches[j].when) })

		best, bestStart := 0, time.Time{}
		left := 0
		for right := range touches {
			for touches[right].when.Sub(touches[left].when) > window {
				left++
			}
			if n := right - left + 1; n > best {
				best, bestStart = n, touches[left].when
			}
		}
		if best < churnMinCommits {
			continue
		}

		hotspot := ChurnHotspot{
			Path:               p,
			MaxCommitsInWindow: best,
			WindowStart:        bestStart,
			TotalCommits:       len(touches),
		}
		for _, t := range touches {
			hotspot.Added += t.added
			hotspot.Deleted += t.deleted
		}
		out = append(out, hotspot)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.MaxCommitsInWindow != b.MaxCommitsInWindow {
			return a.MaxCommitsInWindow > b.MaxCommitsInWindow
		}
		if a.TotalCommits != b.TotalCommits {
			return a.TotalCommits > b.TotalCommits
		}
		return a.Path < b.Path
	})
	if len(out) > churnLimit {
		out = out[:churnLimit]
	}
	return out
}

// pairKey orders two paths so a pair has exactly one representation.
func pairKey(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

func uniquePaths(files []model.FileChange) []string {
	seen := make(map[string]bool, len(files))
	out := make([]string, 0, len(files))
	for _, f := range files {
		if seen[f.Path] {
			continue
		}
		seen[f.Path] = true
		out = append(out, f.Path)
	}
	sort.Strings(out)
	return out
}

// sharesBaseName reports whether two paths are obviously related, such as
// Foo.ts and Foo.test.ts, so their coupling can be shown but de-emphasized.
func sharesBaseName(a, b string) bool {
	return stemOf(a) == stemOf(b)
}

// stemOf strips every extension from a basename: Foo.test.ts becomes Foo.
func stemOf(p string) string {
	base := path.Base(p)
	for {
		ext := path.Ext(base)
		if ext == "" || ext == base {
			return strings.ToLower(base)
		}
		base = strings.TrimSuffix(base, ext)
	}
}

func displayName(in Input, identityID string) string {
	if in.Resolver == nil {
		return identityID
	}
	if id, ok := in.Resolver.Lookup(identityID); ok {
		return id.DisplayName
	}
	return identityID
}
