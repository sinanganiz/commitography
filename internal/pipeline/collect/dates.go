package collect

import (
	"context"
	"time"

	"github.com/sinanganiz/commitography/internal/git"
)

// ResolveDateBounds resolves the analysis plane's date bounds to the instants
// git selects commits by, in RFC 3339 and UTC. An empty bound stays empty: it
// means no bound, and git would read an empty expression as now.
//
// The resolved form is what the report embeds and what the history is read
// with, so rerunning from a report selects the same commits at any later hour
// (ADR-0026 clause 4). Git reads "now" from the collector's clock.
func (c *Collector) ResolveDateBounds(ctx context.Context, repoPath, since, until string) (string, string, error) {
	now := c.clock.Now()
	resolve := func(bound, expression string) (string, error) {
		if expression == "" {
			return "", nil
		}
		at, err := git.ResolveDate(ctx, repoPath, bound, expression, now)
		if err != nil {
			return "", err
		}
		return at.Format(time.RFC3339), nil
	}
	resolvedSince, err := resolve("since", since)
	if err != nil {
		return "", "", err
	}
	resolvedUntil, err := resolve("until", until)
	if err != nil {
		return "", "", err
	}
	return resolvedSince, resolvedUntil, nil
}
