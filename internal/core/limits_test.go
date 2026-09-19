package core

import (
	"fmt"
	"strings"
	"testing"
)

// Every limit of section 12 has exactly one configuration name, and the names
// carry the catalogue's values. The catalogue rows themselves are compared
// with docs/metrics.md by TestMetricCatalogueLimits.
func TestNamedLimitsCoverTheCatalogue(t *testing.T) {
	t.Parallel()
	values := LimitValues()
	if len(values) != len(NamedLimits()) {
		t.Errorf("%d names produce %d values, so two limits share a name", len(NamedLimits()), len(values))
	}

	// Section 12 has one row per limit, except the coupling graph's row, which
	// carries a node limit and an edge limit.
	counted := 0
	for _, row := range CardinalityLimits() {
		counted += len(strings.Split(row.Value, " / "))
	}
	if counted != len(NamedLimits()) {
		t.Errorf("the catalogue holds %d limits and %d are named", counted, len(NamedLimits()))
	}

	for _, l := range NamedLimits() {
		if l.Value <= 0 {
			t.Errorf("the limit %s is %d", l.Name, l.Value)
		}
		found := false
		for _, row := range CardinalityLimits() {
			for _, value := range strings.Split(row.Value, " / ") {
				if value == fmt.Sprint(l.Value) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("the limit %s = %d is not a value any catalogue row carries", l.Name, l.Value)
		}
	}
}

// A configuration may state the limits it ran under, and a report does, but no
// operator sets one: a supplied value is verified against the catalogue and
// refused when it disagrees (ADR-0053 clause 6, ADR-0062 clause 6).
func TestVerifyCardinalityLimits(t *testing.T) {
	t.Parallel()
	if err := VerifyCardinalityLimits(nil); err != nil {
		t.Errorf("supplying no limit is a mismatch: %v", err)
	}
	if err := VerifyCardinalityLimits(LimitValues()); err != nil {
		t.Errorf("the catalogue's own values were refused: %v", err)
	}
	if err := VerifyCardinalityLimits(map[string]int{"coupling_pairs": LimitCouplingPairs}); err != nil {
		t.Errorf("a subset of the catalogue was refused: %v", err)
	}

	for name, supplied := range map[string]map[string]int{
		"a value the catalogue does not carry":  {"coupling_pairs": LimitCouplingPairs + 1},
		"a limit the catalogue does not define": {"coupling_triples": 5},
	} {
		err := VerifyCardinalityLimits(supplied)
		if err == nil {
			t.Errorf("%s was accepted", name)
			continue
		}
		if !strings.Contains(err.Error(), "cardinality_limits.") || !strings.Contains(err.Error(), "section 12") {
			t.Errorf("%s: the refusal does not name the offending limit and the catalogue: %v", name, err)
		}
	}
}
