// Package identity collapses the several name/email pairs a single person
// commits under into one canonical contributor. Unresolved identities silently
// corrupt every aggregate, so this runs before anything is counted.
//
// This is the internal working layer of ADR-0033 clause 1: it holds raw
// addresses, and nothing it hands out carries one. An identity is known outside
// this package by its stable digest alone (clause 4); the addresses behind it
// are held as Address values in unexported fields, which have no marshalling
// path (clause 3). Resolution applies .mailmap first, which git does while the
// history is read, and configuration second, here, before anything is
// aggregated.
package identity

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// Identity is one resolved contributor.
type Identity struct {
	// Digest is the stable digest of the identity's canonical address, and the
	// identity's key everywhere outside this package (ADR-0033 clause 4).
	Digest      string
	DisplayName string
	IsBot       bool

	// canonical is the address the identity is keyed by: the first configured
	// address of a configured identity, and otherwise the one its commits
	// carry. It is empty for an identity whose commits carry no address.
	canonical Address
	// addresses is every address that resolves to this identity: configured
	// ones and observed ones, sorted.
	addresses []Address
	// sources is every address the identity's commits record before .mailmap,
	// sorted: the source addresses folded into it.
	sources []Address
}

// SourceAddresses returns how many distinct addresses, as the commits record
// them before .mailmap and configuration, are folded into the identity
// (docs/metrics.md section 14). It is a count; the addresses stay here.
func (id Identity) SourceAddresses() int { return len(id.sources) }

// Resolved reports whether the identity rests on an address. An identity
// whose commits carry no address cannot be told apart from any other such
// author, so it is kept as one entry but is not a resolved identity
// (ADR-0032; docs/metrics.md section 13, unresolved_identity).
func (id Identity) Resolved() bool { return !id.canonical.Empty() }

// Format prints the digest and the display name under every verb, never the
// addresses, so an identity placed in a log line shows no address.
func (id Identity) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, "{"+id.Digest+" "+id.DisplayName+"}")
}

// Resolver maps raw commit authorship onto canonical identities.
type Resolver struct {
	// byAddress maps a normalised address to its identity's digest.
	byAddress map[Address]string
	// byDigest holds the resolved identity for each digest.
	byDigest map[string]*Identity
	// counts is the number of commits observed per digest.
	counts map[string]int
}

// Format prints the number of identities under every verb. Without it, fmt
// would print the resolver's maps, whose keys are addresses reached through
// unexported fields, where fmt does not consult Address's own Format.
func (r Resolver) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprintf(f, "{%d identities}", len(r.byDigest))
}

// NewResolver builds a resolver from config and the observed commit set.
//
// Git's --use-mailmap has already been applied at collection time when
// UseMailmap is set, so the resolver only has to handle what mailmap could not:
// addresses the user has grouped explicitly, plus everything left over.
func NewResolver(cfg config.Analysis, commits []model.Commit) *Resolver {
	r := &Resolver{
		byAddress: make(map[Address]string),
		byDigest:  make(map[string]*Identity),
		counts:    make(map[string]int),
	}

	// Configured identities come first: the canonical address is the first in
	// the list, and every listed address resolves to it.
	for _, id := range cfg.Identities {
		if len(id.Emails) == 0 {
			continue
		}
		canonical := ParseAddress(id.Emails[0])
		if canonical.Empty() {
			continue
		}
		digest := canonical.Digest()
		entry, ok := r.byDigest[digest]
		if !ok {
			entry = &Identity{Digest: digest, DisplayName: id.Name, canonical: canonical}
			r.byDigest[digest] = entry
		}
		if id.Name != "" {
			entry.DisplayName = id.Name
		}
		for _, email := range id.Emails {
			address := ParseAddress(email)
			if address.Empty() {
				continue
			}
			r.byAddress[address] = digest
			entry.addresses = appendUnique(entry.addresses, address)
		}
	}

	configured := make(map[string]bool, len(r.byDigest))
	for digest := range r.byDigest {
		configured[digest] = true
	}

	// Everything else is keyed by its own address. The display name of an
	// unconfigured identity is the name on its most recent commit, which is
	// the one most likely to be current.
	latest := make(map[string]time.Time)
	for i := range commits {
		c := &commits[i]
		address := ParseAddress(c.AuthorEmail)
		digest, ok := r.byAddress[address]
		if !ok {
			digest = address.Digest()
			r.byAddress[address] = digest
		}
		entry, ok := r.byDigest[digest]
		if !ok {
			entry = &Identity{Digest: digest, canonical: address}
			r.byDigest[digest] = entry
		}
		entry.addresses = appendUnique(entry.addresses, address)
		source := ParseAddress(c.AuthorSourceEmail)
		if source.Empty() {
			// A record that predates the source address, or a commit
			// .mailmap did not touch, has the one address.
			source = address
		}
		entry.addresses = appendUnique(entry.addresses, source)
		entry.sources = appendUnique(entry.sources, source)
		r.counts[digest]++

		if !configured[digest] {
			if when, seen := latest[digest]; !seen || c.AuthorDate.After(when) {
				latest[digest] = c.AuthorDate
				entry.DisplayName = c.AuthorName
			}
		}
	}

	for _, entry := range r.byDigest {
		sortAddresses(entry.addresses)
		sortAddresses(entry.sources)
		entry.IsBot = isBot(cfg, entry)
	}

	return r
}

// Resolve returns the digest of the identity a commit's author resolves to.
func (r *Resolver) Resolve(_, email string) string {
	address := ParseAddress(email)
	if digest, ok := r.byAddress[address]; ok {
		return digest
	}
	return address.Digest()
}

// Lookup returns the resolved identity for a digest.
func (r *Resolver) Lookup(digest string) (Identity, bool) {
	entry, ok := r.byDigest[digest]
	if !ok {
		return Identity{}, false
	}
	return *entry, true
}

// Commits returns the number of commits observed for a digest.
func (r *Resolver) Commits(digest string) int { return r.counts[digest] }

// Identities returns all resolved identities, sorted by descending commit
// count. Ties break on digest so the order is stable across runs.
func (r *Resolver) Identities() []Identity {
	out := make([]Identity, 0, len(r.byDigest))
	for _, entry := range r.byDigest {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		ci, cj := r.counts[out[i].Digest], r.counts[out[j].Digest]
		if ci != cj {
			return ci > cj
		}
		return out[i].Digest < out[j].Digest
	})
	return out
}

// isBot reports whether an identity is an automation account, either by
// appearing in the configured exclusion list or by matching the built-in
// patterns for host-generated bot accounts.
func isBot(cfg config.Analysis, id *Identity) bool {
	for _, excluded := range cfg.ExcludeAuthors {
		excluded = strings.ToLower(strings.TrimSpace(excluded))
		if excluded == "" {
			continue
		}
		if strings.ToLower(id.DisplayName) == excluded {
			return true
		}
		for _, address := range id.addresses {
			if address.value == excluded {
				return true
			}
		}
	}
	for _, address := range id.addresses {
		if config.IsBotIdentity(id.DisplayName, address.value) {
			return true
		}
	}
	return config.IsBotIdentity(id.DisplayName, "")
}

func sortAddresses(list []Address) {
	sort.Slice(list, func(i, j int) bool { return list[i].value < list[j].value })
}

func appendUnique(list []Address, value Address) []Address {
	if value.Empty() {
		return list
	}
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
