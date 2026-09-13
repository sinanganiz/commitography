// Package container detects whether the process runs inside a container, so
// messages can explain causes that only a container has, such as a folder
// that was never mounted.
package container

import "os"

// markers are the files Docker and Podman create in every container.
var markers = []string{"/.dockerenv", "/run/.containerenv"}

// Running reports whether the process runs inside a Docker or Podman
// container.
func Running() bool {
	for _, marker := range markers {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}
