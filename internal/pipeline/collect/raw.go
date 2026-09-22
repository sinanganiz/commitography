package collect

import (
	"fmt"
	"strings"

	"github.com/sinanganiz/commitography/internal/core/model"
)

// Raw diff entries: the part of the record stream that names each changed
// file's content, which replay reads through the object reader (ADR-0072,
// ADR-0073). Git writes one per changed file,
//
//	:<old mode> SP <new mode> SP <old object> SP <new object> SP <status>
//
// followed by the file's path, or by the path before and the path after for a
// rename. The entry itself holds nothing a repository controls: modes, object
// names and a status letter with an optional similarity score.

// submoduleMode is the mode git gives a path that holds a submodule. Its
// object name is a commit in another repository, not content of this one.
const submoduleMode = "160000"

// rawChange is one raw diff entry, with its paths.
type rawChange struct {
	oldMode, newMode string
	oldBlob, newBlob string
	status           byte
	// previousPath is the path before a rename, and empty otherwise; path is
	// the path after.
	previousPath, path string
}

// before is the object name of the file's content before the change, or empty
// where there was no file there.
func (r rawChange) before() string { return fileContent(r.oldMode, r.oldBlob) }

// after is the object name of the file's content after the change, or empty
// where there is no file there.
func (r rawChange) after() string { return fileContent(r.newMode, r.newBlob) }

// fileContent is an object name as a file's content: empty for the all-zero
// name git writes for an absent side, and for a submodule.
func fileContent(mode, object string) string {
	if mode == submoduleMode || strings.Trim(object, "0") == "" {
		return ""
	}
	return object
}

// source is the path the file had before the change.
func (r rawChange) source() string {
	if r.previousPath != "" {
		return r.previousPath
	}
	return r.path
}

// parseRaw reads one raw entry's fields and returns it with the number of path
// records that follow it. The count is taken from the status letter even when
// the rest does not parse, so the records after a malformed entry are still
// read as its paths rather than as anything they resemble.
func parseRaw(chunk string) (rawChange, int, bool) {
	fields := strings.Split(strings.TrimPrefix(chunk, ":"), " ")
	status := fields[len(fields)-1]
	paths := 1
	if status != "" && (status[0] == 'R' || status[0] == 'C') {
		paths = 2
	}
	if len(fields) != 5 || !isMode(fields[0]) || !isMode(fields[1]) || !isObjectName(fields[2]) ||
		!isObjectName(fields[3]) || !isStatus(status) {
		return rawChange{}, paths, false
	}
	return rawChange{
		oldMode: fields[0], newMode: fields[1],
		oldBlob: fields[2], newBlob: fields[3],
		status: status[0],
	}, paths, true
}

// pairBlobs gives each line-count entry the object names of its raw entry. Git
// writes both lists from one diff, in one order, so they pair by position; an
// entry whose paths differ from its partner's, or a list longer than the
// other, means the stream is not the one this parser reads. It returns why, or
// the empty string.
func pairBlobs(files []model.FileChange, raws []rawChange) string {
	if len(files) != len(raws) {
		return fmt.Sprintf("%d line-count entries and %d raw entries", len(files), len(raws))
	}
	for i := range files {
		r := raws[i]
		if files[i].Path != r.path || files[i].PreviousPath != r.previousPath {
			return fmt.Sprintf("entry %d names different paths in its line counts and its raw entry", i+1)
		}
		files[i].OldBlob, files[i].NewBlob = r.before(), r.after()
	}
	return ""
}

// isMode reports whether s is a mode as git writes it: six octal digits.
func isMode(s string) bool {
	if len(s) != 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '7' {
			return false
		}
	}
	return true
}

// isObjectName reports whether s is a full object name: 40 lowercase
// hexadecimal digits for SHA-1, 64 for SHA-256.
func isObjectName(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// isStatus reports whether s is a raw status: one of git's letters, followed
// by a similarity score for a rename or a copy.
func isStatus(s string) bool {
	if s == "" || !strings.ContainsRune("ACDMRTUX", rune(s[0])) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
