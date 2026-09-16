// Report types for the messages section, part of the report.json contract
// (ADR-0021, ADR-0031).

package core

// LongestSubject describes the most verbose commit message in the history.
type LongestSubject struct {
	Hash    string `json:"hash"`
	Length  int    `json:"length"`
	Subject string `json:"subject"`
}

// WordCount is one entry of the word cloud.
type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

// EmojiCount is one entry of the emoji ranking.
type EmojiCount struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

// MessageMetrics describes what commit messages reveal about how a team works.
type MessageMetrics struct {
	TypeDistribution     map[string]int  `json:"typeDistribution"`
	ConventionalRatio    float64         `json:"conventionalRatio"`
	LowConfidence        bool            `json:"lowConfidence"`
	ShortMessages        int             `json:"shortMessages"`
	LongestSubject       *LongestSubject `json:"longestSubject"`
	AverageSubjectLength float64         `json:"averageSubjectLength"`
	EmojiCommits         int             `json:"emojiCommits"`
	TopEmoji             []EmojiCount    `json:"topEmoji"`
	RevertCount          int             `json:"revertCount"`
	TypoFixCount         int             `json:"typoFixCount"`
	TopWords             []WordCount     `json:"topWords"`
}
