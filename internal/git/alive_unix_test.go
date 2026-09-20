//go:build !windows

package git

import "syscall"

// processAlive reports whether a process identifier still names a running
// process. Signal zero performs the permission and existence checks without
// delivering anything, which is the portable Unix way to ask.
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
