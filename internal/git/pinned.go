// The pinned output configuration of ADR-0071: every git configuration key
// that can change the output the analysis parses is set on every invocation,
// here and nowhere else, so that no caller can omit one (clause 2).
//
// The operator's global configuration is still read, because ADR-0016
// delegates credentials to it, and several of its keys move numbers: two
// machines analysing one commit would otherwise produce two reports, which
// ADR-0021 clause 4 forbids. A value given on the command line overrides the
// same key in every configuration file, global and repository-local alike.
// Keys that affect only authentication, transport or credentials are not
// pinned (ADR-0071 clause 4).
//
// TestPinnedConfigSurvivesAHostileGlobalConfiguration in internal/checks runs
// an analysis under a global configuration setting every key below to
// something else, and requires the same report (clause 5).

package git

// pinnedFlags are the configuration flags ADR-0071 pins, placed before the
// subcommand of every invocation. They come after a call site's own settings,
// so that a setting cannot replace one: git takes the last occurrence of a key.
func pinnedFlags() []string {
	return []string{
		// Rename detection is on and finds renames, not copies: a file moved
		// without change is one entry with no lines rather than a removal and
		// an addition (docs/metrics.md section 1). A global diff.renames can
		// turn it off or widen it to copies.
		"-c", "diff.renames=true",

		// The rename limit bounds how many files rename detection compares.
		// 1000 is git's own default, whose value has changed between releases;
		// pinned, a git upgrade cannot move a number either.
		"-c", "diff.renameLimit=1000",

		// The diff algorithm decides which lines a change adds and removes,
		// and therefore every line count.
		"-c", "diff.algorithm=myers",

		// Paths are emitted raw and UTF-8 rather than C-quoted, so a path is
		// read exactly as the repository holds it. With NUL-delimited output
		// there is nothing for quoting to protect, and quoting would corrupt a
		// name that legitimately contains a quote.
		"-c", "core.quotePath=false",

		// The global attributes file can mark any path binary, which removes
		// its lines. Git for Windows maps /dev/null onto NUL. The repository's
		// own attributes are content, and apply (docs/metrics.md section 1).
		"-c", "core.attributesFile=/dev/null",

		// Found to move parsed output, and pinned for that reason (ADR-0071
		// clause 3, last sentence).
		//
		// The root commit's file entries: without them the first commit of
		// every repository changes nothing.
		"-c", "log.showRoot=true",
		// Signature verification writes into the log's own output.
		"-c", "log.showSignature=false",
		// Subjects and names are re-encoded to it.
		"-c", "i18n.logOutputEncoding=UTF-8",
		// An order file reorders a commit's file entries. An empty file keeps
		// git's own order; an empty value would be refused.
		"-c", "diff.orderFile=/dev/null",
		// An operator's own mailmap would rename the repository's authors. The
		// blob is git's default for a bare repository, and in a working copy
		// is the analysed commit's own mailmap, which the working copy's file
		// already carries.
		"-c", "mailmap.file=",
		"-c", "mailmap.blob=HEAD:.mailmap",
	}
}

// pinnedEnvironment is the environment ADR-0071 pins. The system attributes
// file is the machine-wide counterpart of the global one: an installation can
// ship one, and it would move the same numbers. Git has no configuration key
// for it, only this variable.
func pinnedEnvironment() []string {
	return []string{"GIT_ATTR_NOSYSTEM=1"}
}
