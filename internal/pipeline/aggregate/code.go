package aggregate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/metrics/files"
)

// binarySniffBytes is how much of a file is inspected for a NUL byte when
// deciding whether it is a tracked text file (docs/metrics.md section 1).
const binarySniffBytes = 8000

// buildFiles lists the tree at the analysed commit and runs the files family
// over it.
//
// Known deviation from ADR-0020 clause 3, which gives working tree access to
// the replay stage alone: this stage still lists the tracked files through git
// and reads working tree files to detect binaries. WP-0013 removes the
// deviation.
func (b *Builder) buildFiles(in core.Input, lineScoped []model.Commit) (core.Family[core.FilesMetrics], []string, error) {
	var warnings []string
	tracked, err := git.LinesContext(contextOf(in), in.RepoPath, "ls-tree", "-r", "--name-only", "HEAD")
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return core.Family[core.FilesMetrics]{}, nil, err
		}
		warnings = append(warnings, fmt.Sprintf("could not list tracked files: %v", err))
	}

	textFiles := 0
	for _, p := range files.Included(in, tracked) {
		if !b.looksBinary(filepath.Join(in.RepoPath, filepath.FromSlash(p))) {
			textFiles++
		}
	}
	return files.Build(in, lineScoped, files.Tree{Tracked: tracked, TextFileCount: textFiles}), warnings, nil
}

// looksBinary reports whether a file carries a NUL byte near its start, or is
// absent from the working tree.
func (b *Builder) looksBinary(absPath string) bool {
	f, err := b.files.Open(absPath)
	if err != nil {
		// Present in HEAD but missing from the working tree, as with a sparse
		// checkout. Leave it out rather than guess at its contents.
		return true
	}
	defer f.Close()

	buf := make([]byte, binarySniffBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return true
	}
	return bytes.IndexByte(buf[:n], 0) >= 0
}
