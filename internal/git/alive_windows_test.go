//go:build windows

package git

import "syscall"

// processAlive reports whether a process identifier still names a running
// process.
//
// Windows has no signal to ask with, so the process object is opened and
// waited on for no time at all: a timeout means it has not been signalled and
// is therefore still running, and anything else — signalled, or no such
// process — means it is gone.
func processAlive(pid int) bool {
	const (
		synchronize = 0x00100000
		waitTimeout = 0x00000102
	)
	handle, err := syscall.OpenProcess(synchronize, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	event, err := syscall.WaitForSingleObject(handle, 0)
	return err == nil && event == waitTimeout
}
