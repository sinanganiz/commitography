package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/analysis"
	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/server"
)

func TestReportExplainsMissingMountsOnlyInsideContainers(t *testing.T) {
	notRepository := &UsageError{err: &analysis.UsageError{Err: &collect.NotRepositoryError{Path: "/repo"}}}
	missingRoot := &server.MissingRootError{Root: "/repos"}

	for _, tc := range []struct {
		name        string
		err         error
		inContainer bool
		want        []string
		exact       string
	}{
		{
			name:  "native missing repository is unchanged",
			err:   notRepository,
			exact: "Error: /repo is not a git repository\n",
		},
		{
			name:        "container missing repository gets a mount hint",
			err:         notRepository,
			inContainer: true,
			want:        []string{"Error: /repo is not a git repository", "hint:", "--mount type=bind,source=<repository>,target=/repo,readonly", "mistyped"},
		},
		{
			name:  "native missing allowed root is unchanged",
			err:   missingRoot,
			exact: "Error: allowed root \"/repos\" does not exist\n",
		},
		{
			name:        "container missing allowed root gets a mount hint",
			err:         missingRoot,
			inContainer: true,
			want:        []string{`allowed root "/repos" does not exist`, "--mount type=bind,source=<folder>,target=/repos,readonly"},
		},
		{
			name:        "other errors get no hint",
			err:         &collect.ShallowError{Path: "/repo"},
			inContainer: true,
			exact:       "Error: " + collect.ShallowMessage + "\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			report(&out, tc.err, tc.inContainer)
			if tc.exact != "" && out.String() != tc.exact {
				t.Fatalf("output = %q, want %q", out.String(), tc.exact)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output %q does not contain %q", out.String(), want)
				}
			}
		})
	}
}
