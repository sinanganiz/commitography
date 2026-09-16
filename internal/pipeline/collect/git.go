package collect

import (
	"context"

	"github.com/sinanganiz/commitography/internal/git"
)

// runGit executes git and returns trimmed stdout.
func runGit(repoPath string, args ...string) (string, error) {
	return git.Run(repoPath, args...)
}

// runGitContext executes git with cancellation support.
func runGitContext(ctx context.Context, repoPath string, args ...string) (string, error) {
	return git.RunContext(ctx, repoPath, args...)
}
