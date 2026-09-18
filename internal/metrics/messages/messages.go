// Package messages is the messages metric family (ADR-0024, ADR-0040):
// commit subject classification and message statistics.
package messages

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

const (
	// lowConfidenceThreshold is the conventional-commit ratio below which the
	// classification section is labelled unreliable rather than presented as
	// fact. Heuristics on free-form messages are guesses, and the dashboard
	// says so.
	lowConfidenceThreshold = 0.30

	subjectDisplayLimit = 200
	topWordsLimit       = 40
	topEmojiLimit       = 10
	shortSubjectLength  = 5
)

// heuristicRule is one fallback classifier. Order matters: the first match
// wins, so a subject mentioning both a fix and an addition is a fix.
type heuristicRule struct {
	re       *regexp.Regexp
	category string
}

// Classifier assigns commit subjects to categories. Its patterns are compiled
// once, when it is constructed, and nothing in it changes afterwards.
type Classifier struct {
	// conventional is the strict Conventional Commits form. Anything matching
	// it is classified with certainty; everything else falls through to the
	// heuristics.
	conventional *regexp.Regexp
	heuristics   []heuristicRule
}

// NewClassifier compiles the classification patterns.
func NewClassifier() *Classifier {
	return &Classifier{
		conventional: regexp.MustCompile(
			`^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]*\))?(!)?: .+`),
		heuristics: []heuristicRule{
			{regexp.MustCompile(`(?i)^revert\b|^revert "`), "revert"},
			{regexp.MustCompile(`(?i)\b(merge branch|merge pull request|merge remote-tracking)\b`), "merge"},
			{regexp.MustCompile(`(?i)\b(fix|fixes|fixed|bugfix|hotfix|patch|resolves?|resolved|correct)\b`), "fix"},
			{regexp.MustCompile(`(?i)\b(add|adds|added|implement|introduce|create|new feature|feature)\b`), "feat"},
			{regexp.MustCompile(`(?i)\b(refactor|rename|restructure|cleanup|clean up|simplify|extract)\b`), "refactor"},
			{regexp.MustCompile(`(?i)\b(test|tests|testing|spec|specs)\b`), "test"},
			{regexp.MustCompile(`(?i)\b(doc|docs|documentation|readme|comment)\b`), "docs"},
			{regexp.MustCompile(`(?i)\b(bump|upgrade|update dependenc|dependency|deps)\b`), "chore"},
			{regexp.MustCompile(`(?i)\b(style|format|formatting|lint|prettier|gofmt)\b`), "style"},
			{regexp.MustCompile(`(?i)\b(ci|pipeline|workflow|build|release|deploy)\b`), "ci"},
		},
	}
}

// isLowEffort reports whether a lower-cased, trimmed subject is one of the
// placeholder messages that mean "I did not want to write a message".
func isLowEffort(subject string) bool {
	switch subject {
	case "wip", "fix", "fixes", "update", "updates", "changes", "stuff", "oops", "asdf",
		".", "..", "...":
		return true
	}
	return false
}

// Classify returns the category of a commit subject and whether it matched the
// strict Conventional Commits form. It compiles the patterns on every call; a
// caller classifying many subjects constructs one Classifier instead.
func Classify(subject string) (category string, conventional bool) {
	return NewClassifier().Classify(subject)
}

// Classify returns the category of a commit subject and whether it matched the
// strict Conventional Commits form.
func (c *Classifier) Classify(subject string) (category string, conventional bool) {
	if m := c.conventional.FindStringSubmatch(subject); m != nil {
		return strings.ToLower(m[1]), true
	}
	for _, rule := range c.heuristics {
		if rule.re.MatchString(subject) {
			return rule.category, false
		}
	}
	return "other", false
}

// BuildMessages computes the message metrics over the analyzed commits.
func BuildMessages(commits []model.Commit) core.MessageMetrics {
	m := core.MessageMetrics{
		TypeDistribution: map[string]int{},
		TopEmoji:         []core.EmojiCount{},
		TopWords:         []core.WordCount{},
	}
	if len(commits) == 0 {
		return m
	}

	classifier := NewClassifier()
	revertRe := regexp.MustCompile(`(?i)^revert\b`)
	typoFixRe := regexp.MustCompile(`(?i)\btypos?\b`)
	wordRe := regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}'_-]*`)
	ignored := stopwords()

	conventional := 0
	totalLength := 0
	words := map[string]int{}
	emoji := map[string]int{}

	for _, c := range commits {
		subject := c.Subject
		trimmed := strings.TrimSpace(subject)

		category, isConventional := classifier.Classify(subject)
		m.TypeDistribution[category]++
		if isConventional {
			conventional++
		}

		totalLength += len([]rune(subject))

		if len([]rune(trimmed)) <= shortSubjectLength || isLowEffort(strings.ToLower(trimmed)) {
			m.ShortMessages++
		}
		if revertRe.MatchString(trimmed) {
			m.RevertCount++
		}
		if typoFixRe.MatchString(trimmed) {
			m.TypoFixCount++
		}

		if found := emojiIn(subject); len(found) > 0 {
			m.EmojiCommits++
			for _, e := range found {
				emoji[e]++
			}
		}

		if m.LongestSubject == nil || len([]rune(subject)) > m.LongestSubject.Length {
			m.LongestSubject = &core.LongestSubject{
				Hash:    c.Hash,
				Length:  len([]rune(subject)),
				Subject: truncateRunes(subject, subjectDisplayLimit),
			}
		}

		for _, w := range wordRe.FindAllString(strings.ToLower(subject), -1) {
			if len([]rune(w)) < 3 || ignored[w] {
				continue
			}
			words[w]++
		}
	}

	m.ConventionalRatio = core.Round(float64(conventional)/float64(len(commits)), 4)
	m.LowConfidence = m.ConventionalRatio < lowConfidenceThreshold
	m.AverageSubjectLength = core.Round(float64(totalLength)/float64(len(commits)), 1)
	m.TopWords = topWords(words, topWordsLimit)
	m.TopEmoji = topEmoji(emoji, topEmojiLimit)

	return m
}

// isEmojiRune covers the pictographic blocks plus the dingbats and the
// variation selector that turns older symbols into emoji.
func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r == 0xFE0F:
		return true
	default:
		return false
	}
}

// emojiIn returns the emoji present in a subject. An arrow in U+2190–U+21FF
// counts only when followed by the variation selector, since those code points
// are ordinary typographic arrows otherwise.
func emojiIn(subject string) []string {
	var found []string
	runes := []rune(subject)
	for i, r := range runes {
		switch {
		case r == 0xFE0F:
			continue
		case r >= 0x2190 && r <= 0x21FF:
			if i+1 < len(runes) && runes[i+1] == 0xFE0F {
				found = append(found, string(r))
			}
		case isEmojiRune(r):
			found = append(found, string(r))
		}
	}
	return found
}

func topWords(counts map[string]int, limit int) []core.WordCount {
	out := make([]core.WordCount, 0, len(counts))
	for w, n := range counts {
		out = append(out, core.WordCount{Word: w, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Word < out[j].Word
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func topEmoji(counts map[string]int, limit int) []core.EmojiCount {
	out := make([]core.EmojiCount, 0, len(counts))
	for e, n := range counts {
		out = append(out, core.EmojiCount{Emoji: e, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Emoji < out[j].Emoji
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return strings.TrimRightFunc(string(runes[:limit]), unicode.IsSpace) + "…"
}
