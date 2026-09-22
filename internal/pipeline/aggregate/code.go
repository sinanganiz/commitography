package aggregate

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/metrics/files"
)

// buildFiles runs the files family over the analysed commit's tree, as the
// replay stage listed it. This stage lists no tree and reads no file: working
// tree and repository access are the replay stage's alone (ADR-0020
// clause 3).
func (b *Builder) buildFiles(in core.Input, lineScoped []model.Commit) (core.Family[core.FilesMetrics], error) {
	if in.Replay == nil {
		return core.Family[core.FilesMetrics]{}, core.Internalf(nil,
			"aggregating without the replay stage's listing of the analysed commit's tree")
	}
	tree := files.Tree{Tracked: in.Replay.Tracked, TextFileCount: in.Replay.TextFileCount}
	return files.Build(in, lineScoped, tree), nil
}
