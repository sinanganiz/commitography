package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// ResolveDate reads "now" from its caller, so a bare date, which git completes
// with the time of day, resolves differently at a different hour, while an
// instant resolves to itself whatever the hour. The second property is what
// makes a resolved bound reproducible (ADR-0026 clause 4).
func TestResolveDateReadsNowFromTheCaller(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	morning := time.Date(2026, 3, 4, 6, 0, 0, 0, time.UTC)
	evening := morning.Add(9 * time.Hour)

	resolve := func(bound, expression string, now time.Time) time.Time {
		t.Helper()
		at, err := ResolveDate(context.Background(), repo, bound, expression, now)
		if err != nil {
			t.Fatalf("ResolveDate(%s, %q): %v", bound, expression, err)
		}
		return at
	}

	early := resolve("since", "2026-02-01", morning)
	late := resolve("since", "2026-02-01", evening)
	if early.Equal(late) {
		t.Errorf("the bare date resolved to %s at both hours; git completes it with the time of day", early)
	}
	if !early.Before(late) {
		t.Errorf("the earlier hour resolved to %s, the later to %s", early, late)
	}

	instant := "2026-02-01T09:30:00Z"
	for _, now := range []time.Time{morning, evening} {
		got := resolve("since", instant, now)
		if want := time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC); !got.Equal(want) {
			t.Errorf("the instant %s resolved to %s at %s, want %s", instant, got, now, want)
		}
	}
	// The two bounds are read from different rev-parse output, so both forms
	// are exercised.
	if got := resolve("until", instant, morning); !got.Equal(time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("the until bound resolved to %s, want the instant itself", got)
	}
	// A relative expression is resolved against the caller's now as well.
	if got := resolve("since", "2 days ago", morning); !got.Equal(morning.AddDate(0, 0, -2)) {
		t.Errorf("two days before %s resolved to %s", morning, got)
	}
}

func TestResolveDateRejectsAnUnknownBound(t *testing.T) {
	t.Parallel()
	if _, err := ResolveDate(context.Background(), "", "between", "2026-02-01", time.Unix(0, 0)); err == nil {
		t.Error("a bound that is neither since nor until was accepted")
	}
}

func TestCommandContextCancelsGitProcess(t *testing.T) {
	t.Parallel()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	cmd := CommandContext(ctx, repo, "cat-file", "--batch")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting git: %v", err)
	}

	err = cmd.Wait()
	_ = stdin.Close()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("context error = %v, want deadline exceeded", ctx.Err())
	}
	if err == nil {
		t.Fatal("cancelled git process returned nil error")
	}
}
