package aggregate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// authored builds a commit by name and address on a UTC day of 2026.
func authored(name, email string, day int) model.Commit {
	when := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, day)
	return model.Commit{
		Hash:          fmt.Sprintf("%s-%d", email, day),
		AuthorName:    name,
		AuthorEmail:   email,
		AuthorDate:    when,
		CommitterDate: when,
	}
}

// identitiesOf resolves commits the way the pipeline does and builds the
// identities section over all of them.
func identitiesOf(t *testing.T, cfg config.Config, commits []model.Commit) []core.IdentityEntry {
	t.Helper()
	resolver := identity.NewResolver(cfg, commits)
	for i := range commits {
		commits[i].IdentityID = resolver.Resolve(commits[i].AuthorName, commits[i].AuthorEmail)
	}
	return buildIdentities(core.Input{Config: cfg, Resolver: resolver}, commits)
}

func TestIdentitiesAreOrderedByFirstCommitDateNotVolume(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		// Grace commits most but starts last; Ada and Alan start the same day.
		authored("Grace", "grace@example.com", 5),
		authored("Grace", "grace@example.com", 6),
		authored("Grace", "grace@example.com", 7),
		authored("Alan", "alan@example.com", 2),
		authored("Ada", "ada@example.com", 2),
		authored("Ada", "ada@example.com", 9),
	}
	got := identitiesOf(t, config.Default(), commits)
	if len(got) != 3 {
		t.Fatalf("identities = %+v, want 3", got)
	}
	if got[2].DisplayName != "Grace" || got[2].CommitCount != 3 {
		t.Errorf("last entry = %+v, want Grace with 3 commits", got[2])
	}
	if got[0].FirstCommitDate != "2026-01-03" || got[1].FirstCommitDate != "2026-01-03" || got[0].ID > got[1].ID {
		t.Errorf("first two entries = %+v, %+v; want the same first date, ordered by id", got[0], got[1])
	}
	for _, e := range got {
		if e.DisplayName == "Ada" && (e.LastCommitDate != "2026-01-10" || e.CommitCount != 2) {
			t.Errorf("Ada = %+v, want last 2026-01-10 and 2 commits", e)
		}
		if e.ID != core.IdentityDigest(strings.ToLower(e.DisplayName)+"@example.com") {
			t.Errorf("%s has id %q, want the digest of the address", e.DisplayName, e.ID)
		}
		if e.Aggregate {
			t.Errorf("%s is flagged aggregate below the limit", e.DisplayName)
		}
	}
}

func TestIdentitiesAreBoundedWithOneAggregateEntry(t *testing.T) {
	t.Parallel()
	const extra = 5
	var commits []model.Commit
	for i := 0; i < core.LimitIdentities+extra; i++ {
		email := fmt.Sprintf("person%03d@example.com", i)
		// The first identities have one commit; every other identity has two,
		// so the least active are the ones folded, whatever their dates.
		commits = append(commits, authored(fmt.Sprintf("Person %03d", i), email, i))
		if i >= extra {
			commits = append(commits, authored(fmt.Sprintf("Person %03d", i), email, 400))
		}
	}
	got := identitiesOf(t, config.Default(), commits)
	if len(got) != core.LimitIdentities+1 {
		t.Fatalf("got %d entries, want %d individual and one aggregate", len(got), core.LimitIdentities)
	}
	last := got[len(got)-1]
	if !last.Aggregate || last.ID != "" || last.DisplayName != "5 other identities" || last.CommitCount != extra {
		t.Errorf("aggregate entry = %+v, want the five least active folded, without an id", last)
	}
	if last.FirstCommitDate != "2026-01-01" || last.LastCommitDate != "2026-01-05" {
		t.Errorf("aggregate dates = %s..%s, want the span of the folded identities", last.FirstCommitDate, last.LastCommitDate)
	}
	for i, e := range got[:len(got)-1] {
		if e.Aggregate || e.CommitCount != 2 {
			t.Errorf("entry %d = %+v, want an individual identity with 2 commits", i, e)
		}
		if i > 0 && e.FirstCommitDate < got[i-1].FirstCommitDate {
			t.Errorf("entry %d starts %s, before entry %d at %s", i, e.FirstCommitDate, i-1, got[i-1].FirstCommitDate)
		}
	}
}

func TestIdentitiesCarryNoRawAddress(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		// No name: the resolver falls back to the address itself.
		authored("", "nameless@example.com", 1),
		// A name that is, or contains, an address.
		authored("someone@example.com", "someone@example.com", 2),
		authored("Mallory <mallory@example.org>", "mallory@example.org", 3),
		authored("Ada", "ada@example.com", 4),
	}
	got := identitiesOf(t, config.Default(), commits)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("encoding the identities: %v", err)
	}
	if strings.Contains(string(encoded), "@") {
		t.Errorf("the identities section carries an address: %s", encoded)
	}
	for _, e := range got {
		if e.DisplayName != "Ada" && e.DisplayName != e.ID {
			t.Errorf("entry %+v: a name that is empty or holds an address is replaced by the id", e)
		}
	}
}

func TestIdentitiesAnonymisedCarryNoName(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Anonymize = true
	got := identitiesOf(t, cfg, []model.Commit{authored("Ada", "ada@example.com", 1)})
	if len(got) != 1 || got[0].DisplayName != got[0].ID || got[0].ID == "" {
		t.Errorf("anonymised identities = %+v, want the id in place of the name", got)
	}
}

func TestIdentitiesWithoutAnAddressStillHaveAnID(t *testing.T) {
	t.Parallel()
	got := identitiesOf(t, config.Default(), []model.Commit{authored("Nobody", "", 1)})
	if len(got) != 1 || len(got[0].ID) != len(core.IdentityDigest("a@example.com")) {
		t.Errorf("identities = %+v, want one entry with a digest id", got)
	}
}

func TestIdentitiesOverNoCommitIsAnEmptyList(t *testing.T) {
	t.Parallel()
	got := identitiesOf(t, config.Default(), nil)
	encoded, err := json.Marshal(got)
	if err != nil || string(encoded) != "[]" {
		t.Errorf("identities over no commit encode as %s (%v), want []", encoded, err)
	}
}
