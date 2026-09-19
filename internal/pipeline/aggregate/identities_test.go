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

func TestAnUnresolvableAuthorDegradesTheIdentityAttributedFamilies(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	commits := []model.Commit{authored("Ada", "ada@example.com", 1), authored("Nobody", "  ", 2)}
	resolver := identity.NewResolver(cfg, commits)
	for i := range commits {
		commits[i].IdentityID = resolver.Resolve(commits[i].AuthorName, commits[i].AuthorEmail)
	}
	in := core.Input{Config: cfg, Resolver: resolver}
	if !unresolvedAuthor(in, commits) {
		t.Fatal("a commit carrying no address was treated as resolved")
	}
	if unresolvedAuthor(in, commits[:1]) {
		t.Fatal("a commit carrying an address was treated as unresolved")
	}

	// The identity-attributed families are computed here, as they will be
	// once their packages exist, so the degradation can be observed.
	var f core.Families
	f.Temporal = core.Computed(core.Version{Major: 1}, core.TemporalMetrics{})
	f.Ownership = core.Computed(core.Version{Major: 1}, core.OwnershipMetrics{})
	f.Worktype = core.Computed(core.Version{Major: 1}, core.WorktypeMetrics{})
	f.AIArchaeology = core.Skipped[core.AIArchaeologyMetrics](core.Version{}, core.ReasonNotImplemented)
	degradeIdentityAttributed(&f)

	for name, got := range map[string]struct {
		status  core.Status
		reasons []core.Reason
	}{
		"ownership": {f.Ownership.Status, f.Ownership.Reasons},
		"worktype":  {f.Worktype.Status, f.Worktype.Reasons},
	} {
		if got.status != core.StatusDegraded || len(got.reasons) != 1 || got.reasons[0] != core.ReasonUnresolvedIdentity {
			t.Errorf("%s = %s %v, want degraded with unresolved_identity", name, got.status, got.reasons)
		}
	}
	if f.Ownership.Confidence != core.ConfidenceLow {
		t.Errorf("ownership confidence = %q, want low", f.Ownership.Confidence)
	}
	if f.Temporal.Status != core.StatusOK {
		t.Errorf("temporal attributes nothing to identities, but became %s", f.Temporal.Status)
	}
	if f.AIArchaeology.Status != core.StatusSkipped {
		t.Errorf("a skipped family became %s", f.AIArchaeology.Status)
	}

	// The unresolved author is not dropped from the identities section.
	got := buildIdentities(in, commits)
	total := 0
	for _, e := range got {
		total += e.CommitCount
	}
	if total != len(commits) {
		t.Errorf("the identities section counts %d commits of %d", total, len(commits))
	}
}

func TestIdentitiesCarrySourceAddressesAndCandidates(t *testing.T) {
	t.Parallel()
	folded := authored("Ada", "ada@example.com", 2)
	folded.AuthorSourceEmail = "ada.lovelace@corp.example.com"
	commits := []model.Commit{
		authored("Ada", "ada@example.com", 1),
		folded,
		authored("Ada", "ada@elsewhere.example.org", 3),
		authored("Grace", "grace@example.com", 4),
	}
	for _, anonymise := range []bool{false, true} {
		cfg := config.Default()
		cfg.Anonymize = anonymise
		got := identitiesOf(t, cfg, append([]model.Commit(nil), commits...))
		byID := map[string]core.IdentityEntry{}
		for _, e := range got {
			byID[e.ID] = e
		}
		ada := byID[core.IdentityDigest("ada@example.com")]
		other := byID[core.IdentityDigest("ada@elsewhere.example.org")]
		grace := byID[core.IdentityDigest("grace@example.com")]
		if ada.SourceAddressCount != 2 || other.SourceAddressCount != 1 || grace.SourceAddressCount != 1 {
			t.Errorf("source address counts = %d, %d, %d; want 2, 1, 1",
				ada.SourceAddressCount, other.SourceAddressCount, grace.SourceAddressCount)
		}
		want := []core.MergeCandidate{{ID: other.ID, Signal: "display_name"}, {ID: other.ID, Signal: "local_part"}}
		if fmt.Sprint(ada.MergeCandidates) != fmt.Sprint(want) {
			t.Errorf("anonymised %v: Ada's candidates = %v, want %v", anonymise, ada.MergeCandidates, want)
		}
		if grace.MergeCandidates == nil || len(grace.MergeCandidates) != 0 {
			t.Errorf("Grace's candidates = %#v, want an empty list", grace.MergeCandidates)
		}
		if len(got) != 3 {
			t.Errorf("computing candidates changed the identities: %d entries, want 3", len(got))
		}
	}
}

func TestTheAggregateEntrySumsSourceAddressesAndHasNoCandidates(t *testing.T) {
	t.Parallel()
	var commits []model.Commit
	for i := 0; i < core.LimitIdentities+2; i++ {
		commits = append(commits, authored("Same Name", fmt.Sprintf("p%03d@example.com", i), i))
	}
	got := identitiesOf(t, config.Default(), commits)
	last := got[len(got)-1]
	if !last.Aggregate || last.SourceAddressCount != 2 || last.MergeCandidates != nil {
		t.Errorf("aggregate entry = %+v, want two source addresses and no candidate list", last)
	}
	for _, e := range got[:len(got)-1] {
		for _, c := range e.MergeCandidates {
			if c.ID == "" {
				t.Errorf("a candidate names no id: %+v", e)
			}
		}
		if len(e.MergeCandidates) != core.LimitIdentities-1 {
			t.Errorf("entry %s has %d candidates, want one for every other individual entry", e.ID, len(e.MergeCandidates))
			break
		}
	}
}
