// Package filter removes from the analysis the commits and files that would
// otherwise distort it: merge commits, automation accounts, generated content,
// and one-off bulk imports.
package filter

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/sinanganiz/commitography/internal/config"
)

// caseInsensitiveFS mirrors git's own default: path comparisons ignore case on
// Windows and macOS-style filesystems, and respect it elsewhere.
var caseInsensitiveFS = runtime.GOOS == "windows"

// PathFilter decides which files are omitted from line-based metrics.
type PathFilter struct {
	exclude []string // glob patterns
	// negated holds patterns re-included by a `-linguist-generated` attribute,
	// which overrides an exclusion coming from .gitattributes.
	negated []string
	cache   map[string]bool
}

// NewPathFilter compiles the exclusion patterns and reads .gitattributes.
func NewPathFilter(cfg config.Config, repoPath string) (*PathFilter, error) {
	f := &PathFilter{cache: make(map[string]bool)}

	for _, pattern := range cfg.ExcludePaths {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if !doublestar.ValidatePattern(pattern) {
			return nil, fmt.Errorf("invalid exclude_paths pattern %q", pattern)
		}
		f.exclude = append(f.exclude, normalizePattern(pattern))
	}

	generated, reincluded, err := readGitAttributes(repoPath)
	if err != nil {
		return nil, err
	}
	f.exclude = append(f.exclude, generated...)
	f.negated = reincluded

	return f, nil
}

// Excluded reports whether a path should be omitted from line-based metrics.
// Results are memoized because the same paths recur across thousands of commits.
func (f *PathFilter) Excluded(path string) bool {
	if f == nil {
		return false
	}
	if cached, ok := f.cache[path]; ok {
		return cached
	}
	result := f.match(path)
	f.cache[path] = result
	return result
}

func (f *PathFilter) match(path string) bool {
	candidate := foldPath(path)
	for _, pattern := range f.negated {
		if matches(pattern, candidate) {
			return false
		}
	}
	for _, pattern := range f.exclude {
		if matches(pattern, candidate) {
			return true
		}
	}
	return false
}

func matches(pattern, path string) bool {
	ok, err := doublestar.Match(pattern, path)
	return err == nil && ok
}

func foldPath(path string) string {
	if caseInsensitiveFS {
		return strings.ToLower(path)
	}
	return path
}

func normalizePattern(pattern string) string {
	return foldPath(pattern)
}

// readGitAttributes extracts the paths marked linguist-generated, which GitHub
// itself treats as machine-written, and the paths that explicitly opt back in.
//
// Only the repository-root .gitattributes is consulted. Nested attribute files
// are rare in practice and walking for them would cost a full tree scan on
// every run.
func readGitAttributes(repoPath string) (generated, reincluded []string, err error) {
	path := filepath.Join(repoPath, ".gitattributes")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pattern := attributePatternToGlob(fields[0])
		for _, attr := range fields[1:] {
			switch {
			case attr == "linguist-generated", attr == "linguist-generated=true":
				generated = append(generated, pattern)
			case attr == "-linguist-generated", attr == "linguist-generated=false":
				reincluded = append(reincluded, pattern)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return generated, reincluded, nil
}

// attributePatternToGlob converts a gitattributes pattern into the doublestar
// form used everywhere else. A pattern without a slash applies at any depth,
// exactly as git treats it; a leading slash anchors to the repository root.
func attributePatternToGlob(pattern string) string {
	pattern = strings.Trim(pattern, `"`)
	switch {
	case strings.HasPrefix(pattern, "/"):
		pattern = strings.TrimPrefix(pattern, "/")
	case !strings.Contains(pattern, "/"):
		pattern = "**/" + pattern
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	return normalizePattern(pattern)
}
