package aggregate

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

const dateLayout = "2006-01-02"

// addressPattern is the shape of an email address. A resolved name that
// contains one is not written to the report: the resolver falls back to the
// address when a commit carries no name, and a name is attacker-controlled
// (ADR-0045), so either could put an address list into the artifact.
func addressPattern() *regexp.Regexp {
	return regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
}

// identityTally is one identity's analysed commits, accumulated.
type identityTally struct {
	entry       core.IdentityEntry
	first, last time.Time
}

// buildIdentities returns the identities section, docs/metrics.md section 14:
// every identity with an analysed commit, the most active up to the identity
// limit individually and the rest folded into one aggregate entry, ordered by
// first commit date. It is never nil, so an analysis with no analysed commit
// writes an empty list rather than null.
func buildIdentities(in core.Input, analyzed []model.Commit) []core.IdentityEntry {
	address := addressPattern()
	tallies := map[string]*identityTally{}
	for _, c := range analyzed {
		when := in.Date(c)
		tally, ok := tallies[c.IdentityID]
		if !ok {
			tally = &identityTally{
				entry: core.IdentityEntry{
					ID:          c.IdentityID,
					DisplayName: displayName(in, c.IdentityID, address),
				},
				first: when,
				last:  when,
			}
			tallies[c.IdentityID] = tally
		}
		tally.entry.CommitCount++
		if tally.entry.CommitCount == 1 && in.Resolver != nil {
			if resolved, ok := in.Resolver.Lookup(c.IdentityID); ok {
				tally.entry.SourceAddressCount = resolved.SourceAddresses()
			}
		}
		if when.Before(tally.first) {
			tally.first = when
		}
		if when.After(tally.last) {
			tally.last = when
		}
	}

	all := make([]core.IdentityEntry, 0, len(tallies))
	for _, tally := range tallies {
		tally.entry.FirstCommitDate = tally.first.Format(dateLayout)
		tally.entry.LastCommitDate = tally.last.Format(dateLayout)
		all = append(all, tally.entry)
	}

	// The bound selects by activity; the order the reader sees does not
	// (ADR-0009 clause 3).
	sort.Slice(all, func(i, j int) bool {
		a, b := all[i], all[j]
		if a.CommitCount != b.CommitCount {
			return a.CommitCount > b.CommitCount
		}
		if a.FirstCommitDate != b.FirstCommitDate {
			return a.FirstCommitDate < b.FirstCommitDate
		}
		return a.ID < b.ID
	})
	shown, folded := all, []core.IdentityEntry(nil)
	if len(all) > core.LimitIdentities {
		shown, folded = all[:core.LimitIdentities], all[core.LimitIdentities:]
	}

	sort.Slice(shown, func(i, j int) bool {
		a, b := shown[i], shown[j]
		if a.FirstCommitDate != b.FirstCommitDate {
			return a.FirstCommitDate < b.FirstCommitDate
		}
		return a.ID < b.ID
	})
	withCandidates(shown, analyzed)
	if len(folded) == 0 {
		return shown
	}
	return append(shown, aggregateEntry(folded))
}

// withCandidates gives every individual entry its merge candidates, computed
// in the identity layer, where the raw data they compare is (ADR-0033
// clause 1). Candidates are sought among the individual entries only, so every
// id a candidate names is an entry the reader can see.
//
// The evidence is the analysed commits (ADR-0069 clause 1), which is why they
// are passed here rather than read from the resolver: the resolver also holds
// what the configuration supplied, and a value that only a configuration
// carries is not evidence about this repository.
func withCandidates(shown []core.IdentityEntry, analyzed []model.Commit) {
	ids := make([]string, len(shown))
	for i, e := range shown {
		ids[i] = e.ID
	}
	found := identity.Candidates(ids, analyzed)
	for i := range shown {
		shown[i].MergeCandidates = []core.MergeCandidate{}
		for _, c := range found[shown[i].ID] {
			shown[i].MergeCandidates = append(shown[i].MergeCandidates,
				core.MergeCandidate{ID: c.Digest, Signal: string(c.Signal)})
		}
	}
}

// aggregateEntry folds identities into the one entry that stands for all of
// them.
func aggregateEntry(folded []core.IdentityEntry) core.IdentityEntry {
	out := core.IdentityEntry{
		DisplayName:     fmt.Sprintf("%d other identities", len(folded)),
		FirstCommitDate: folded[0].FirstCommitDate,
		LastCommitDate:  folded[0].LastCommitDate,
		Aggregate:       true,
	}
	for _, e := range folded {
		out.CommitCount += e.CommitCount
		out.SourceAddressCount += e.SourceAddressCount
		if e.FirstCommitDate < out.FirstCommitDate {
			out.FirstCommitDate = e.FirstCommitDate
		}
		if e.LastCommitDate > out.LastCommitDate {
			out.LastCommitDate = e.LastCommitDate
		}
	}
	return out
}

// displayName returns the name the report may carry for an identity: its
// resolved name, or its digest where the name is empty or contains an address,
// and where anonymised output was requested. The digest is the identity's
// stable pseudonym (ADR-0033 clause 5): the same in every run and every
// repository, and derived from nothing a reader can reverse.
func displayName(in core.Input, id string, address *regexp.Regexp) string {
	if in.Config.Anonymize || in.Resolver == nil {
		return id
	}
	resolved, ok := in.Resolver.Lookup(id)
	if !ok || resolved.DisplayName == "" || address.MatchString(resolved.DisplayName) {
		return id
	}
	return resolved.DisplayName
}

// unresolvedAuthor reports whether any analysed commit's author could not be
// resolved into an identity: its commit carries no address, so neither
// .mailmap nor configuration can place it, and it cannot be told apart from
// any other such author.
func unresolvedAuthor(in core.Input, analyzed []model.Commit) bool {
	if in.Resolver == nil {
		return false
	}
	for _, c := range analyzed {
		if resolved, ok := in.Resolver.Lookup(c.IdentityID); !ok || !resolved.Resolved() {
			return true
		}
	}
	return false
}

// degradeIdentityAttributed marks degraded, with reason unresolved_identity,
// every family whose values are attributed to identities: ownership's lines by
// identity and bus factor, worktype's editor-by-owner breakdown, and
// ai-archaeology's identity ratio (docs/metrics.md sections 7 to 9). The
// commits of an unresolved author are counted, so the values present may be
// attributed wrongly, which is low confidence (ADR-0032 clause 2). A family
// that is skipped stays skipped: it computed nothing to distrust.
func degradeIdentityAttributed(f *core.Families) {
	f.Ownership.Degrade(core.ReasonUnresolvedIdentity, core.ConfidenceLow)
	f.Worktype.Degrade(core.ReasonUnresolvedIdentity, core.ConfidenceLow)
	f.AIArchaeology.Degrade(core.ReasonUnresolvedIdentity, core.ConfidenceLow)
}
