// The family envelope of the report document: every family's version, status,
// reason codes and confidence, around the metrics it owns (ADR-0024 clause 1,
// ADR-0031 clause 2, ADR-0032 clauses 1 to 4).
//
// A family's metrics are held as pointers and slices tagged omitzero, so a
// metric that was not measured is absent from the document rather than null or
// zero, and a skipped family's metrics serialise as an empty object
// (ADR-0032 clause 3). A measured zero is a non-nil pointer to zero, or an
// empty non-nil slice, and is written out.

package core

import "sort"

// Version is a major and a minor component. The document carries one for its
// structure and every family carries one for the meaning of its output
// (ADR-0031 clauses 1 and 2).
type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

// Status is a family's outcome (ADR-0032 clause 2).
type Status string

// The three statuses. There is no fourth.
const (
	// StatusOK is computed and reliable.
	StatusOK Status = "ok"
	// StatusSkipped is not computed. The family carries a reason and no
	// metrics.
	StatusSkipped Status = "skipped"
	// StatusDegraded is computed, with a reason and a confidence indicator.
	StatusDegraded Status = "degraded"
)

// Confidence is the indicator a degraded family carries (ADR-0032 clause 2).
type Confidence string

// The two confidence levels, ordered from least to most reliable.
const (
	// ConfidenceLow means the values present may themselves be wrong: the
	// history is incomplete, or a classification is a guess.
	ConfidenceLow Confidence = "low"
	// ConfidencePartial means the values present are correct but incomplete:
	// a list was cut at its cardinality limit, or a metric over an empty
	// population is absent.
	ConfidencePartial Confidence = "partial"
)

// Family is one family's section of the report. M is the family's metric
// type, whose fields are its namespace (ADR-0024 clause 4).
type Family[M any] struct {
	Version Version `json:"version"`
	Status  Status  `json:"status"`
	// Reasons is empty for an ok family and holds at least one family status
	// code otherwise (ADR-0032 clause 4). It lists every condition that
	// applies, in enumeration order, so one does not hide another.
	Reasons []Reason `json:"reasons,omitzero"`
	// Confidence is set on a degraded family only.
	Confidence Confidence `json:"confidence,omitzero"`
	// Method states how the family's values were derived where that differs
	// from a well-known reference or from a measurement (ADR-0032 clause 8).
	Method  string `json:"method,omitzero"`
	Metrics M      `json:"metrics"`
}

// Computed returns an ok family holding the given metrics.
func Computed[M any](version Version, metrics M) Family[M] {
	return Family[M]{Version: version, Status: StatusOK, Metrics: metrics}
}

// Skipped returns a skipped family, for the reason given and any others that
// also apply, listed in enumeration order. Its metrics are the zero value,
// which serialises as an empty object.
func Skipped[M any](version Version, reason Reason, more ...Reason) Family[M] {
	reasons := []Reason{reason}
	for _, r := range more {
		reasons = withReason(reasons, r)
	}
	return Family[M]{Version: version, Status: StatusSkipped, Reasons: reasons}
}

// Degrade marks a computed family degraded for reason. The confidence it ends
// with is the lower of the one it had and the one given. A skipped family
// stays skipped: it was not computed, so there is nothing to degrade.
func (f *Family[M]) Degrade(reason Reason, confidence Confidence) {
	if f.Status == StatusSkipped {
		return
	}
	f.Status = StatusDegraded
	f.Reasons = withReason(f.Reasons, reason)
	if f.Confidence == "" || confidenceRank(confidence) < confidenceRank(f.Confidence) {
		f.Confidence = confidence
	}
}

// withReason adds reason to reasons once, keeping enumeration order so the
// document does not depend on the order conditions were found in.
func withReason(reasons []Reason, reason Reason) []Reason {
	for _, r := range reasons {
		if r == reason {
			return reasons
		}
	}
	out := append(append([]Reason(nil), reasons...), reason)
	order := map[Reason]int{}
	for i, r := range Reasons() {
		order[r] = i
	}
	sort.Slice(out, func(i, j int) bool { return order[out[i]] < order[out[j]] })
	return out
}

func confidenceRank(c Confidence) int {
	if c == ConfidenceLow {
		return 0
	}
	return 1
}
