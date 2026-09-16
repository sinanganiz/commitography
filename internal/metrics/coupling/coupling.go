// Package coupling is the coupling metric family (ADR-0024, ADR-0040): which
// files keep changing together.
package coupling

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
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
)

// BuildCoupling finds file pairs that keep changing together. Confidence is
// measured against the rarer of the two files, so a pair is only reported when
// the smaller partner nearly always brings the larger one along.
func BuildCoupling(scoped []core.ScopedCommit) ([]core.CoupledPair, []string) {
	var warnings []string

	changes := map[string]int{}
	support := map[[2]string]int{}
	droppedSingles := false

	for _, sc := range scoped {
		paths := uniquePaths(sc.Files)
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

	out := []core.CoupledPair{}
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
		out = append(out, core.CoupledPair{
			A:          key[0],
			B:          key[1],
			Support:    n,
			Confidence: core.Round(confidence, 4),
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
