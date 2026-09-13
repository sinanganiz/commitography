package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture %q not built; run `make fixtures`", name)
	}
	return path
}

func TestRunDoesNothingWithACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var events []ProgressEvent
	result, err := Run(ctx, Options{RepoPath: fixture(t, "basic"), NoBlame: true}, func(event ProgressEvent) {
		events = append(events, event)
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("a cancelled run returned a result: %+v", result)
	}
	if len(events) != 0 {
		t.Fatalf("a cancelled run emitted %d progress events", len(events))
	}
}

// Cancelling while git log streams history stops the Git process, and the run
// reports the cancellation rather than the failure of the killed process.
func TestRunReportsCancellationDuringHistoryCollection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := Run(ctx, Options{RepoPath: fixture(t, "basic"), NoBlame: true}, func(event ProgressEvent) {
		if event.Stage == StageCollecting && event.Current > 0 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("a cancelled run returned a result: %+v", result)
	}
}

// Cancelling once history has been read stops the run at the next checkpoint:
// no metric stage runs and no report is produced.
func TestRunStopsAtTheNextCheckpointAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stages []string
	result, err := Run(ctx, Options{RepoPath: fixture(t, "basic"), NoBlame: true}, func(event ProgressEvent) {
		stages = append(stages, event.Stage)
		if event.Stage == StageIdentity {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("a cancelled run returned a result: %+v", result)
	}
	for _, stage := range stages {
		switch stage {
		case StageTemporal, StageCode, StageMessages, StageSocial, StageNotables, StageFinalizing:
			t.Fatalf("stage %q ran after cancellation; stages seen: %v", stage, stages)
		}
	}
}

// The job server and the CLI may run analyses side by side; a warning raised
// by one run must reach only that run's sink.
func TestConcurrentRunsKeepTheirWarningsApart(t *testing.T) {
	repo := fixture(t, "basic")
	keys := []string{"first_unknown_key", "second_unknown_key"}

	var wg sync.WaitGroup
	warnings := make([][]string, len(keys))
	for i, key := range keys {
		path := filepath.Join(t.TempDir(), "commitography.yml")
		if err := os.WriteFile(path, []byte(key+": true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			var mu sync.Mutex
			_, err := Run(context.Background(), Options{
				RepoPath:   repo,
				ConfigPath: path,
				NoBlame:    true,
				OnWarning: func(message string) {
					mu.Lock()
					defer mu.Unlock()
					warnings[i] = append(warnings[i], message)
				},
			}, nil)
			if err != nil {
				t.Errorf("run %d: %v", i, err)
			}
		}(i, path)
	}
	wg.Wait()

	for i, key := range keys {
		other := keys[1-i]
		joined := strings.Join(warnings[i], "\n")
		if !strings.Contains(joined, key) {
			t.Errorf("the run configured with %s did not receive its warning: %q", key, warnings[i])
		}
		if strings.Contains(joined, other) {
			t.Errorf("the run configured with %s received the other run's warning: %q", key, warnings[i])
		}
	}
}
