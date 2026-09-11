package jobs

import (
	"errors"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/analysis"
)

func TestManagerAllowsOneActiveJobAndRetainsTerminalHistory(t *testing.T) {
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	manager := New(Options{
		Limit: 2,
		Now: func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		},
		NewID: func() (string, error) {
			return "job-" + clock.Format("150405"), nil
		},
	})

	first, err := manager.Create("/repos/first")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != StatusQueued || first.RepoName != "first" {
		t.Fatalf("first = %+v", first)
	}
	if _, err := manager.Create("/repos/second"); !errors.Is(err, ErrActiveJob) {
		t.Fatalf("second create error = %v, want ErrActiveJob", err)
	}
	if err := manager.MarkRunning(first.ID, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateProgress(first.ID, analysis.ProgressEvent{Sequence: 1, Stage: analysis.StageCollecting}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Complete(first.ID, &analysis.Result{}, time.Time{}); err != nil {
		t.Fatal(err)
	}

	second, err := manager.Create("/repos/second")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Fail(second.ID, Failure{Code: "test", Message: "failed"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if got := len(manager.List()); got != 2 {
		t.Fatalf("history length = %d, want 2", got)
	}
}

func TestManagerEvictsOldestTerminalJob(t *testing.T) {
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	next := 0
	manager := New(Options{
		Limit: 2,
		Now: func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		},
		NewID: func() (string, error) {
			next++
			return string(rune('a' + next - 1)), nil
		},
	})

	for _, path := range []string{"/repos/one", "/repos/two", "/repos/three"} {
		job, err := manager.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.Fail(job.ID, Failure{Code: "test"}, time.Time{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := manager.Get("a"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("oldest job error = %v, want ErrJobNotFound", err)
	}
	if got := len(manager.List()); got != 2 {
		t.Fatalf("history length = %d, want 2", got)
	}
}
