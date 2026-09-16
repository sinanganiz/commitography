package identity

import (
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
)

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
