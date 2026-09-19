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
	// carry. It is empty for an identity whose commits carry no address, and
	// for one the configuration named by digest alone.
	canonical Address
	// rests records that the identity rests on an address, whether or not this
	// layer ever saw that address in raw form.
	rests bool
	// references is every address digest that resolves to this identity: the
	// digest of each address below, and each digest a configuration gave in
	// place of an address (ADR-0068 clause 2). Sorted.
	references []string
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
func (id Identity) Resolved() bool { return id.rests }

// Format prints the digest and the display name under every verb, never the
// addresses, so an identity placed in a log line shows no address.
func (id Identity) Format(f fmt.State, _ rune) {
	_, _ = io.WriteString(f, "{"+id.Digest+" "+id.DisplayName+"}")
}

// Resolver maps raw commit authorship onto canonical identities.
type Resolver struct {
	// byAddress maps a normalised address to its identity's digest.
	byAddress map[Address]string
	// byReference maps an address digest to its identity's digest, which is
	// how a configuration that names an address by digest resolves (ADR-0068
	// clause 3).
	byReference map[string]string
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
		byAddress:   make(map[Address]string),
		byReference: make(map[string]string),
		byDigest:    make(map[string]*Identity),
		counts:      make(map[string]int),
	}

	// Configured identities come first: the canonical address is the first in
	// the list, and every listed address resolves to it. Each may be written
	// as an address or as the digest a report shows in its place, and the two
	// resolve alike (ADR-0068 clause 3).
	for _, id := range cfg.Identities {
		if len(id.Emails) == 0 {
			continue
		}
		digest, canonical, ok := parseReference(id.Emails[0])
		if !ok {
			continue
		}
		entry, exists := r.byDigest[digest]
		if !exists {
			entry = &Identity{Digest: digest, DisplayName: id.Name, canonical: canonical, rests: true}
			r.byDigest[digest] = entry
		}
		if id.Name != "" {
			entry.DisplayName = id.Name
		}
		for _, email := range id.Emails {
			reference, address, ok := parseReference(email)
			if !ok {
				continue
			}
			r.byReference[reference] = digest
			entry.references = appendUniqueString(entry.references, reference)
			if !address.Empty() {
				r.byAddress[address] = digest
				entry.addresses = appendUnique(entry.addresses, address)
			}
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
			// A configuration that named this address by its digest resolves
			// the commit exactly as one that named the address would.
			if configured, ok := r.byReference[digest]; ok {
				digest = configured
			}
			r.byAddress[address] = digest
		}
		entry, ok := r.byDigest[digest]
		if !ok {
			entry = &Identity{Digest: digest, canonical: address, rests: !address.Empty()}
			r.byDigest[digest] = entry
		}
		entry.addAddress(address)
		source := ParseAddress(c.AuthorSourceEmail)
		if source.Empty() {
			// A record that predates the source address, or a commit
			// .mailmap did not touch, has the one address.
			source = address
		}
		entry.addAddress(source)
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
		sort.Strings(entry.references)
		entry.IsBot = isBot(cfg, entry)
	}

	return r
}

// addAddress records an address and the digest that refers to it.
func (id *Identity) addAddress(address Address) {
	if address.Empty() {
		return
	}
	id.addresses = appendUnique(id.addresses, address)
	id.references = appendUniqueString(id.references, address.Digest())
}

// Resolve returns the digest of the identity a commit's author resolves to.
func (r *Resolver) Resolve(_, email string) string {
	address := ParseAddress(email)
	if digest, ok := r.byAddress[address]; ok {
		return digest
	}
	digest := address.Digest()
	if configured, ok := r.byReference[digest]; ok {
		return configured
	}
	return digest
}

// MatchedBy returns the digest of every identity an exclusion entry matches,
// in digest order. Anonymised output uses it to write an entry that names a
// person as references to the identities it named (ADR-0068 clause 4).
func (r *Resolver) MatchedBy(entry string) []string {
	var out []string
	for digest, id := range r.byDigest {
		if matchesEntry(entry, id) {
			out = append(out, digest)
		}
	}
	sort.Strings(out)
	return out
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
		if matchesEntry(excluded, id) {
			return true
		}
	}
	for _, address := range id.addresses {
		if config.IsBotIdentity(id.DisplayName, address.value) {
			return true
		}
	}
	return config.IsBotIdentity(id.DisplayName, "")
}

// matchesEntry reports whether an exclusion entry names an identity. What the
// entry is decides what it is compared with:
//
//   - a reference names one identity: its digest, a digest that resolves to
//     it, or the pseudonym anonymised output gives it, which is that same
//     digest (ADR-0068 clauses 2 and 4);
//   - an entry containing @ is an address, or is written as its digest, and
//     matches addresses only. A display name that merely looks like an address
//     is not one, and an address cannot be compared with a name once it is a
//     digest;
//   - anything else is a name or a class pattern, compared with the display
//     name and, for the address forms git allows that carry no @, with the
//     addresses themselves.
func matchesEntry(entry string, id *Identity) bool {
	entry = strings.ToLower(strings.TrimSpace(entry))
	if entry == "" {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(id.DisplayName))
	switch {
	case IsReference(entry):
		return id.Digest == entry || containsString(id.references, entry) || name == entry
	case strings.Contains(entry, "@"):
		return containsString(id.references, Digest(entry))
	default:
		if name == entry {
			return true
		}
		for _, address := range id.addresses {
			if address.value == entry {
				return true
			}
		}
		return false
	}
}

// parseReference reads a configured value that may be an address or the digest
// a report shows in its place (ADR-0068 clause 3). It returns the digest that
// refers to it, the address itself where one was given, and whether the value
// names anything at all.
func parseReference(value string) (reference string, address Address, ok bool) {
	if trimmed := strings.ToLower(strings.TrimSpace(value)); IsReference(trimmed) {
		return trimmed, Address{}, true
	}
	address = ParseAddress(value)
	if address.Empty() {
		return "", Address{}, false
	}
	return address.Digest(), address, true
}

// Reference returns the form the embedded configuration carries for a value
// that names a person by address: the address's digest, or the digest itself
// where one was given. It returns the empty string for a value that names
// nothing.
func Reference(value string) string {
	reference, _, ok := parseReference(value)
	if !ok {
		return ""
	}
	return reference
}

func sortAddresses(list []Address) {
	sort.Slice(list, func(i, j int) bool { return list[i].value < list[j].value })
}

func appendUniqueString(list []string, value string) []string {
	if value == "" || containsString(list, value) {
		return list
	}
	return append(list, value)
}

func containsString(list []string, value string) bool {
	for _, existing := range list {
		if existing == value {
			return true
		}
	}
	return false
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
