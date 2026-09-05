// Package version exposes build metadata injected at link time.
package version

import "fmt"

// Version is the semantic version, injected at build time.
var Version = "dev"

// Commit is the git commit hash of the build, injected at build time.
var Commit = "unknown"

// BuildDate is the RFC 3339 build timestamp, injected at build time.
var BuildDate = "unknown"

// String returns a single-line human-readable version string.
func String() string {
	return fmt.Sprintf("commitography %s (commit %s, built %s)", Version, Commit, BuildDate)
}
