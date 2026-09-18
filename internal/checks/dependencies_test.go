package checks

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
	"testing"
)

const allowListFile = "dependency-allowlist.txt"

// pendingRemoval narrows the checker for direct frontend dependencies that the
// audit marks as forbidden by ADR-0038 clause 1. They are deliberately absent
// from the allow list, and web/package.json is outside the scope of the
// package that introduced this checker. WP-0047 removes them; once they are
// gone this list must be emptied, which widens the checker to every
// dependency.
func pendingRemoval() map[string]bool {
	return map[string]bool{
		"npm @mui/material":   true,
		"npm @emotion/react":  true,
		"npm @emotion/styled": true,
	}
}

// TestDependencyAllowList enforces ADR-0049 clause 2 and 3: every direct
// dependency in a tracked Go or npm manifest appears in the allow list.
func TestDependencyAllowList(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	if !repo.isTracked(allowListFile) {
		fatal(t, 49, "the dependency allow list %s is not tracked", allowListFile)
	}
	allowed := parseAllowList(t, repo.read(t, 49, allowListFile))
	pending := pendingRemoval()

	manifests := 0
	for _, file := range repo.tracked {
		var declared []string
		switch path.Base(file) {
		case "go.mod":
			declared = goDirectDependencies(repo.read(t, 49, file))
		case "package.json":
			declared = npmDirectDependencies(t, file, repo.read(t, 49, file))
		default:
			continue
		}
		manifests++
		for _, dep := range declared {
			if !allowed[dep] && !pending[dep] {
				report(t, 49, "%s declares direct dependency %q, which is not on the allow list in %s", file, dep, allowListFile)
			}
		}
	}
	if manifests == 0 {
		fatal(t, 49, "no tracked go.mod or package.json found")
	}
}

func parseAllowList(t *testing.T, content string) map[string]bool {
	t.Helper()
	allowed := map[string]bool{}
	for i, line := range lines(content) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || (fields[0] != "go" && fields[0] != "npm") {
			report(t, 49, "%s:%d: expected \"go <module>\" or \"npm <package>\", got %q", allowListFile, i+1, line)
			continue
		}
		allowed[fields[0]+" "+fields[1]] = true
	}
	return allowed
}

// goDirectDependencies returns the modules required without an indirect
// marker, prefixed with "go ".
func goDirectDependencies(content string) []string {
	var deps []string
	inBlock := false
	for _, raw := range lines(content) {
		line := strings.TrimSpace(raw)
		indirect := strings.Contains(line, "// indirect")
		if c := strings.Index(line, "//"); c >= 0 {
			line = strings.TrimSpace(line[:c])
		}
		var fields []string
		switch {
		case line == "require (":
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case inBlock:
			fields = strings.Fields(line)
		case strings.HasPrefix(line, "require "):
			fields = strings.Fields(strings.TrimPrefix(line, "require "))
		default:
			continue
		}
		if len(fields) >= 1 && !indirect {
			deps = append(deps, "go "+fields[0])
		}
	}
	return deps
}

// npmDirectDependencies returns every package named in a dependency section of
// package.json, prefixed with "npm ".
func npmDirectDependencies(t *testing.T, file, content string) []string {
	t.Helper()
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		fatal(t, 49, "%s does not parse: %v", file, err)
	}
	var deps []string
	for _, section := range []string{"dependencies", "devDependencies", "optionalDependencies", "peerDependencies"} {
		raw, ok := manifest[section]
		if !ok {
			continue
		}
		var names map[string]string
		if err := json.Unmarshal(raw, &names); err != nil {
			fatal(t, 49, "%s: %s does not parse: %v", file, section, err)
		}
		for name := range names {
			deps = append(deps, "npm "+name)
		}
	}
	sort.Strings(deps)
	return deps
}
