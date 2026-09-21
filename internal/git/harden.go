// The hardening set of ADR-0065 clause 2, in one place so that no invocation
// can omit an item: the configuration flags that go before every subcommand,
// and the sanitised environment every invocation runs in.
//
// Both are functions rather than package variables, because a variable would
// be mutable state the rest of the process could reach (ADR-0042 clause 2)
// and, worse, something a caller could weaken.

package git

import (
	"os"
	"strings"
)

// configFlags are the mandatory configuration flags ADR-0065 clause 2 places
// before the subcommand of every invocation.
func configFlags() []string {
	return []string{
		// The pager would block on a pipe nothing reads.
		"--no-pager",

		// Hooks are repository content and therefore attacker-controlled
		// (ADR-0045). An empty value does not disable them: git resolves it
		// against the working directory, so a file named after a hook in the
		// repository's own root would run. A path that cannot be a directory
		// disables the lookup on every supported platform, and Git for
		// Windows maps /dev/null onto NUL.
		"-c", "core.hooksPath=/dev/null",

		// No invocation on the analysis path uses a transport, so none is
		// allowed. The package that clones a remote (WP-0041) states which it
		// needs rather than inheriting a permissive default.
		"-c", "protocol.allow=never",

		// Belt and braces with --no-pager: a pager configured in the
		// repository cannot start one either.
		"-c", "core.pager=cat",

		// A submodule is a separate repository that the analysis never
		// descends into.
		"-c", "submodule.recurse=false",

		// An empty helper list means an invocation that would need a
		// credential fails instead of prompting or reading a store
		// (ADR-0016 clause 2).
		"-c", "credential.helper=",
	}
}

// fixedEnvironment is the environment ADR-0065 clause 2 requires of every
// invocation. The C locale is load-bearing twice over: it keeps git's own
// output stable, and it is what makes matching on git's standard error in
// classify.go valid rather than a guess about the operator's language.
func fixedEnvironment() []string {
	return []string{
		// System configuration disabled.
		"GIT_CONFIG_NOSYSTEM=1",
		// Terminal prompting disabled.
		"GIT_TERMINAL_PROMPT=0",
		// An empty credential prompt helper, for git and for ssh.
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		// Pager, again, for the path that reads the variable rather than the
		// configuration key.
		"GIT_PAGER=cat",
		// Reading a repository takes no lock the operator would want kept.
		"GIT_OPTIONAL_LOCKS=0",
		// A fixed C locale.
		"LC_ALL=C",
		"LANG=C",
		"LANGUAGE=",
	}
}

// environment builds the sanitised environment for one invocation: the
// process's own, less every GIT_ variable it inherited, plus the fixed set and
// the pinned one (pinned.go), plus whatever the invocation sets for itself.
//
// Inherited GIT_ variables go because each of them changes what git reads or
// writes — GIT_DIR, GIT_WORK_TREE, GIT_CONFIG_GLOBAL and GIT_CONFIG_COUNT
// among them — and the operator's shell is not part of the analysis input.
// Everything else stays: git needs PATH, a home directory and, on Windows,
// the system root, and removing those would not harden anything.
//
// Later entries win, which os/exec guarantees by keeping the last occurrence
// of a name, so the fixed set overrides an inherited value and an
// invocation's own entry overrides the fixed set.
func environment(extra []string) []string {
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+len(fixedEnvironment())+len(pinnedEnvironment())+len(extra))
	for _, entry := range inherited {
		if name, _, ok := strings.Cut(entry, "="); !ok || isGitVariable(name) {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, fixedEnvironment()...)
	env = append(env, pinnedEnvironment()...)
	return append(env, extra...)
}

// isGitVariable reports whether a variable name is one git reads. Windows
// treats names case-insensitively, so the comparison does too rather than
// depending on the case the parent process used.
func isGitVariable(name string) bool {
	return strings.HasPrefix(strings.ToUpper(name), "GIT_")
}
