package identity

import (
	"reflect"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
)

func commitBy(name, email string) model.Commit {
	return model.Commit{AuthorName: name, AuthorEmail: email, AuthorDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

// resolved returns the commits with their identities resolved, as the filter
// stage leaves them before aggregation reads them.
func resolved(cfg config.Analysis, commits []model.Commit) (*Resolver, []model.Commit) {
	r := NewResolver(cfg, commits)
	out := append([]model.Commit(nil), commits...)
	for i := range out {
		out[i].IdentityID = r.Resolve(out[i].AuthorName, out[i].AuthorEmail)
	}
	return r, out
}

func candidatesOf(t *testing.T, commits ...model.Commit) (*Resolver, map[string][]Candidate) {
	t.Helper()
	r, analysed := resolved(config.Default(), commits)
	var digests []string
	for _, id := range r.Identities() {
		digests = append(digests, id.Digest)
	}
	return r, Candidates(digests, analysed)
}

func TestCandidatesComeFromEachSignal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		a, b   model.Commit
		signal Signal
	}{
		{"an identical name after normalisation",
			commitBy("Ada  Lovelace", "ada@example.com"), commitBy("ada lovelace", "a.l@corp.example.com"), SignalDisplayName},
		{"an identical local part",
			commitBy("Ada", "ada@example.com"), commitBy("A. Lovelace", "ADA@corp.example.com"), SignalLocalPart},
		{"a GitHub noreply account matching a local part",
			commitBy("Ada", "ada@example.com"), commitBy("A. L.", "12345+ada@users.noreply.github.com"), SignalNoreply},
		{"a GitLab noreply account matching a compact name",
			commitBy("Ada Lovelace", "al@example.com"), commitBy("A", "42-adalovelace@users.noreply.gitlab.com"), SignalNoreply},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, got := candidatesOf(t, tc.a, tc.b)
			a, b := r.Resolve("", tc.a.AuthorEmail), r.Resolve("", tc.b.AuthorEmail)
			if want := []Candidate{{Digest: b, Signal: tc.signal}}; !reflect.DeepEqual(got[a], want) {
				t.Errorf("candidates of a = %v, want %v", got[a], want)
			}
			if want := []Candidate{{Digest: a, Signal: tc.signal}}; !reflect.DeepEqual(got[b], want) {
				t.Errorf("candidates of b = %v, want %v", got[b], want)
			}
		})
	}
}

func TestCandidatesAreSuggestionsOnly(t *testing.T) {
	t.Parallel()
	r, got := candidatesOf(t,
		commitBy("Ada", "ada@example.com"),
		commitBy("Ada", "ada@corp.example.com"),
		commitBy("Grace", "grace@example.com"))
	if len(r.Identities()) != 3 {
		t.Errorf("computing candidates merged identities: %d remain of 3", len(r.Identities()))
	}
	a := r.Resolve("", "ada@example.com")
	want := []Candidate{
		{Digest: r.Resolve("", "ada@corp.example.com"), Signal: SignalDisplayName},
		{Digest: r.Resolve("", "ada@corp.example.com"), Signal: SignalLocalPart},
	}
	if !reflect.DeepEqual(got[a], want) {
		t.Errorf("candidates = %v, want %v", got[a], want)
	}
	if grace := got[r.Resolve("", "grace@example.com")]; grace == nil || len(grace) != 0 {
		t.Errorf("an unconnected identity has candidates %v, want an empty list", grace)
	}
}

func TestCandidatesStayWithinTheGivenSet(t *testing.T) {
	t.Parallel()
	r, analysed := resolved(config.Default(), []model.Commit{
		commitBy("Ada", "ada@example.com"), commitBy("Ada", "ada@corp.example.com"),
	})
	only := r.Resolve("", "ada@example.com")
	got := Candidates([]string{only}, analysed)
	if len(got) != 1 || len(got[only]) != 0 {
		t.Errorf("candidates = %v, want none outside the given set", got)
	}
}

// ADR-0069 clause 2: a value that exists only because it was configured is not
// evidence about this repository, so it suggests nothing. Before the record,
// the configured address below produced a local_part candidate that no commit
// supported, and that no rerun from the report's embedded configuration —
// where addresses are digests — could reproduce.
func TestConfiguredValuesProduceNoCandidate(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		commitBy("Ada", "ada@a.example.com"),
		commitBy("L", "lovelace@b.example.com"),
	}
	cfg := config.Default()
	cfg.Identities = []config.Identity{{
		Name:   "Ada King",
		Emails: []string{"ada@a.example.com", "lovelace@corp.example.com"},
	}}
	r, analysed := resolved(cfg, commits)
	digests := []string{r.Resolve("", "ada@a.example.com"), r.Resolve("", "lovelace@b.example.com")}
	got := Candidates(digests, analysed)
	for _, digest := range digests {
		if len(got[digest]) != 0 {
			t.Errorf("the configured address produced the candidates %v", got[digest])
		}
	}

	// The configured name is not evidence either: the identity's recorded name
	// is what a signal compares.
	if same := Candidates(digests, analysed); len(same[digests[0]]) != 0 {
		t.Errorf("the configured name produced the candidates %v", same[digests[0]])
	}
}

// Every name a commit records is evidence, not only the most recent, and the
// same holds for the addresses before and after .mailmap.
func TestEveryRecordedNameAndAddressIsEvidence(t *testing.T) {
	t.Parallel()
	renamed := commitBy("Ada Lovelace", "ada@example.com")
	renamed.AuthorDate = renamed.AuthorDate.AddDate(1, 0, 0)
	folded := commitBy("Grace", "grace@example.com")
	folded.AuthorSourceEmail = "ada.lovelace@old.example.com"

	r, got := candidatesOf(t,
		commitBy("Ada", "ada@example.com"),
		renamed,
		commitBy("Ada Lovelace", "someone@corp.example.com"),
		folded)

	ada := r.Resolve("", "ada@example.com")
	other := r.Resolve("", "someone@corp.example.com")
	if want := []Candidate{{Digest: other, Signal: SignalDisplayName}}; !reflect.DeepEqual(got[ada], want) {
		t.Errorf("candidates of the renamed identity = %v, want %v", got[ada], want)
	}
	// Grace's pre-mailmap address shares its local part with nobody, but the
	// address it folded into does not hide it either.
	grace := r.Resolve("", "grace@example.com")
	if len(got[grace]) != 0 {
		t.Errorf("candidates of the folded identity = %v, want none", got[grace])
	}
}

func TestSourceAddressesCountWhatMailmapFolded(t *testing.T) {
	t.Parallel()
	commits := []model.Commit{
		{AuthorName: "Ada", AuthorEmail: "ada@example.com", AuthorSourceEmail: "ada@example.com"},
		{AuthorName: "Ada", AuthorEmail: "ada@example.com", AuthorSourceEmail: "ada.lovelace@corp.example.com"},
		{AuthorName: "Grace", AuthorEmail: "grace@example.com"},
	}
	cfg := config.Default()
	cfg.Identities = []config.Identity{{Name: "Grace", Emails: []string{"grace@example.com", "unused@example.com"}}}
	r := NewResolver(cfg, commits)
	ada, _ := r.Lookup(Digest("ada@example.com"))
	grace, _ := r.Lookup(Digest("grace@example.com"))
	if ada.SourceAddresses() != 2 || grace.SourceAddresses() != 1 {
		t.Errorf("source addresses = %d and %d, want 2 and 1; a configured address no commit records is not a source",
			ada.SourceAddresses(), grace.SourceAddresses())
	}
}
