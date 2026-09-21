package collect

import (
	"context"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
)

// Normalization: the second half of the collect stage. The records git
// produced are given every per-commit definition of docs/metrics.md section 1
// — identity, analysed-commit membership, excluded paths, effective lines, the
// bulk flag, local time and the active date — here and only here (WP-0012).
//
// Author exclusion is decided per resolved identity, so the identity resolver
// runs inside this stage to decide membership. The artifact keeps the raw
// names and addresses beside what was decided, because it is internal working
// data (ADR-0033 clause 1) and a later stage resolves identities from it again.
//
// The artifact therefore depends on the analysis configuration, not only on
// the repository: exclusion lists, identity merges, merge counting, the date
// source and the outlier threshold all change it. Any cache of it must be keyed
// on the configuration digest (core.ConfigurationDigest) as well as on the
// repository and the analysed commit; WP-0033 builds that cache.

// attributesFile is the attributes file at the root of a tree.
const attributesFile = ".gitattributes"

// normalize annotates the records under the analysis plane, with the path
// filter the analysed commit's attributes give.
func normalize(commits []model.Commit, cfg config.Analysis, attributes []byte) ([]model.Commit, error) {
	pf, err := filter.NewPathFilterFromAttributes(cfg, attributes)
	if err != nil {
		// The pipeline root compiles the patterns before the stage runs and
		// refuses a bad one as the operator's to fix, so reaching this is a
		// defect rather than a configuration the operator wrote.
		return nil, core.Internalf(err, "building the path filter from the analysed commit's attributes")
	}
	resolver := identity.NewResolver(cfg, commits)
	return filter.Annotate(commits, cfg, resolver, pf), nil
}

// analysedAttributes returns the content of the analysed commit's root
// .gitattributes, read through git, or nothing where the commit has none.
//
// Attributes are repository content as of the analysed commit. A working-tree
// copy can carry uncommitted changes, and ADR-0020 clause 3 reserves
// working-tree access for replay, so the file is never read from disk.
func analysedAttributes(ctx context.Context, repoPath, commit string) ([]byte, error) {
	// ls-tree first, rather than asking for the blob and reading a failure as
	// absence: a failure is then always a failure. The object name is this
	// build's own value, from rev-parse; the path is the pathspec.
	entries, err := git.Records(ctx, git.At(repoPath, "ls-tree", "-z", commit).Pathspecs(attributesFile))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		meta, path, ok := strings.Cut(entry, "\t")
		if !ok || path != attributesFile {
			continue
		}
		// <mode> <type> <object>. Only a regular file is read as attributes,
		// as git itself reads them: a symbolic link, a directory or a
		// submodule of that name is not an attributes file.
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, nil
		}
		content, err := git.Output(ctx, git.At(repoPath, "cat-file", "blob", fields[2]))
		if err != nil {
			return nil, err
		}
		return []byte(content), nil
	}
	return nil, nil
}
