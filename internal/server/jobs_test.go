package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
)

func TestManagerAllowsOneActiveJobAndRetainsTerminalHistory(t *testing.T) {
	t.Parallel()
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	manager := newTestManager(ManagerOptions{
		Limit: 2,
		Clock: core.ClockFunc(func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		}),
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
	if err := manager.UpdateProgress(first.ID, pipeline.ProgressEvent{Sequence: 1, Stage: pipeline.StageCollecting}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Complete(first.ID, &pipeline.Result{}, time.Time{}); err != nil {
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
	t.Parallel()
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	next := 0
	manager := newTestManager(ManagerOptions{
		Limit: 2,
		Clock: core.ClockFunc(func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		}),
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
	t.Parallel()
	started := make(chan struct{}, 1)
	manager := newTestManager(ManagerOptions{
		Runner: func(ctx context.Context, _ pipeline.Options, sink pipeline.ProgressSink) (*pipeline.Result, error) {
			sink(pipeline.ProgressEvent{Sequence: 1, Stage: pipeline.StageCollecting})
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})

	job, err := manager.Start("/repos/cancellable", pipeline.Options{})
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
	t.Parallel()
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	manager := newTestManager(ManagerOptions{
		Clock: core.ClockFunc(func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		}),
		NewID: func() (string, error) { return "progress-job", nil },
	})
	job, err := manager.Create("/repos/progress")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.MarkRunning(job.ID, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateProgress(job.ID, pipeline.ProgressEvent{Sequence: 2, Stage: pipeline.StageCode}); err != nil {
		t.Fatal(err)
	}
	if err := manager.UpdateProgress(job.ID, pipeline.ProgressEvent{Sequence: 1, Stage: pipeline.StageCollecting}); !errors.Is(err, ErrInvalidState) {
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
	t.Parallel()
	manager := newTestManager(ManagerOptions{NewID: func() (string, error) { return "report-job", nil }})
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

	manager = newTestManager(ManagerOptions{NewID: func() (string, error) { return "success-job", nil }})
	job, err = manager.Create("/repos/report")
	if err != nil {
		t.Fatal(err)
	}
	result := &pipeline.Result{Report: &core.Report{}}
	if err := manager.Complete(job.ID, result, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if report, err := manager.Report(job.ID); err != nil || report == nil {
		t.Fatalf("successful report = %v, %v", report, err)
	}
}

// The build version is injected at composition and must reach every analysis
// the manager runs, whatever the caller put in the options (ADR-0061 clause 4).
func TestStartInjectsTheManagersToolVersion(t *testing.T) {
	t.Parallel()
	versions := make(chan string, 1)
	manager := newTestManager(ManagerOptions{
		ToolVersion: "v-injected",
		Runner: func(_ context.Context, options pipeline.Options, _ pipeline.ProgressSink) (*pipeline.Result, error) {
			versions <- options.ToolVersion
			return &pipeline.Result{Report: &core.Report{}}, nil
		},
	})
	job, err := manager.Start("/repos/version", pipeline.Options{ToolVersion: "from-caller"})
	if err != nil {
		t.Fatal(err)
	}
	if got := <-versions; got != "v-injected" {
		t.Errorf("runner received ToolVersion %q, want the manager's %q", got, "v-injected")
	}
	waitForTerminal(t, manager, job.ID)
}

func TestCancelAllRequestsWorkerCancellation(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 1)
	manager := newTestManager(ManagerOptions{
		Runner: func(ctx context.Context, _ pipeline.Options, _ pipeline.ProgressSink) (*pipeline.Result, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	job, err := manager.Start("/repos/shutdown", pipeline.Options{})
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

func TestWarningsAreVisibleDuringRunAndMergedWithResult(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	warned := make(chan struct{})
	manager := newTestManager(ManagerOptions{
		Runner: func(_ context.Context, opts pipeline.Options, _ pipeline.ProgressSink) (*pipeline.Result, error) {
			opts.OnWarning("config: unknown key")
			opts.OnWarning("history: skipped commit")
			opts.OnWarning("history: skipped commit")
			close(warned)
			<-release
			return &pipeline.Result{
				Report:   &core.Report{},
				Warnings: []string{"history: skipped commit", "coupling: pairs discarded"},
			}, nil
		},
	})
	job, err := manager.Start("/repos/project", pipeline.Options{})
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
	want := []string{"config: unknown key", "history: skipped commit", "coupling: pairs discarded"}
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
	t.Parallel()
	manager := newTestManager(ManagerOptions{})
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
	t.Parallel()
	started := make(chan struct{})
	manager := newTestManager(ManagerOptions{
		Runner: func(ctx context.Context, _ pipeline.Options, _ pipeline.ProgressSink) (*pipeline.Result, error) {
			close(started)
			<-ctx.Done()
			return &pipeline.Result{Report: &core.Report{}}, nil
		},
	})
	job, err := manager.Start("/repos/late", pipeline.Options{})
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
	t.Parallel()
	report := func() *pipeline.Result { return &pipeline.Result{Report: &core.Report{}} }
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
			manager := newTestManager(ManagerOptions{})
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
	t.Parallel()
	clock := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	next := 0
	manager := newTestManager(ManagerOptions{
		Clock: core.ClockFunc(func() time.Time {
			clock = clock.Add(time.Second)
			return clock
		}),
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
	// Bounded by a poll count rather than a deadline, so the test reads no
	// clock (ADR-0042 clause 4): 400 polls of 5ms is two seconds.
	const maxPolls = 400
	for poll := 0; ; poll++ {
		current, err := manager.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status != StatusQueued && current.Status != StatusRunning {
			return current.Status
		}
		if poll == maxPolls {
			t.Fatalf("job %s is still %s", id, current.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStaleFailureNeverCarriesUnderlyingErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		reason string
		want   string
	}{
		{reason: pipeline.StaleHeadChanged, want: pipeline.StaleHeadChanged},
		{reason: pipeline.StaleCheckoutChanged, want: pipeline.StaleCheckoutChanged},
		{reason: pipeline.StaleHistoryChanged, want: pipeline.StaleHistoryChanged},
		{
			reason: pipeline.StaleRevalidationFailed + ": fatal: not a git repository: /home/user/secret/.git",
			want:   pipeline.StaleRevalidationFailed,
		},
	} {
		manager := newTestManager(ManagerOptions{})
		job, err := manager.Create("/repos/project")
		if err != nil {
			t.Fatal(err)
		}
		result := &pipeline.Result{Report: &core.Report{}, Stale: true, StaleReason: tc.reason}
		if err := manager.Complete(job.ID, result, time.Time{}); err != nil {
			t.Fatal(err)
		}
		got, _ := manager.Get(job.ID)
		if got.Status != StatusStale || got.Failure == nil || got.Failure.Message != tc.want {
			t.Errorf("reason %q produced %s %+v, want message %q", tc.reason, got.Status, got.Failure, tc.want)
		}
	}
}
