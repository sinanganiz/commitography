// Container detection reports whether the process runs inside a container, so
// messages can explain causes that only a container has, such as a folder
// that was never mounted.

package server

import "github.com/sinanganiz/commitography/internal/core"

// Running reports whether the process runs inside a Docker or Podman
// container, by looking for the marker files both create in every container.
// The entry points ask once, at composition, and pass the answer on.
func Running(files core.Filesystem) bool {
	for _, marker := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := files.Stat(marker); err == nil {
			return true
		}
	}
	return false
}
