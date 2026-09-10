package collect

import (
	"context"
	"os/exec"

	"github.com/sinanganiz/commitography/internal/gitcmd"
)

// gitCommand builds an *exec.Cmd for a git invocation against repoPath.
func gitCommand(repoPath string, args ...string) *exec.Cmd {
	return gitcmd.Command(repoPath, args...)
}

// gitCommandContext builds a cancellable git command.
func gitCommandContext(ctx context.Context, repoPath string, args ...string) *exec.Cmd {
	return gitcmd.CommandContext(ctx, repoPath, args...)
}

// runGit executes git and returns trimmed stdout.
func runGit(repoPath string, args ...string) (string, error) {
	return gitcmd.Run(repoPath, args...)
}

// runGitContext executes git with cancellation support.
func runGitContext(ctx context.Context, repoPath string, args ...string) (string, error) {
	return gitcmd.RunContext(ctx, repoPath, args...)
}
