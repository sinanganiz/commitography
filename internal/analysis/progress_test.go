package analysis

import "testing"

func TestEventEmitterProducesEstimatedProgress(t *testing.T) {
	var events []ProgressEvent
	emitter := eventEmitter{sink: func(event ProgressEvent) {
		events = append(events, event)
	}}

	emitter.emit(StageCollecting, "reading history")
	emitter.emitCount(StageCollecting, "5 of 10 commits", 5, 10)
	emitter.emit(StageFinalizing, "analysis complete")

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	for i, event := range events {
		if event.Sequence != uint64(i+1) {
			t.Errorf("event %d sequence = %d, want %d", i, event.Sequence, i+1)
		}
		if event.Fraction == nil {
			t.Errorf("event %d has no fraction", i)
		}
	}
	if events[1].Current != 5 || events[1].Total != 10 || !events[1].Estimated {
		t.Errorf("counted event = %+v, want 5/10 estimated", events[1])
	}
	if events[2].Fraction == nil || *events[2].Fraction != 1 || events[2].Estimated {
		t.Errorf("final event = %+v, want 1.0 and not estimated", events[2])
	}
}
