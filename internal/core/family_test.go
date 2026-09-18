package core

import (
	"encoding/json"
	"reflect"
	"testing"
)

// sampleMetrics stands in for a family's metric type: a scalar, a list and a
// nested value, each held so that absence and a measured zero differ.
type sampleMetrics struct {
	Count *int      `json:"count,omitzero"`
	Ratio *float64  `json:"ratio,omitzero"`
	List  []string  `json:"list,omitzero"`
	Day   *struct{} `json:"day,omitzero"`
}

func encode(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// TestSkippedFamilyIsEmptyNotZero is ADR-0032 clause 3: a skipped family's
// metrics are an empty object, while a computed family's measured zeros are
// written out, so a consumer can tell "not measured" from "measured as zero".
func TestSkippedFamilyIsEmptyNotZero(t *testing.T) {
	t.Parallel()
	skipped := encode(t, Skipped[sampleMetrics](Version{Major: 0}, ReasonNotImplemented))
	if got := skipped["metrics"]; !reflect.DeepEqual(got, map[string]any{}) {
		t.Errorf("skipped metrics = %v, want an empty object", got)
	}
	if got := skipped["reasons"]; !reflect.DeepEqual(got, []any{"not_implemented"}) {
		t.Errorf("skipped reasons = %v, want [not_implemented]", got)
	}
	if _, ok := skipped["confidence"]; ok {
		t.Error("a skipped family carries a confidence indicator")
	}

	zero, none := 0, 0.0
	measured := encode(t, Computed(Version{Major: 1}, sampleMetrics{Count: &zero, Ratio: &none, List: []string{}}))
	metrics, _ := measured["metrics"].(map[string]any)
	want := map[string]any{"count": 0.0, "ratio": 0.0, "list": []any{}}
	if !reflect.DeepEqual(metrics, want) {
		t.Errorf("measured zeros = %v, want %v", metrics, want)
	}
	if _, ok := measured["reasons"]; ok {
		t.Error("an ok family carries reasons")
	}
}

func TestDegradeKeepsEveryReasonAndTheLowerConfidence(t *testing.T) {
	t.Parallel()
	f := Computed(Version{Major: 1}, sampleMetrics{})
	f.Degrade(ReasonShallowClone, ConfidenceLow)
	f.Degrade(ReasonCardinalityLimit, ConfidencePartial)
	f.Degrade(ReasonCardinalityLimit, ConfidencePartial)

	if f.Status != StatusDegraded {
		t.Errorf("status = %s, want degraded", f.Status)
	}
	// Enumeration order, not arrival order: cardinality_limit is declared
	// before shallow_clone.
	if want := []Reason{ReasonCardinalityLimit, ReasonShallowClone}; !reflect.DeepEqual(f.Reasons, want) {
		t.Errorf("reasons = %v, want %v", f.Reasons, want)
	}
	if f.Confidence != ConfidenceLow {
		t.Errorf("confidence = %s, want low", f.Confidence)
	}

	s := Skipped[sampleMetrics](Version{}, ReasonNotImplemented)
	s.Degrade(ReasonShallowClone, ConfidenceLow)
	if s.Status != StatusSkipped || len(s.Reasons) != 1 || s.Confidence != "" {
		t.Errorf("degrading a skipped family changed it: %+v", s)
	}
}
