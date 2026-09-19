package identity

import (
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
)

func TestUnconfiguredDisplayNameComesFromMostRecentCommit(t *testing.T) {
	t.Parallel()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	commits := []model.Commit{
		{Hash: "a", AuthorName: "Old Name", AuthorEmail: "P@Example.com", AuthorDate: base},
		{Hash: "b", AuthorName: "New Name", AuthorEmail: "p@example.com", AuthorDate: base.AddDate(1, 0, 0)},
	}

	r := NewResolver(config.Default(), commits)
	if got := len(r.Identities()); got != 1 {
		t.Fatalf("case differences split one person into %d identities", got)
	}
	id, _ := r.Lookup(Digest("p@example.com"))
	if id.DisplayName != "New Name" {
		t.Errorf("DisplayName = %q, want %q", id.DisplayName, "New Name")
	}
}

func TestIdentitiesSortedByDescendingCommitCount(t *testing.T) {
	t.Parallel()
	var commits []model.Commit
	for i := 0; i < 5; i++ {
		commits = append(commits, model.Commit{AuthorName: "A", AuthorEmail: "a@x.com"})
	}
	for i := 0; i < 3; i++ {
		commits = append(commits, model.Commit{AuthorName: "B", AuthorEmail: "b@x.com"})
	}
	commits = append(commits, model.Commit{AuthorName: "C", AuthorEmail: "c@x.com"})

	got := NewResolver(config.Default(), commits).Identities()
	want := []string{Digest("a@x.com"), Digest("b@x.com"), Digest("c@x.com")}
	for i, id := range got {
		if id.Digest != want[i] {
			t.Errorf("position %d = %q, want %q", i, id.Digest, want[i])
		}
	}
}

// A configuration may name an address by the digest a report shows in its
// place, and the two resolve alike (ADR-0068 clause 3).
func TestConfiguredDigestResolvesLikeTheAddress(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		{Hash: "a", AuthorName: "Ada", AuthorEmail: "ada@example.com"},
		{Hash: "b", AuthorName: "A. L.", AuthorEmail: "ada.lovelace@corp.example.com"},
		{Hash: "c", AuthorName: "Grace", AuthorEmail: "grace@example.com"},
	}
	byAddress := config.Default()
	byAddress.Identities = []config.Identity{{
		Name:   "Ada Lovelace",
		Emails: []string{"ada@example.com", "ada.lovelace@corp.example.com"},
	}}
	byDigest := config.Default()
	byDigest.Identities = []config.Identity{{
		Name:   "Ada Lovelace",
		Emails: []string{Digest("ada@example.com"), Digest("ada.lovelace@corp.example.com")},
	}}

	for name, cfg := range map[string]config.Analysis{"addresses": byAddress, "digests": byDigest} {
		r := NewResolver(cfg, commits)
		if got := len(r.Identities()); got != 2 {
			t.Errorf("%s: %d identities, want Ada merged with her second address and Grace", name, got)
		}
		ada, ok := r.Lookup(Digest("ada@example.com"))
		if !ok {
			t.Fatalf("%s: the configured identity is missing", name)
		}
		if ada.DisplayName != "Ada Lovelace" || !ada.Resolved() {
			t.Errorf("%s: the configured identity is %v, resolved %v", name, ada, ada.Resolved())
		}
		if got := r.Resolve("", "ada.lovelace@corp.example.com"); got != ada.Digest {
			t.Errorf("%s: the second address resolves to %q, want %q", name, got, ada.Digest)
		}
		if got := r.Commits(ada.Digest); got != 2 {
			t.Errorf("%s: the merged identity has %d commits, want 2", name, got)
		}
	}
}

// An exclusion entry is read by what it is: a reference names one identity, an
// entry with @ is an address and matches addresses only, and anything else is
// a name or a class pattern (ADR-0068 clauses 2 and 5).
func TestExclusionEntriesMatchByWhatTheyAre(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		{Hash: "a", AuthorName: "Ada", AuthorEmail: "ada@example.com"},
		// A display name that is an address belonging to someone else.
		{Hash: "b", AuthorName: "ada@example.com", AuthorEmail: "impostor@example.com"},
		{Hash: "c", AuthorName: "release-robot", AuthorEmail: "robot@example.com"},
	}
	ada, impostor, robot := Digest("ada@example.com"), Digest("impostor@example.com"), Digest("robot@example.com")

	for _, tc := range []struct {
		entry string
		want  []string
	}{
		{"ada@example.com", []string{ada}},
		{ada, []string{ada}},
		{"release-robot", []string{robot}},
		{"nobody@example.com", nil},
	} {
		cfg := config.Default()
		cfg.ExcludeAuthors = []string{tc.entry}
		r := NewResolver(cfg, commits)
		var excluded []string
		for _, id := range r.Identities() {
			if id.IsBot {
				excluded = append(excluded, id.Digest)
			}
		}
		if len(excluded) != len(tc.want) {
			t.Errorf("%q excluded %v, want %v", tc.entry, excluded, tc.want)
			continue
		}
		for i := range excluded {
			if excluded[i] != tc.want[i] {
				t.Errorf("%q excluded %v, want %v", tc.entry, excluded, tc.want)
			}
		}
		if got := r.MatchedBy(tc.entry); len(got) != len(tc.want) {
			t.Errorf("MatchedBy(%q) = %v, want %v", tc.entry, got, tc.want)
		}
	}

	// The impostor is excluded by nothing above: its display name looks like
	// Ada's address, and an address entry matches addresses only.
	cfg := config.Default()
	cfg.ExcludeAuthors = []string{"ada@example.com"}
	if id, _ := NewResolver(cfg, commits).Lookup(impostor); id.IsBot {
		t.Error("an address entry matched a display name that looks like that address")
	}
}

func TestIsReference(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]bool{
		Digest("ada@example.com"): true,
		"0123456789abcdef":        true,
		"0123456789ABCDEF":        false,
		"0123456789abcde":         false,
		"0123456789abcdefa":       false,
		"ada@example.com":         false,
		"":                        false,
	} {
		if got := IsReference(value); got != want {
			t.Errorf("IsReference(%q) = %v, want %v", value, got, want)
		}
	}
	if got := Reference("  Ada@Example.com "); got != Digest("ada@example.com") {
		t.Errorf("Reference of an address = %q, want its digest", got)
	}
	if got := Reference(Digest("ada@example.com")); got != Digest("ada@example.com") {
		t.Errorf("Reference of a digest = %q, want it unchanged", got)
	}
	if got := Reference("   "); got != "" {
		t.Errorf("Reference of nothing = %q, want the empty string", got)
	}
}

func TestResolveIsStableForUnknownAddresses(t *testing.T) {
	t.Parallel()
	r := NewResolver(config.Default(), nil)
	if got := r.Resolve("Someone", "  Someone@Example.COM "); got != Digest("someone@example.com") {
		t.Errorf("Resolve = %q, want the digest of the normalized address", got)
	}
}
