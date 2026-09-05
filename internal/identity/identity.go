// Package identity collapses the several name/email pairs a single person
// commits under into one canonical contributor. Unresolved identities silently
// corrupt every aggregate, so this runs before anything is counted.
package identity

import (
	"sort"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/model"
)

// Identity is one resolved contributor.
type Identity struct {
	ID          string // canonical key
	DisplayName string
	Emails      []string
	IsBot       bool
}

// Resolver maps raw commit authorship onto canonical identities.
type Resolver struct {
	// emailToID maps a normalized email to its canonical identity ID.
	emailToID map[string]string
	// byID holds the resolved identity for each canonical ID.
	byID map[string]*Identity
	// counts is the number of commits observed per canonical ID.
	counts map[string]int
}

// NewResolver builds a resolver from config and the observed commit set.
//
// Git's --use-mailmap has already been applied at collection time when
// UseMailmap is set, so the resolver only has to handle what mailmap could not:
// addresses the user has grouped explicitly, plus everything left over.
func NewResolver(cfg config.Config, commits []model.Commit) *Resolver {
	r := &Resolver{
		emailToID: make(map[string]string),
		byID:      make(map[string]*Identity),
		counts:    make(map[string]int),
	}

	// Configured identities come first: the canonical ID is the first email in
	// the list, and every listed address resolves to it.
	for _, id := range cfg.Identities {
		if len(id.Emails) == 0 {
			continue
		}
		canonical := NormalizeEmail(id.Emails[0])
		if canonical == "" {
			continue
		}
		entry, ok := r.byID[canonical]
		if !ok {
			entry = &Identity{ID: canonical, DisplayName: id.Name}
			r.byID[canonical] = entry
		}
		if id.Name != "" {
			entry.DisplayName = id.Name
		}
		for _, email := range id.Emails {
			normalized := NormalizeEmail(email)
			if normalized == "" {
				continue
			}
			r.emailToID[normalized] = canonical
			entry.Emails = appendUnique(entry.Emails, normalized)
		}
	}

	configured := make(map[string]bool, len(r.byID))
	for id := range r.byID {
		configured[id] = true
	}

	// Everything else is keyed by its own address. The display name of an
	// unconfigured identity is the name on its most recent commit, which is
	// the one most likely to be current.
	latest := make(map[string]time.Time)
	for i := range commits {
		c := &commits[i]
		email := NormalizeEmail(c.AuthorEmail)
		id, ok := r.emailToID[email]
		if !ok {
			id = email
			r.emailToID[email] = id
		}
		entry, ok := r.byID[id]
		if !ok {
			entry = &Identity{ID: id}
			r.byID[id] = entry
		}
		entry.Emails = appendUnique(entry.Emails, email)
		r.counts[id]++

		if !configured[id] {
			if when, seen := latest[id]; !seen || c.AuthorDate.After(when) {
				latest[id] = c.AuthorDate
				entry.DisplayName = c.AuthorName
			}
		}
	}

	for _, entry := range r.byID {
		if entry.DisplayName == "" {
			entry.DisplayName = entry.ID
		}
		sort.Strings(entry.Emails)
		entry.IsBot = isBot(cfg, entry)
	}

	return r
}

// Resolve returns the canonical identity ID for a commit's author.
func (r *Resolver) Resolve(name, email string) string {
	normalized := NormalizeEmail(email)
	if id, ok := r.emailToID[normalized]; ok {
		return id
	}
	return normalized
}

// Lookup returns the resolved identity for a canonical ID.
func (r *Resolver) Lookup(id string) (Identity, bool) {
	entry, ok := r.byID[id]
	if !ok {
		return Identity{}, false
	}
	return *entry, true
}

// Commits returns the number of commits observed for a canonical ID.
func (r *Resolver) Commits(id string) int { return r.counts[id] }

// Identities returns all resolved identities, sorted by descending commit
// count. Ties break on ID so the order is stable across runs.
func (r *Resolver) Identities() []Identity {
	out := make([]Identity, 0, len(r.byID))
	for _, entry := range r.byID {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		ci, cj := r.counts[out[i].ID], r.counts[out[j].ID]
		if ci != cj {
			return ci > cj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// isBot reports whether an identity is an automation account, either by
// appearing in the configured exclusion list or by matching the built-in
// patterns for host-generated bot accounts.
func isBot(cfg config.Config, id *Identity) bool {
	for _, excluded := range cfg.ExcludeAuthors {
		excluded = strings.ToLower(strings.TrimSpace(excluded))
		if excluded == "" {
			continue
		}
		if strings.ToLower(id.DisplayName) == excluded {
			return true
		}
		for _, email := range id.Emails {
			if email == excluded {
				return true
			}
		}
	}
	for _, email := range id.Emails {
		if config.IsBotIdentity(id.DisplayName, email) {
			return true
		}
	}
	return config.IsBotIdentity(id.DisplayName, "")
}

// NormalizeEmail trims surrounding whitespace and lowercases an address so
// that casing differences never split one person into two contributors.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func appendUnique(list []string, value string) []string {
	if value == "" {
		return list
	}
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
