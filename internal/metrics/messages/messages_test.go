package messages

import (
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

func TestConventionalCommitDetection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		subject      string
		category     string
		conventional bool
	}{
		{"feat: add login", "feat", true},
		{"fix(parser): handle empty input", "fix", true},
		{"feat(api)!: drop v1 endpoints", "feat", true},
		{"chore(deps): bump lodash", "chore", true},
		{"BUILD: something", "ci", false}, // type must be lowercase
		// The strict form requires ": ", so this falls through to the
		// heuristics, where nothing matches.
		{"feat:no space", "other", false},
		{"feat", "other", false}, // bare word, no colon
		{"refactor: extract helper", "refactor", true},
	}
	for _, c := range cases {
		category, conventional := Classify(c.subject)
		if conventional != c.conventional {
			t.Errorf("Classify(%q) conventional = %v, want %v", c.subject, conventional, c.conventional)
		}
		if category != c.category {
			t.Errorf("Classify(%q) = %q, want %q", c.subject, category, c.category)
		}
	}
}

func TestHeuristicRuleOrdering(t *testing.T) {
	t.Parallel()
	cases := []struct {
		subject string
		want    string
	}{
		// 1. revert
		{"Revert the broken migration", "revert"},
		{`Revert "feat: add login"`, "revert"},
		{"revert everything", "revert"},

		// 2. merge
		{"Merge branch 'topic' into main", "merge"},
		{"Merge pull request #42 from fork/branch", "merge"},
		{"Merge remote-tracking branch 'origin/main'", "merge"},

		// 3. fix
		{"Fixed the crash on startup", "fix"},
		{"hotfix for the login flow", "fix"},
		{"Correct the rounding in totals", "fix"},

		// 4. feat
		{"Implement the export button", "feat"},
		{"Introduce a settings panel", "feat"},
		{"Create the onboarding flow", "feat"},

		// 5. refactor
		{"Rename the handler package", "refactor"},
		{"Simplify the retry loop", "refactor"},
		{"Extract the parser into its own file", "refactor"},

		// 6. test
		{"More testing around the queue", "test"},
		{"Specs for the reducer", "test"},
		{"Flaky test quarantined", "test"},

		// 7. docs
		{"README polish", "docs"},
		{"Documentation for the new flag", "docs"},
		{"Comment the tricky branch", "docs"},

		// 8. chore
		{"Bump the go toolchain", "chore"},
		{"Upgrade the linter", "chore"},
		{"deps: nothing interesting", "chore"},

		// 9. style
		{"gofmt the tree", "style"},
		{"Prettier pass", "style"},
		{"Formatting only", "style"},

		// 10. ci
		{"Pipeline tweaks", "ci"},
		{"Deploy script adjustments", "ci"},
		{"Workflow permissions", "ci"},

		// 11. no match
		{"Tuesday", "other"},
		{"asdf", "other"},
		{"...", "other"},
	}
	for _, c := range cases {
		if got, _ := Classify(c.subject); got != c.want {
			t.Errorf("Classify(%q) = %q, want %q", c.subject, got, c.want)
		}
	}
}

func TestEarlierRuleWinsOverLaterOne(t *testing.T) {
	t.Parallel()
	// Rule 3 (fix) precedes rule 4 (feat), so a subject mentioning both is a fix.
	if got, _ := Classify("Fix the crash and add a guard"); got != "fix" {
		t.Errorf("Classify = %q, want %q by rule ordering", got, "fix")
	}
	// Rule 1 (revert) precedes rule 2 (merge).
	if got, _ := Classify(`Revert "Merge branch 'topic'"`); got != "revert" {
		t.Errorf("Classify = %q, want %q by rule ordering", got, "revert")
	}
	// A subject that falls through several rules before matching.
	if got, _ := Classify("Prettier config"); got != "style" {
		t.Errorf("Classify = %q, want %q after falling through earlier rules", got, "style")
	}
}

func subjects(list ...string) []model.Commit {
	out := make([]model.Commit, 0, len(list))
	for i, s := range list {
		out = append(out, model.Commit{Hash: string(rune('a' + i)), Subject: s})
	}
	return out
}

func TestConventionalRatioAndConfidence(t *testing.T) {
	t.Parallel()
	f := Build(subjects("feat: a", "fix: b", "random thing", "another one"))
	if *f.Metrics.ConventionalRatio != 0.5 {
		t.Errorf("conventional_ratio = %v, want 0.5", *f.Metrics.ConventionalRatio)
	}
	if f.Status != core.StatusOK {
		t.Errorf("0.5 is above the 0.30 threshold, yet the family is %s %v", f.Status, f.Reasons)
	}

	f = Build(subjects("feat: a", "random", "random", "random", "random"))
	if *f.Metrics.ConventionalRatio != 0.2 {
		t.Errorf("conventional_ratio = %v, want 0.2", *f.Metrics.ConventionalRatio)
	}
	if f.Status != core.StatusDegraded || f.Reasons[0] != core.ReasonLowClassificationConfidence ||
		f.Confidence != core.ConfidenceLow {
		t.Errorf("0.2 is below the 0.30 threshold, yet the family is %s %v %s", f.Status, f.Reasons, f.Confidence)
	}
}

func TestRevertAndTypoCounters(t *testing.T) {
	t.Parallel()
	m := Build(subjects(
		`Revert "feat: x"`,
		"revert the thing",
		"fix typo in readme",
		"Fix typos across docs",
		"not a revertible statement",
	)).Metrics
	if *m.RevertCount != 2 {
		t.Errorf("revert_count = %d, want 2", *m.RevertCount)
	}
	if *m.FixTypoCount != 2 {
		t.Errorf("fix_typo_count = %d, want 2", *m.FixTypoCount)
	}
}

func TestMeanSubjectLength(t *testing.T) {
	t.Parallel()
	m := Build(subjects("abc", "abcdefg")).Metrics // 3 and 7
	if *m.MeanSubjectLength != 5.0 {
		t.Errorf("mean_subject_length = %v, want 5.0", *m.MeanSubjectLength)
	}
}

func TestMessagesOnEmptyInput(t *testing.T) {
	t.Parallel()
	f := Build(nil)
	if f.Status != core.StatusDegraded || f.Reasons[0] != core.ReasonEmptyPopulation {
		t.Errorf("family = %s %v, want degraded with empty_population", f.Status, f.Reasons)
	}
	if f.Metrics != (core.MessagesMetrics{}) {
		t.Errorf("metrics over no commits = %+v, want every metric absent", f.Metrics)
	}
}
