//go:build !windows

package server

import "path/filepath"

// resolvePath returns path with every symbolic link resolved.
func resolvePath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
