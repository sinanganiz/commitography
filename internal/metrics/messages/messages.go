// Package messages is the messages metric family (ADR-0024, ADR-0040): what
// commit subjects reveal. Its metrics are those of docs/metrics.md section 4
// (ADR-0062); a metric that section does not define is not computed here,
// which is why the word-frequency metric and its stopword list are gone
// (ADR-0062 clause 5).
package messages

import (
	"regexp"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// lowConfidenceThreshold is the conventional-commit ratio below which the
// family is degraded with low_classification_confidence (docs/metrics.md
// section 4).
const lowConfidenceThreshold = 0.30

// version is the family version (ADR-0031 clause 2).
func version() core.Version { return core.Version{Major: 1, Minor: 0} }

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

// Build computes the messages family over the analysed commits. With none,
// every metric is absent and the family is degraded with empty_population.
func Build(commits []model.Commit) core.Family[core.MessagesMetrics] {
	if len(commits) == 0 {
		f := core.Computed(version(), core.MessagesMetrics{})
		f.Degrade(core.ReasonEmptyPopulation, core.ConfidencePartial)
		return f
	}

	classifier := NewClassifier()
	revertRe := regexp.MustCompile(`(?i)^revert\b`)
	typoFixRe := regexp.MustCompile(`(?i)\btypos?\b`)

	conventional, totalLength, reverts, typoFixes := 0, 0, 0, 0
	for _, c := range commits {
		trimmed := strings.TrimSpace(c.Subject)
		if _, isConventional := classifier.Classify(c.Subject); isConventional {
			conventional++
		}
		totalLength += len([]rune(c.Subject))
		if revertRe.MatchString(trimmed) {
			reverts++
		}
		if typoFixRe.MatchString(trimmed) {
			typoFixes++
		}
	}

	ratio := core.Round(float64(conventional)/float64(len(commits)), 4)
	meanLength := core.Round(float64(totalLength)/float64(len(commits)), 1)
	f := core.Computed(version(), core.MessagesMetrics{
		ConventionalRatio: &ratio,
		MeanSubjectLength: &meanLength,
		RevertCount:       &reverts,
		FixTypoCount:      &typoFixes,
	})
	if ratio < lowConfidenceThreshold {
		f.Degrade(core.ReasonLowClassificationConfidence, core.ConfidenceLow)
	}
	return f
}
