// Link-time build metadata (ADR-0049 clause 6, ADR-0061).
//
// This is the one file in the repository that declares package-level
// variables for the linker to set, and the one file that suppresses the
// package-variable lint rule (ADR-0061 clause 2). Nothing writes these
// variables at runtime: the command reads them once at composition and
// injects them onward (ADR-0061 clause 4). They may appear only in a report's
// generation metadata, never in a metric, a cache key or a golden comparison
// (ADR-0061 clause 6).

package core

// The Makefile and the release configuration set these with -ldflags -X.
// version is the semantic version, commit the git commit of the build, and
// buildDate the built commit's own timestamp, never the clock (ADR-0063
// clause 3).
//
//nolint:gochecknoglobals // ADR-0061: link-time build metadata, never written at runtime.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// BuildMetadata returns the version, commit and build date set at link time,
// in that order.
func BuildMetadata() (string, string, string) {
	return version, commit, buildDate
}
