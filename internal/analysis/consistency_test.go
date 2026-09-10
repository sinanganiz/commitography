package analysis

import (
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/model"
)

func TestRepositoryChangedDetectsRelevantMutations(t *testing.T) {
	base := model.RepositoryInfo{
		HeadCommit:    "abc",
		DefaultBranch: "main",
	}

	cases := []struct {
		name   string
		end    model.RepositoryInfo
		stale  bool
		reason string
	}{
		{name: "stable", end: base, stale: false},
		{name: "head", end: model.RepositoryInfo{HeadCommit: "def", DefaultBranch: "main"}, stale: true, reason: "HEAD"},
		{name: "branch", end: model.RepositoryInfo{HeadCommit: "abc", DefaultBranch: "develop"}, stale: true, reason: "checkout"},
		{name: "metadata", end: model.RepositoryInfo{HeadCommit: "abc", DefaultBranch: "main", IsShallow: true}, stale: true, reason: "metadata"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stale, reason := repositoryChanged(base, tc.end)
			if stale != tc.stale {
				t.Fatalf("stale = %v, want %v", stale, tc.stale)
			}
			if tc.reason != "" && reason != tc.reason && !strings.Contains(reason, tc.reason) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.reason)
			}
		})
	}
}
