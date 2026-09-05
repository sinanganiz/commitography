package aggregate

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/sinanganiz/commitography/internal/config"
)

// emailHashLength is how much of the SHA-256 digest is kept. Sixteen hex
// characters is enough to keep identities distinct in any real repository
// while producing a value nobody can mail.
const emailHashLength = 16

// ApplyPrivacy rewrites identity-bearing fields according to configuration.
//
// It runs after every metric has been computed and before serialization, so no
// number depends on a transformed value. Publishing a dashboard should not
// publish a team's mailing list.
func ApplyPrivacy(r *Report, cfg config.Config) {
	if r.PerAuthor == nil {
		// Identity data only ever reaches the report through the per-author
		// section and the knowledge-concentration leader, so there is nothing
		// else to redact.
		if cfg.Anonymize {
			for i := range r.Social.KnowledgeConcentration {
				r.Social.KnowledgeConcentration[i].TopContributor = ""
			}
		}
		return
	}

	authors := r.PerAuthor.Authors

	// Pseudonyms are assigned in arrival order so they stay stable between runs
	// and carry no ranking.
	pseudonym := map[string]string{}
	if cfg.Anonymize {
		ordered := append([]AuthorSummary(nil), authors...)
		sort.Slice(ordered, func(i, j int) bool {
			if !ordered[i].FirstCommit.Equal(ordered[j].FirstCommit) {
				return ordered[i].FirstCommit.Before(ordered[j].FirstCommit)
			}
			return ordered[i].IdentityID < ordered[j].IdentityID
		})
		for i, a := range ordered {
			pseudonym[a.IdentityID] = "Contributor " + alphabeticLabel(i)
		}
	}

	for i := range authors {
		a := &authors[i]
		switch {
		case cfg.Anonymize:
			a.DisplayName = pseudonym[a.IdentityID]
			a.Emails = nil
			a.IdentityID = a.DisplayName
		case cfg.HashEmails:
			a.IdentityID = HashEmail(a.IdentityID)
			for j, email := range a.Emails {
				a.Emails[j] = HashEmail(email)
			}
		}
	}

	if cfg.Anonymize {
		for i := range r.Social.KnowledgeConcentration {
			r.Social.KnowledgeConcentration[i].TopContributor = ""
		}
	}

	// Commit message content is not identity data. The first commit's subject,
	// the longest subject and the word cloud all stay, even under --anonymize.
	// This is stated explicitly so nobody over-redacts them later.
}

// HashEmail returns a stable, non-reversible stand-in for an address.
func HashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:emailHashLength]
}

// alphabeticLabel converts a zero-based index into A, B, ... Z, AA, AB, ...
func alphabeticLabel(i int) string {
	label := ""
	for {
		label = string(rune('A'+i%26)) + label
		i = i/26 - 1
		if i < 0 {
			return label
		}
	}
}
