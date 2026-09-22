package filter

import (
	"strings"
	"testing"
)

// Binary detection is git's rule: a NUL byte within the first 8 000 bytes,
// which the `diff` attribute overrides (docs/metrics.md section 1).
func TestReplayBinaryRule(t *testing.T) {
	t.Parallel()
	text := []byte("plain text\n")
	nul := []byte("PNG\x00\x01")
	late := []byte(strings.Repeat("a", BinarySniffBytes) + "\x00")
	attributes := []byte(strings.Join([]string{
		"# a comment, and a macro definition that is not expanded",
		"[attr]mine -diff",
		"*.dat binary",
		"*.raw -diff",
		"*.bytes diff",
		"docs/*.md -diff",
		"*.md !diff",
		"*.cpp diff=cpp",
		"*.old -diff",
		"legacy/*.old diff",
		"*.mine mine",
	}, "\n"))
	rule, err := NewBinaryRule(attributes)
	if err != nil {
		t.Fatalf("reading the attributes: %v", err)
	}
	for _, tc := range []struct {
		path    string
		content []byte
		want    bool
	}{
		{"a.txt", text, false},
		{"a.png", nul, true},
		{"late.txt", late, false},
		{"table.dat", text, true},
		{"deep/table.raw", text, true},
		{"image.bytes", nul, false},
		{"docs/guide.md", text, false},
		{"guide.md", nul, true},
		{"main.cpp", nul, true},
		{"main.cpp", text, false},
		{"notes.old", text, true},
		{"legacy/notes.old", nul, false},
		{"file.mine", text, false},
	} {
		if got := rule.IsBinary(tc.path, tc.content); got != tc.want {
			t.Errorf("IsBinary(%q, %q) = %v, want %v", tc.path, tc.content, got, tc.want)
		}
	}

	none, err := NewBinaryRule(nil)
	if err != nil {
		t.Fatalf("reading no attributes: %v", err)
	}
	if none.IsBinary("a.txt", text) || !none.IsBinary("a.png", nul) {
		t.Errorf("without attributes the content did not decide")
	}
}
