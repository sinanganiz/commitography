package pipeline

import (
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

func collectFixture(t *testing.T, name string, useMailmap bool) []model.Commit {
	t.Helper()
	h, err := newCollector().Collect(collect.Options{RepoPath: fixture(t, name), UseMailmap: useMailmap})
	if err != nil {
		t.Fatalf("Collect(%s): %v", name, err)
	}
	return h.Commits
}

func TestResolverReconcilesWithShortlogOnMailmapFixture(t *testing.T) {
	t.Parallel()
	repo := fixture(t, "mailmap")
	commits := collectFixture(t, "mailmap", true)

	r := identity.NewResolver(config.Default(), commits)
	got := len(r.Identities())

	out, err := git.Command(repo, "shortlog", "-sn", "--all").Output()
	if err != nil {
		t.Fatalf("git shortlog: %v", err)
	}
	want := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			want++
		}
	}

	if got != want {
		t.Errorf("resolved %d identities, git shortlog reports %d", got, want)
	}
}

func TestConfiguredIdentityMergesTwoEmails(t *testing.T) {
	t.Parallel()
	commits := collectFixture(t, "basic", false)

	plain := identity.NewResolver(config.Default(), commits)
	a := plain.Commits(identity.Digest("ada@example.com"))
	b := plain.Commits(identity.Digest("ada.lovelace@corp.example.com"))
	if a == 0 || b == 0 {
		t.Fatalf("fixture should have commits under both addresses, got %d and %d", a, b)
	}

	cfg := config.Default()
	cfg.Identities = []config.Identity{{
		Name:   "Ada Lovelace",
		Emails: []string{"ada@example.com", "ada.lovelace@corp.example.com"},
	}}

	merged := identity.NewResolver(cfg, commits)
	if got := merged.Commits(identity.Digest("ada@example.com")); got != a+b {
		t.Errorf("merged identity has %d commits, want %d", got, a+b)
	}
	if len(merged.Identities()) != len(plain.Identities())-1 {
		t.Errorf("merging two addresses should reduce the identity count by one")
	}

	id, ok := merged.Lookup(identity.Digest("ada@example.com"))
	if !ok {
		t.Fatal("merged identity not found")
	}
	if id.DisplayName != "Ada Lovelace" {
		t.Errorf("DisplayName = %q, want the configured name", id.DisplayName)
	}
	if got := merged.Resolve("", "ada.lovelace@corp.example.com"); got != id.Digest {
		t.Errorf("the second address resolves to %q, want the merged identity %q", got, id.Digest)
	}
}

func TestBotsAreFlagged(t *testing.T) {
	t.Parallel()
	commits := collectFixture(t, "bots", true)
	r := identity.NewResolver(config.Default(), commits)

	bots := 0
	for _, id := range r.Identities() {
		if id.IsBot {
			bots++
		}
	}
	if bots != 2 {
		t.Errorf("flagged %d bot identities, want 2 (dependabot and renovate)", bots)
	}

	for _, id := range r.Identities() {
		if strings.Contains(id.DisplayName, "[bot]") && !id.IsBot {
			t.Errorf("identity %q not flagged as a bot", id.DisplayName)
		}
		if id.DisplayName == "Ada Lovelace" && id.IsBot {
			t.Error("a human contributor was flagged as a bot")
		}
	}
}
