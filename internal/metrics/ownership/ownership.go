// Package ownership is the ownership metric family (ADR-0024, ADR-0040): where
// knowledge of the code sits.
package ownership

import (
	"path"
	"sort"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// directoryMinWork is the activity a directory needs before its bus factor
// is worth reporting. Below it the number is noise.
const directoryMinWork = 10

// BuildOwnership fills the ownership figures of the social section: the
// repository and directory bus factors and knowledge concentration.
func BuildOwnership(in core.Input, scoped []core.ScopedCommit, m *core.SocialMetrics) {
	repoCounts, dirCounts := contributionCounts(scoped)
	m.BusFactor = busFactor(repoCounts)
	m.DirectoryBusFactor = directoryBusFactors(dirCounts)
	m.KnowledgeConcentration = knowledgeConcentration(in, dirCounts)
}

// contributionCounts tallies, per identity, the commits touching the repository
// as a whole and each directory at depth 1 and 2.
func contributionCounts(scoped []core.ScopedCommit) (repo map[string]int, dirs map[string]map[string]int) {
	repo = map[string]int{}
	dirs = map[string]map[string]int{}
	for _, sc := range scoped {
		repo[sc.IdentityID]++
		for _, dir := range scopesOf(sc.Files) {
			if dirs[dir] == nil {
				dirs[dir] = map[string]int{}
			}
			dirs[dir][sc.IdentityID]++
		}
	}
	return repo, dirs
}

func directoryBusFactors(dirCounts map[string]map[string]int) []core.DirectoryBusFactor {
	out := []core.DirectoryBusFactor{}
	for dir, counts := range dirCounts {
		total := 0
		for _, n := range counts {
			total += n
		}
		if total < directoryMinWork {
			continue
		}
		out = append(out, core.DirectoryBusFactor{
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

func knowledgeConcentration(in core.Input, dirCounts map[string]map[string]int) []core.KnowledgeShare {
	out := []core.KnowledgeShare{}
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
		share := core.KnowledgeShare{
			Path:         dir,
			LargestShare: core.Round(float64(largest)/float64(total), 4),
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

func displayName(in core.Input, identityID string) string {
	if in.Resolver == nil {
		return identityID
	}
	if id, ok := in.Resolver.Lookup(identityID); ok {
		return id.DisplayName
	}
	return identityID
}

// SurvivingFromFirstYear is the share of blamed lines last written in the
// calendar year of the earliest analyzed commit. It is nil when nothing was
// blamed.
func SurvivingFromFirstYear(in core.Input, analyzed []model.Commit, ages []core.YearLines) *float64 {
	if total := totalLines(ages); total > 0 && len(analyzed) > 0 {
		firstYear := in.Date(earliestCommit(in, analyzed)).Year()
		surviving := 0
		for _, a := range ages {
			if a.Year == firstYear {
				surviving = a.Lines
			}
		}
		ratio := core.Round(float64(surviving)/float64(total), 4)
		return &ratio
	}
	return nil
}

func totalLines(ages []core.YearLines) int {
	total := 0
	for _, a := range ages {
		total += a.Lines
	}
	return total
}

func earliestCommit(in core.Input, commits []model.Commit) model.Commit {
	best := commits[0]
	for _, c := range commits[1:] {
		if in.Date(c).Before(in.Date(best)) {
			best = c
		}
	}
	return best
}
