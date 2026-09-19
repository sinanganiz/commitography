package identity

import (
	"regexp"
	"sort"
	"strings"
)

// Signal names the evidence a merge candidate rests on. ADR-0010 clause 4
// requires at least these three.
type Signal string

const (
	// SignalDisplayName is an identical display name after normalisation.
	SignalDisplayName Signal = "display_name"
	// SignalLocalPart is an identical address local part.
	SignalLocalPart Signal = "local_part"
	// SignalNoreply is a hosting provider noreply address whose account name
	// matches the other identity's local part, account name or display name.
	SignalNoreply Signal = "noreply"
)

// Candidate is one suggestion that another identity may be the same person.
// It is a suggestion only: nothing in the product applies it, and a person
// decides (ADR-0010 clause 4). It names the other identity by digest, never by
// address.
type Candidate struct {
	Digest string
	Signal Signal
}

// noreplyPatterns match the noreply addresses hosting providers issue, and
// capture the account name inside them:
//
//	GitHub  [<id>+]<account>@users.noreply.github.com
//	GitLab  [<id>-]<account>@users.noreply.gitlab.com
func noreplyPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`^(?:[0-9]+\+)?([^@+]+)@users\.noreply\.github\.com$`),
		regexp.MustCompile(`^(?:[0-9]+-)?([^@]+)@users\.noreply\.gitlab\.com$`),
	}
}

// evidence is what the signals compare for one identity, derived from its raw
// data here in the internal layer (ADR-0033 clause 1).
type evidence struct {
	name       string          // normalised display name
	compact    string          // normalised display name without spaces
	localParts map[string]bool // local parts of non-noreply addresses
	accounts   map[string]bool // account names inside noreply addresses
}

// Candidates computes the merge candidates among the identities given by
// digest, for each of them: every other identity in the set that one of the
// three signals connects it to. The relation is symmetric; each identity's
// list is ordered by digest, then by signal, and names each pair and signal
// once. An identity with no candidate maps to an empty list.
//
// It computes suggestions and applies none. Nothing in the resolver changes.
func (r *Resolver) Candidates(digests []string) map[string][]Candidate {
	patterns := noreplyPatterns()
	facts := make(map[string]evidence, len(digests))
	for _, digest := range digests {
		if entry, ok := r.byDigest[digest]; ok {
			facts[digest] = evidenceOf(*entry, patterns)
		}
	}

	out := make(map[string][]Candidate, len(digests))
	for _, digest := range digests {
		out[digest] = []Candidate{}
	}
	for i, a := range digests {
		for _, b := range digests[i+1:] {
			if a == b {
				continue
			}
			for _, signal := range signals(facts[a], facts[b]) {
				out[a] = append(out[a], Candidate{Digest: b, Signal: signal})
				out[b] = append(out[b], Candidate{Digest: a, Signal: signal})
			}
		}
	}
	for _, list := range out {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Digest != list[j].Digest {
				return list[i].Digest < list[j].Digest
			}
			return list[i].Signal < list[j].Signal
		})
	}
	return out
}

// signals returns every signal connecting two identities, in a fixed order.
func signals(a, b evidence) []Signal {
	var out []Signal
	if a.name != "" && a.name == b.name {
		out = append(out, SignalDisplayName)
	}
	if intersects(a.localParts, b.localParts) {
		out = append(out, SignalLocalPart)
	}
	if noreplyMatches(a, b) || noreplyMatches(b, a) {
		out = append(out, SignalNoreply)
	}
	return out
}

// noreplyMatches reports whether an account name inside one identity's noreply
// address names the other identity: as its local part, as the account in its
// own noreply address, or as its display name written without spaces.
func noreplyMatches(a, b evidence) bool {
	for account := range a.accounts {
		if b.localParts[account] || b.accounts[account] || (b.compact != "" && b.compact == account) {
			return true
		}
	}
	return false
}

func evidenceOf(id Identity, patterns []*regexp.Regexp) evidence {
	name := normaliseName(id.DisplayName)
	e := evidence{
		name:       name,
		compact:    strings.ReplaceAll(name, " ", ""),
		localParts: map[string]bool{},
		accounts:   map[string]bool{},
	}
	for _, address := range id.addresses {
		if account := noreplyAccount(address, patterns); account != "" {
			e.accounts[account] = true
			continue
		}
		if local := address.localPart(); local != "" {
			e.localParts[local] = true
		}
	}
	return e
}

func noreplyAccount(address Address, patterns []*regexp.Regexp) string {
	for _, pattern := range patterns {
		if m := pattern.FindStringSubmatch(address.value); m != nil {
			return m[1]
		}
	}
	return ""
}

// normaliseName lowercases a display name and collapses its whitespace, so
// that casing and spacing never hide a match.
func normaliseName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

func intersects(a, b map[string]bool) bool {
	for key := range a {
		if b[key] {
			return true
		}
	}
	return false
}
