package jobs

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/aggregate"
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

func TestStartRunsWorkerAndCancellationReleasesSlot(t *testing.T) {
	started := make(chan struct{}, 1)
	manager := New(Options{
		Runner: func(ctx context.Context, _ analysis.Options, sink analysis.ProgressSink) (*analysis.Result, error) {
			sink(analysis.ProgressEvent{Sequence: 1, Stage: analysis.StageCollecting})
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})

	job, err := manager.Start("/repos/cancellable", analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	if err := manager.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		current, err := manager.Get(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status == StatusCancelled {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("job status = %s, want cancelled", current.Status)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := manager.Create("/repos/after-cancel"); err != nil {
		t.Fatalf("new job after cancellation: %v", err)
	}
}

func TestProgressSequenceIsMonotonicAndSnapshotsProjectElapsed(t *testing.T) {
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	manager := New(Options{
		Now: func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		},
		NewID: func() (string, error) { return "progress-job", nil },
	})
	job, err := manager.Create("/repos/progress")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.MarkRunning(job.ID, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateProgress(job.ID, analysis.ProgressEvent{Sequence: 2, Stage: analysis.StageCode}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateProgress(job.ID, analysis.ProgressEvent{Sequence: 1, Stage: analysis.StageCollecting}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("out-of-order progress error = %v, want ErrInvalidState", err)
	}
	snapshot, err := manager.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Progress == nil || snapshot.Progress.Sequence != 2 {
		t.Fatalf("progress = %+v, want sequence 2", snapshot.Progress)
	}
	if snapshot.Elapsed <= 0 {
		t.Fatalf("elapsed = %s, want positive duration", snapshot.Elapsed)
	}
}

func TestOnlySucceededJobsExposeReports(t *testing.T) {
	manager := New(Options{NewID: func() (string, error) { return "report-job", nil }})
	job, err := manager.Create("/repos/report")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Fail(job.ID, Failure{Code: "test"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Report(job.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("failed report error = %v, want ErrInvalidState", err)
	}

	manager = New(Options{NewID: func() (string, error) { return "success-job", nil }})
	job, err = manager.Create("/repos/report")
	if err != nil {
		t.Fatal(err)
	}
	result := &analysis.Result{Report: &aggregate.Report{}}
	if err := manager.Complete(job.ID, result, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if report, err := manager.Report(job.ID); err != nil || report == nil {
		t.Fatalf("successful report = %v, %v", report, err)
	}
}

func TestCancelAllRequestsWorkerCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	manager := New(Options{
		Runner: func(ctx context.Context, _ analysis.Options, _ analysis.ProgressSink) (*analysis.Result, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	job, err := manager.Start("/repos/shutdown", analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	manager.CancelAll()
	deadline := time.After(2 * time.Second)
	for {
		current, err := manager.Get(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status == StatusCancelled {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("job status = %s, want cancelled", current.Status)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestWarningsAreVisibleDuringRunAndMergedWithReport(t *testing.T) {
	release := make(chan struct{})
	warned := make(chan struct{})
	manager := New(Options{
		Runner: func(_ context.Context, opts analysis.Options, _ analysis.ProgressSink) (*analysis.Result, error) {
			opts.OnWarning("config: unknown key")
			opts.OnWarning("history: skipped commit")
			opts.OnWarning("history: skipped commit")
			close(warned)
			<-release
			report := &aggregate.Report{Warnings: []string{"history: skipped commit", "blame: sampled"}}
			return &analysis.Result{Report: report}, nil
		},
	})
	job, err := manager.Start("/repos/project", analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	<-warned

	running, err := manager.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if running.WarningCount != 2 || len(running.Warnings) != 2 {
		t.Fatalf("running warnings = %d %q, want two distinct messages", running.WarningCount, running.Warnings)
	}
	// Snapshots are copies: mutating one must not reach the manager.
	running.Warnings[0] = "mutated"

	close(release)
	var done Snapshot
	for i := 0; i < 200; i++ {
		done, _ = manager.Get(job.ID)
		if done.Status == StatusSucceeded {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	want := []string{"config: unknown key", "history: skipped commit", "blame: sampled"}
	if done.Status != StatusSucceeded || done.WarningCount != len(want) {
		t.Fatalf("done = %s with %d warnings %q", done.Status, done.WarningCount, done.Warnings)
	}
	for i, message := range want {
		if done.Warnings[i] != message {
			t.Errorf("warning %d = %q, want %q", i, done.Warnings[i], message)
		}
	}
}

func TestWarningsAreBounded(t *testing.T) {
	manager := New(Options{})
	job, err := manager.Create("/repos/project")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxJobWarnings+20; i++ {
		if err := manager.AddWarning(job.ID, fmt.Sprintf("warning %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := manager.Get(job.ID)
	if got.WarningCount != maxJobWarnings || len(got.Warnings) != maxJobWarnings {
		t.Fatalf("retained %d warnings (count %d), want %d", len(got.Warnings), got.WarningCount, maxJobWarnings)
	}
	if err := manager.Fail(job.ID, Failure{Code: "test"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.AddWarning(job.ID, "late"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("warning after completion error = %v, want ErrInvalidState", err)
	}
}

// A runner that ignores cancellation and returns a report anyway must not turn
// a cancelled job into a succeeded one.
func TestCancellationWinsOverALateResult(t *testing.T) {
	started := make(chan struct{})
	manager := New(Options{
		Runner: func(ctx context.Context, _ analysis.Options, _ analysis.ProgressSink) (*analysis.Result, error) {
			close(started)
			<-ctx.Done()
			return &analysis.Result{Report: &aggregate.Report{}}, nil
		},
	})
	job, err := manager.Start("/repos/late", analysis.Options{})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start")
	}
	if err := manager.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	if status := waitForTerminal(t, manager, job.ID); status != StatusCancelled {
		t.Fatalf("status = %s, want cancelled", status)
	}
	if _, err := manager.Report(job.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("report of a cancelled job error = %v, want ErrInvalidState", err)
	}
}

func TestTerminalStatesCannotBeOverwritten(t *testing.T) {
	report := func() *analysis.Result { return &analysis.Result{Report: &aggregate.Report{}} }
	for _, tc := range []struct {
		name   string
		finish func(*Manager, string) error
		want   Status
	}{
		{name: "cancelled", finish: func(m *Manager, id string) error { return m.Cancelled(id, time.Time{}) }, want: StatusCancelled},
		{name: "succeeded", finish: func(m *Manager, id string) error { return m.Complete(id, report(), time.Time{}) }, want: StatusSucceeded},
		{name: "failed", finish: func(m *Manager, id string) error { return m.Fail(id, Failure{Code: "test"}, time.Time{}) }, want: StatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := New(Options{})
			job, err := manager.Create("/repos/project")
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.finish(manager, job.ID); err != nil {
				t.Fatal(err)
			}
			attempts := map[string]error{
				"complete":     manager.Complete(job.ID, report(), time.Time{}),
				"fail":         manager.Fail(job.ID, Failure{Code: "late"}, time.Time{}),
				"cancelled":    manager.Cancelled(job.ID, time.Time{}),
				"mark running": manager.MarkRunning(job.ID, time.Time{}),
			}
			for label, err := range attempts {
				if !errors.Is(err, ErrInvalidState) {
					t.Errorf("%s after %s error = %v, want ErrInvalidState", label, tc.name, err)
				}
			}
			if got, _ := manager.Get(job.ID); got.Status != tc.want {
				t.Errorf("status = %s after overwrite attempts, want %s", got.Status, tc.want)
			}
		})
	}
}

func TestHistoryKeepsExactlyTheTenNewestJobs(t *testing.T) {
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	next := 0
	manager := New(Options{
		Now: func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		},
		NewID: func() (string, error) {
			next++
			return fmt.Sprintf("job-%02d", next), nil
		},
	})
	for i := 0; i < 12; i++ {
		job, err := manager.Create("/repos/project")
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.Fail(job.ID, Failure{Code: "test"}, time.Time{}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	for _, job := range manager.List() {
		got = append(got, job.ID)
	}
	want := []string{"job-12", "job-11", "job-10", "job-09", "job-08", "job-07", "job-06", "job-05", "job-04", "job-03"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("history = %v, want %v", got, want)
	}
}

func waitForTerminal(t *testing.T, manager *Manager, id string) Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		current, err := manager.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status != StatusQueued && current.Status != StatusRunning {
			return current.Status
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s is still %s", id, current.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStaleFailureNeverCarriesUnderlyingErrors(t *testing.T) {
	for _, tc := range []struct {
		reason string
		want   string
	}{
		{reason: analysis.StaleHeadChanged, want: analysis.StaleHeadChanged},
		{reason: analysis.StaleCheckoutChanged, want: analysis.StaleCheckoutChanged},
		{reason: analysis.StaleHistoryChanged, want: analysis.StaleHistoryChanged},
		{
			reason: analysis.StaleRevalidationFailed + ": fatal: not a git repository: /home/user/secret/.git",
			want:   analysis.StaleRevalidationFailed,
		},
	} {
		manager := New(Options{})
		job, err := manager.Create("/repos/project")
		if err != nil {
			t.Fatal(err)
		}
		result := &analysis.Result{Report: &aggregate.Report{}, Stale: true, StaleReason: tc.reason}
		if err := manager.Complete(job.ID, result, time.Time{}); err != nil {
			t.Fatal(err)
		}
		got, _ := manager.Get(job.ID)
		if got.Status != StatusStale || got.Failure == nil || got.Failure.Message != tc.want {
			t.Errorf("reason %q produced %s %+v, want message %q", tc.reason, got.Status, got.Failure, tc.want)
		}
	}
}
