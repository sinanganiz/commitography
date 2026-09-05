package aggregate

// stopwords are dropped from the word cloud. The list mixes ordinary English
// function words with the vocabulary every commit log is saturated with, since
// "commit", "update" and "merge" say nothing about what a repository is for.
var stopwords = map[string]bool{}

func init() {
	list := []string{
		// English function words
		"the", "and", "for", "are", "but", "not", "you", "all", "any", "can",
		"had", "her", "was", "one", "our", "out", "day", "get", "has", "him",
		"his", "how", "its", "may", "new", "now", "old", "see", "two", "way",
		"who", "did", "she", "use", "her", "than", "them", "then", "there",
		"these", "they", "this", "that", "with", "from", "have", "here",
		"into", "just", "like", "more", "most", "some", "such", "only",
		"other", "over", "same", "should", "since", "still", "their", "those",
		"through", "under", "until", "very", "were", "what", "when", "where",
		"which", "while", "will", "would", "your", "about", "after", "again",
		"also", "been", "before", "being", "both", "each", "does", "doing",
		"done", "during", "even", "ever", "every", "off", "once", "onto",
		"per", "than", "too", "via", "was", "yet",

		// Commit-log filler: present in every repository, informative in none
		"commit", "commits", "committed", "merge", "merged", "merges",
		"branch", "branches", "pull", "request", "master", "main", "develop",
		"change", "changes", "changed", "update", "updates", "updated",
		"updating", "add", "adds", "added", "adding", "remove", "removes",
		"removed", "removing", "fix", "fixes", "fixed", "fixing", "make",
		"makes", "made", "set", "sets", "use", "uses", "used", "using",
		"initial", "version", "release", "wip", "todo", "misc", "minor",
		"small", "little", "some", "stuff", "things", "thing", "code",
		"file", "files", "line", "lines", "test", "tests", "testing",
		"revert", "reverts", "reverted", "bump", "bumps", "bumped",
	}
	for _, w := range list {
		stopwords[w] = true
	}
}
