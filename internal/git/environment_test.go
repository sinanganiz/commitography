package git

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestEnvironmentAndFlagsOnEveryInvocation enforces ADR-0065 clause 2 the only
// way it can be enforced: by reading what reached the process. Every entry
// point of this package is driven against a real repository through a
// recording program, and every invocation it captured is then checked for the
// whole hardening set.
//
// It asserts over the invocations that happened rather than over a list this
// test wrote, so an entry point that built its command some other way would
// show up as a captured invocation missing a flag, not as a case nobody
// added.
func TestEnvironmentAndFlagsOnEveryInvocation(t *testing.T) {
	t.Parallel()
	recorder := newRecorder(t)
	repo := repositoryRoot(t)
	ctx := context.Background()

	// One call per exported way of running git.
	if _, err := Output(ctx, recorder.spec(At(repo, "rev-parse", "--git-dir"))); err != nil {
		t.Fatalf("Output: %v", err)
	}
	if _, err := Records(ctx, recorder.spec(At(repo, "ls-files", "-z").Pathspecs("go.mod"))); err != nil {
		t.Fatalf("Records: %v", err)
	}
	if err := Scan(ctx, recorder.spec(At(repo, "log", "-z", "--pretty=format:%H", "-n", "1").Pathspecs()),
		func(string) error { return nil }); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	process, err := Start(ctx, recorder.spec(At(repo, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("Start/Wait: %v", err)
	}
	process.Close()
	if _, err := resolveDate(ctx, recorder.spec(At(repo)), "since", "2026-02-01",
		time.Date(2026, 3, 4, 6, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("ResolveDate: %v", err)
	}

	invocations := recorder.invocations(t)
	if len(invocations) < 5 {
		t.Fatalf("ADR-0064: only %d invocations were recorded, so the checker is not reading the "+
			"entry points it drives", len(invocations))
	}

	for _, invocation := range invocations {
		named := strings.Join(invocation.Args, " ")

		// The mandatory configuration flags, before the subcommand.
		for _, flag := range configFlags() {
			if !contains(invocation.Args, flag) {
				t.Errorf("ADR-0065 clause 2: `git %s` was invoked without %q", named, flag)
			}
		}
		if subcommand := firstSubcommand(invocation.Args); subcommand >= 0 {
			for i, arg := range invocation.Args {
				if arg == "-c" && i > subcommand {
					t.Errorf("ADR-0065 clause 2: `git %s` carries a configuration flag after its subcommand",
						named)
				}
			}
		}

		// The sanitised environment.
		env := environmentMap(invocation.Env)
		for _, entry := range fixedEnvironment() {
			name, want, _ := strings.Cut(entry, "=")
			got, ok := env[name]
			if !ok {
				t.Errorf("ADR-0065 clause 2: `git %s` ran without %s", named, name)
				continue
			}
			if got != want {
				t.Errorf("ADR-0065 clause 2: `git %s` ran with %s=%q, want %q", named, name, got, want)
			}
		}
		for name := range env {
			if !isGitVariable(name) || isFixed(name) {
				continue
			}
			t.Errorf("ADR-0065 clause 2: `git %s` inherited %s, which the sanitised environment removes",
				named, name)
		}

		// User-derived operands behind the separator.
		if i := index(invocation.Args, "--"); i >= 0 {
			for _, arg := range invocation.Args[i+1:] {
				if strings.HasPrefix(arg, "-") {
					t.Errorf("ADR-0065 clause 2: `git %s` places %q after the separator, where it is "+
						"an operand rather than an option", named, arg)
				}
			}
		}
	}
}

// TestPathspecsPlacesTheSeparator covers the operand half of ADR-0065
// clause 2 at the level it is decided, including the case that has no
// operands: the separator still goes in, which is what stops a pathspec added
// later from being read as an option.
func TestPathspecsPlacesTheSeparator(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		spec Spec
		want []string
	}{
		{
			name: "no separator without pathspecs",
			spec: At("", "rev-parse", "--git-dir"),
			want: []string{"rev-parse", "--git-dir"},
		},
		{
			name: "a separator with no operands",
			spec: At("", "log").Pathspecs(),
			want: []string{"log", "--"},
		},
		{
			name: "operands after the separator",
			spec: At("", "log").Pathspecs("--not-an-option", "docs/"),
			want: []string{"log", "--", "--not-an-option", "docs/"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.spec.arguments()
			tail := got[len(got)-len(tc.want):]
			for i, want := range tc.want {
				if tail[i] != want {
					t.Errorf("argument vector ends %v, want it to end %v", tail, tc.want)
					break
				}
			}
		})
	}
}

// TestRepositoryIsPassedWithoutChangingDirectory keeps -C in the vector: an
// invocation that relied on the process's working directory would not be
// reproducible from the Spec alone.
func TestRepositoryIsPassedWithoutChangingDirectory(t *testing.T) {
	t.Parallel()
	args := At(filepath.FromSlash("/tmp/repo"), "status").arguments()
	if i := index(args, "-C"); i < 0 || args[i+1] != filepath.FromSlash("/tmp/repo") {
		t.Errorf("the argument vector %v does not carry -C with the repository", args)
	}
}

// environmentMap indexes an environment by variable name. Names are compared
// in upper case because Windows treats them that way.
func environmentMap(env []string) map[string]string {
	out := map[string]string{}
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok {
			out[strings.ToUpper(name)] = value
		}
	}
	return out
}

// isFixed reports whether a git variable is one the fixed set sets.
func isFixed(name string) bool {
	for _, entry := range append(fixedEnvironment(), "GIT_TEST_DATE_NOW=") {
		if fixed, _, _ := strings.Cut(entry, "="); strings.EqualFold(fixed, name) {
			return true
		}
	}
	return false
}

// firstSubcommand is the index of the first argument that is not a global
// option or a global option's value.
func firstSubcommand(args []string) int {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-c", args[i] == "-C":
			i++
		case strings.HasPrefix(args[i], "-"):
		default:
			return i
		}
	}
	return -1
}

func index(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

func contains(args []string, want string) bool { return index(args, want) >= 0 }
