package gitcmd

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandContextCancelsGitProcess(t *testing.T) {
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
