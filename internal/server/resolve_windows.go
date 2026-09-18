package server

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// resolvePath returns the final path of an existing file or directory, with
// every symbolic link and junction resolved. A junction needs no privilege to
// create, and filepath.EvalSymlinks stops resolving junctions under the Go
// 1.23 winsymlink default, so the path check asks Windows for the final path
// rather than depending on the module's Go version.
func resolvePath(path string) (string, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", &fs.PathError{Op: "resolve", Path: path, Err: err}
	}
	// Backup semantics lets CreateFile open a directory; no access is requested.
	const fileFlagBackupSemantics = 0x02000000
	handle, err := syscall.CreateFile(name, 0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, fileFlagBackupSemantics, 0)
	if err != nil {
		return "", &fs.PathError{Op: "resolve", Path: path, Err: err}
	}
	defer syscall.CloseHandle(handle)

	// kernel32 is loaded in every Windows process, so resolving the procedure
	// on each call costs a lookup, not a load.
	getFinalPathNameByHandle := syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
	buffer := make([]uint16, syscall.MAX_PATH)
	for {
		n, _, callErr := getFinalPathNameByHandle.Call(uintptr(handle), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
		if n == 0 {
			return "", &fs.PathError{Op: "resolve", Path: path, Err: callErr}
		}
		if int(n) < len(buffer) {
			buffer = buffer[:n]
			break
		}
		buffer = make([]uint16, n)
	}

	final := syscall.UTF16ToString(buffer)
	switch {
	case strings.HasPrefix(final, `\\?\UNC\`):
		final = `\\` + final[len(`\\?\UNC\`):]
	case strings.HasPrefix(final, `\\?\`):
		final = final[len(`\\?\`):]
	}
	return filepath.Clean(final), nil
}
