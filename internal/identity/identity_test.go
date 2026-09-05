package identity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/model"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture %q not built; run `make fixtures`", name)
	}
	return path
}

func collectFixture(t *testing.T, name string, useMailmap bool) []model.Commit {
	t.Helper()
	h, err := collect.Collect(collect.Options{RepoPath: fixture(t, name), UseMailmap: useMailmap})
	if err != nil {
		t.Fatalf("Collect(%s): %v", name, err)
	}
	return h.Commits
}

func TestResolverReconcilesWithShortlogOnMailmapFixture(t *testing.T) {
	repo := fixture(t, "mailmap")
	commits := collectFixture(t, "mailmap", true)

	r := NewResolver(config.Default(), commits)
	got := len(r.Identities())

	out, err := exec.Command("git", "-C", repo, "shortlog", "-sn", "--all").Output()
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
	commits := collectFixture(t, "basic", false)

	plain := NewResolver(config.Default(), commits)
	a := plain.Commits(NormalizeEmail("ada@example.com"))
	b := plain.Commits(NormalizeEmail("ada.lovelace@corp.example.com"))
	if a == 0 || b == 0 {
		t.Fatalf("fixture should have commits under both addresses, got %d and %d", a, b)
	}

	cfg := config.Default()
	cfg.Identities = []config.Identity{{
		Name:   "Ada Lovelace",
		Emails: []string{"ada@example.com", "ada.lovelace@corp.example.com"},
	}}

	merged := NewResolver(cfg, commits)
	if got := merged.Commits("ada@example.com"); got != a+b {
		t.Errorf("merged identity has %d commits, want %d", got, a+b)
	}
	if len(merged.Identities()) != len(plain.Identities())-1 {
		t.Errorf("merging two addresses should reduce the identity count by one")
	}

	id, ok := merged.Lookup("ada@example.com")
	if !ok {
		t.Fatal("merged identity not found")
	}
	if id.DisplayName != "Ada Lovelace" {
		t.Errorf("DisplayName = %q, want the configured name", id.DisplayName)
	}
	if len(id.Emails) != 2 {
		t.Errorf("merged identity holds %d emails, want 2", len(id.Emails))
	}
}

func TestBotsAreFlagged(t *testing.T) {
	commits := collectFixture(t, "bots", true)
	r := NewResolver(config.Default(), commits)

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

func TestUnconfiguredDisplayNameComesFromMostRecentCommit(t *testing.T) {
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	commits := []model.Commit{
		{Hash: "a", AuthorName: "Old Name", AuthorEmail: "P@Example.com", AuthorDate: base},
		{Hash: "b", AuthorName: "New Name", AuthorEmail: "p@example.com", AuthorDate: base.AddDate(1, 0, 0)},
	}

	r := NewResolver(config.Default(), commits)
	if got := len(r.Identities()); got != 1 {
		t.Fatalf("case differences split one person into %d identities", got)
	}
	id, _ := r.Lookup("p@example.com")
	if id.DisplayName != "New Name" {
		t.Errorf("DisplayName = %q, want %q", id.DisplayName, "New Name")
	}
}

func TestIdentitiesSortedByDescendingCommitCount(t *testing.T) {
	var commits []model.Commit
	for i := 0; i < 5; i++ {
		commits = append(commits, model.Commit{AuthorName: "A", AuthorEmail: "a@x.com"})
	}
	for i := 0; i < 3; i++ {
		commits = append(commits, model.Commit{AuthorName: "B", AuthorEmail: "b@x.com"})
	}
	commits = append(commits, model.Commit{AuthorName: "C", AuthorEmail: "c@x.com"})

	got := NewResolver(config.Default(), commits).Identities()
	want := []string{"a@x.com", "b@x.com", "c@x.com"}
	for i, id := range got {
		if id.ID != want[i] {
			t.Errorf("position %d = %q, want %q", i, id.ID, want[i])
		}
	}
}

func TestResolveIsStableForUnknownAddresses(t *testing.T) {
	r := NewResolver(config.Default(), nil)
	if got := r.Resolve("Someone", "  Someone@Example.COM "); got != "someone@example.com" {
		t.Errorf("Resolve = %q, want the normalized address", got)
	}
}
