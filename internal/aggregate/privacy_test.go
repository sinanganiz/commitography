package aggregate

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/config"
)

// emailShaped is deliberately loose: the point is that nothing resembling an
// address survives, not that a specific one does not.
var emailShaped = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.]+`)

func reportWithAuthors() *Report {
	base := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	return &Report{
		SchemaVersion: SchemaVersion,
		PerAuthor: &PerAuthor{Authors: []AuthorSummary{
			{
				IdentityID:  "grace@example.com",
				DisplayName: "Grace Hopper",
				Emails:      []string{"grace@example.com", "grace.hopper@navy.example.org"},
				FirstCommit: base.AddDate(0, 1, 0),
				Commits:     20,
			},
			{
				IdentityID:  "ada@example.com",
				DisplayName: "Ada Lovelace",
				Emails:      []string{"ada@example.com"},
				FirstCommit: base,
				Commits:     30,
			},
		}},
		Social: SocialMetrics{
			KnowledgeConcentration: []KnowledgeShare{
				{Path: "src", LargestShare: 0.8, TopContributor: "Ada Lovelace"},
			},
		},
		Notables: Notables{FirstCommitSubject: strptr("feat: begin, by Ada Lovelace")},
		Messages: MessageMetrics{
			LongestSubject: &LongestSubject{Subject: "refactor: rename things, per Ada Lovelace"},
			TopWords:       []WordCount{{Word: "lovelace", Count: 3}},
		},
	}
}

func strptr(s string) *string { return &s }

func serialize(t *testing.T, r *Report) string {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDefaultsHashEmails(t *testing.T) {
	cfg := config.Default() // HashEmails true, Anonymize false
	r := reportWithAuthors()
	ApplyPrivacy(r, cfg)

	out := serialize(t, r)
	if match := emailShaped.FindString(out); match != "" {
		t.Errorf("report still contains an email-shaped string: %q", match)
	}

	for _, a := range r.PerAuthor.Authors {
		if len(a.IdentityID) != emailHashLength {
			t.Errorf("identityId %q is not a %d-character digest", a.IdentityID, emailHashLength)
		}
		for _, e := range a.Emails {
			if len(e) != emailHashLength {
				t.Errorf("email %q was not hashed", e)
			}
		}
	}

	// Names are not secret by default; only addresses are.
	if !strings.Contains(out, "Ada Lovelace") {
		t.Error("display names should survive when only hash_emails is set")
	}
}

func TestHashEmailIsStableAndCaseInsensitive(t *testing.T) {
	a := HashEmail("Ada@Example.COM")
	b := HashEmail("  ada@example.com  ")
	if a != b {
		t.Errorf("HashEmail is not normalizing: %q != %q", a, b)
	}
	if a == HashEmail("grace@example.com") {
		t.Error("different addresses collided")
	}
	if HashEmail("") != "" {
		t.Error("an empty address should hash to nothing")
	}
}

func TestAnonymizeReplacesNamesAndDropsEmails(t *testing.T) {
	cfg := config.Default()
	cfg.Anonymize = true
	r := reportWithAuthors()
	ApplyPrivacy(r, cfg)

	out := serialize(t, r)

	if strings.Contains(out, "Grace Hopper") {
		t.Error("a real contributor name survived --anonymize")
	}
	if emailShaped.MatchString(out) {
		t.Error("an email-shaped string survived --anonymize")
	}
	for _, a := range r.PerAuthor.Authors {
		if len(a.Emails) != 0 {
			t.Errorf("emails should be dropped entirely, got %v", a.Emails)
		}
	}
	if r.Social.KnowledgeConcentration[0].TopContributor != "" {
		t.Error("the leading contributor must not be named under --anonymize")
	}
}

func TestPseudonymsFollowArrivalOrder(t *testing.T) {
	cfg := config.Default()
	cfg.Anonymize = true
	r := reportWithAuthors() // Ada arrives first, Grace a month later
	ApplyPrivacy(r, cfg)

	byOriginalCommits := map[int]string{}
	for _, a := range r.PerAuthor.Authors {
		byOriginalCommits[a.Commits] = a.DisplayName
	}
	if byOriginalCommits[30] != "Contributor A" {
		t.Errorf("the earliest contributor is %q, want Contributor A", byOriginalCommits[30])
	}
	if byOriginalCommits[20] != "Contributor B" {
		t.Errorf("the second contributor is %q, want Contributor B", byOriginalCommits[20])
	}
}

func TestAnonymizeKeepsMessageContent(t *testing.T) {
	cfg := config.Default()
	cfg.Anonymize = true
	r := reportWithAuthors()
	ApplyPrivacy(r, cfg)

	// Commit message content is not identity data. Stated explicitly in the
	// spec so nobody over-redacts it later.
	if r.Notables.FirstCommitSubject == nil || *r.Notables.FirstCommitSubject == "" {
		t.Error("firstCommitSubject must survive --anonymize")
	}
	if r.Messages.LongestSubject == nil || r.Messages.LongestSubject.Subject == "" {
		t.Error("longestSubject must survive --anonymize")
	}
	if len(r.Messages.TopWords) == 0 {
		t.Error("topWords must survive --anonymize")
	}
}

func TestAlphabeticLabel(t *testing.T) {
	cases := map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"}
	for i, want := range cases {
		if got := alphabeticLabel(i); got != want {
			t.Errorf("alphabeticLabel(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestPrivacyWithoutPerAuthorSection(t *testing.T) {
	cfg := config.Default()
	cfg.Anonymize = true
	r := &Report{
		Social: SocialMetrics{
			KnowledgeConcentration: []KnowledgeShare{{Path: "src", TopContributor: "Ada Lovelace"}},
		},
	}
	ApplyPrivacy(r, cfg) // must not panic on a nil PerAuthor
	if r.Social.KnowledgeConcentration[0].TopContributor != "" {
		t.Error("the leading contributor must be cleared even without a per-author section")
	}
}
