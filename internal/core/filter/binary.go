package filter

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// BinarySniffBytes is how much of a file git inspects for a NUL byte when
// deciding whether it is binary, and so how much docs/metrics.md section 1
// inspects when deciding whether a file is a tracked text file.
const BinarySniffBytes = 8000

// BinaryRule is binary detection as docs/metrics.md section 1 defines it:
// git's rule, a NUL byte within the first BinarySniffBytes of the content,
// which a repository attribute may override. The attributes are those of the
// analysed commit's root .gitattributes, the file the path filter reads too.
//
// The `diff` attribute decides, as it does for git: unset, directly or through
// the built-in `binary` macro, the file is binary; set, it is text; given a
// driver's name, or left unspecified, the content decides. The last line that
// matches a path and names the attribute wins. A macro the file defines for
// itself is not expanded, and a driver's own `binary` setting, which lives in
// configuration rather than in the repository's content, is not read.
type BinaryRule struct {
	rules []diffRule
}

// diffRule is one line's say on the `diff` attribute.
type diffRule struct {
	pattern string
	state   diffState
}

type diffState int

const (
	// diffAuto leaves the decision to the content.
	diffAuto diffState = iota
	// diffBinary makes the file binary.
	diffBinary
	// diffText makes the file text.
	diffText
)

// NewBinaryRule reads the `diff` attribute from a .gitattributes file's
// content, empty where there is none.
func NewBinaryRule(attributes []byte) (*BinaryRule, error) {
	r := &BinaryRule{}
	scanner := bufio.NewScanner(bytes.NewReader(attributes))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[attr]") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pattern := attributePatternToGlob(fields[0])
		for _, attr := range fields[1:] {
			state, ok := diffStateOf(attr)
			if ok {
				r.rules = append(r.rules, diffRule{pattern: pattern, state: state})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading .gitattributes: %w", err)
	}
	return r, nil
}

// diffStateOf reads what one attribute says about `diff`, if anything.
func diffStateOf(attr string) (diffState, bool) {
	switch {
	case attr == "binary", attr == "-diff":
		return diffBinary, true
	case attr == "diff":
		return diffText, true
	case attr == "!diff", strings.HasPrefix(attr, "diff="):
		return diffAuto, true
	}
	return diffAuto, false
}

// IsBinary decides whether a file at path with the given content is binary.
// content need hold no more than the first BinarySniffBytes of the file.
func (r *BinaryRule) IsBinary(path string, content []byte) bool {
	state := diffAuto
	if r != nil {
		candidate := foldPath(path)
		for _, rule := range r.rules {
			if matches(rule.pattern, candidate) {
				state = rule.state
			}
		}
	}
	switch state {
	case diffBinary:
		return true
	case diffText:
		return false
	}
	if len(content) > BinarySniffBytes {
		content = content[:BinarySniffBytes]
	}
	return bytes.IndexByte(content, 0) >= 0
}
