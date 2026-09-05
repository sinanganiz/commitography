package aggregate

import (
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/model"
)

func TestConventionalCommitDetection(t *testing.T) {
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

func TestConventionalRatioAndConfidenceLabel(t *testing.T) {
	m := buildMessages(subjects("feat: a", "fix: b", "random thing", "another one"))
	if m.ConventionalRatio != 0.5 {
		t.Errorf("conventionalRatio = %v, want 0.5", m.ConventionalRatio)
	}
	if m.LowConfidence {
		t.Error("0.5 is above the 0.30 threshold and must not be flagged low confidence")
	}

	m = buildMessages(subjects("feat: a", "random", "random", "random", "random"))
	if m.ConventionalRatio != 0.2 {
		t.Errorf("conventionalRatio = %v, want 0.2", m.ConventionalRatio)
	}
	if !m.LowConfidence {
		t.Error("0.2 is below the 0.30 threshold and must be flagged low confidence")
	}
}

func TestShortMessageCounter(t *testing.T) {
	m := buildMessages(subjects(
		"wip",                        // low-effort word
		"WIP",                        // case-insensitive
		"...",                        // punctuation placeholder
		"fixed",                      // 5 characters, at the limit
		"a much longer subject line", // not short
	))
	if m.ShortMessages != 4 {
		t.Errorf("shortMessages = %d, want 4", m.ShortMessages)
	}
}

func TestRevertAndTypoCounters(t *testing.T) {
	m := buildMessages(subjects(
		`Revert "feat: x"`,
		"revert the thing",
		"fix typo in readme",
		"Fix typos across docs",
		"not a revertible statement",
	))
	if m.RevertCount != 2 {
		t.Errorf("revertCount = %d, want 2", m.RevertCount)
	}
	if m.TypoFixCount != 2 {
		t.Errorf("typoFixCount = %d, want 2", m.TypoFixCount)
	}
}

func TestEmojiDetection(t *testing.T) {
	m := buildMessages(subjects(
		"🎉 launch day",
		"🎉 another party",
		"✨ sparkle",
		"plain text",
		"arrows -> are not emoji",
	))
	if m.EmojiCommits != 3 {
		t.Errorf("emojiCommits = %d, want 3", m.EmojiCommits)
	}
	if len(m.TopEmoji) == 0 || m.TopEmoji[0].Emoji != "🎉" || m.TopEmoji[0].Count != 2 {
		t.Errorf("topEmoji = %+v, want 🎉 leading with 2", m.TopEmoji)
	}
}

func TestLongestSubjectIsTruncatedForDisplay(t *testing.T) {
	long := strings.Repeat("x", 500)
	m := buildMessages(subjects("short", long))
	if m.LongestSubject == nil {
		t.Fatal("longestSubject is nil")
	}
	if m.LongestSubject.Length != 500 {
		t.Errorf("length = %d, want the true length 500", m.LongestSubject.Length)
	}
	if len([]rune(m.LongestSubject.Subject)) > subjectDisplayLimit+1 {
		t.Errorf("display subject is %d runes, want at most %d plus an ellipsis",
			len([]rune(m.LongestSubject.Subject)), subjectDisplayLimit)
	}
}

func TestWordCloudDropsStopwordsAndShortWords(t *testing.T) {
	m := buildMessages(subjects(
		"add caching to the resolver",
		"caching for the resolver again",
		"resolver caching improvements",
	))
	words := map[string]int{}
	for _, w := range m.TopWords {
		words[w.Word] = w.Count
	}
	if words["caching"] != 3 || words["resolver"] != 3 {
		t.Errorf("topWords = %+v, want caching and resolver at 3 each", m.TopWords)
	}
	for _, dropped := range []string{"the", "for", "add", "to"} {
		if _, present := words[dropped]; present {
			t.Errorf("%q should have been dropped as a stopword or too short", dropped)
		}
	}
}

func TestAverageSubjectLength(t *testing.T) {
	m := buildMessages(subjects("abc", "abcdefg")) // 3 and 7
	if m.AverageSubjectLength != 5.0 {
		t.Errorf("averageSubjectLength = %v, want 5.0", m.AverageSubjectLength)
	}
}

func TestMessagesOnEmptyInput(t *testing.T) {
	m := buildMessages(nil)
	if m.TypeDistribution == nil || m.TopWords == nil || m.TopEmoji == nil {
		t.Error("collections must serialize as empty, not null")
	}
	if m.LongestSubject != nil {
		t.Error("longestSubject must be null with no commits")
	}
}
