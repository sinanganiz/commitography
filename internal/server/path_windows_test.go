package server

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
)

// These tests cover the Windows path forms of WP-3.5 and WP-6.5: junctions,
// drive letter case, separators, extended-length and UNC paths.

func requireForbidden(t *testing.T, app *App, path string) {
	t.Helper()
	_, err := app.validateRepositoryPath(path, false)
	if err == nil {
		t.Fatalf("%s was accepted outside the allowed root", path)
	}
	switch reason := core.ReasonOf(err); reason {
	case core.ReasonPathOutsideAllowedRoots, core.ReasonPathNotFound:
	default:
		t.Fatalf("%s was refused with reason %q, want a path rejection", path, reason)
	}
}

// A junction needs no privilege on Windows, unlike a symbolic link, so it is
// the escape an unprivileged user can actually create. Go 1.23 stopped
// resolving junctions in filepath.EvalSymlinks by default; this test fails if
// that ever lets a junction lead out of the allowed root.
func TestWindowsJunctionOutOfTheRootIsRejected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside-repository")
	if out, err := git.Command("", "init", "-q", outside).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	link := filepath.Join(root, "junction")
	if err := createJunction(link, outside); err != nil {
		t.Skipf("creating a junction is unavailable: %v", err)
	}
	app, err := newAppWithRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.validateRepositoryPath(link, false)
	if got := core.ReasonOf(err); got != core.ReasonPathOutsideAllowedRoots {
		t.Fatalf("a junction to a repository outside the root was refused with %q, want %q: %v",
			got, core.ReasonPathOutsideAllowedRoots, err)
	}
}

func TestWindowsCaseAndSeparatorVariantsStayInsideTheRoot(t *testing.T) {
	t.Parallel()
	root := testRepoPath(t)
	app, err := newAppWithRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{
		strings.ToLower(root[:1]) + root[1:],
		strings.ToUpper(root),
		strings.ToLower(root),
		filepath.ToSlash(root),
		root + `\`,
		root + `\internal\..`,
	} {
		if _, err := app.validateRepositoryPath(variant, false); err != nil {
			t.Errorf("%q was refused inside the allowed root: %v", variant, err)
		}
	}
}

func TestWindowsParentTraversalLeavesTheRoot(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := git.Command("", "init", "-q", filepath.Join(parent, "sibling")).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	app, err := newAppWithRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		root + `\..\sibling`,
		filepath.ToSlash(root) + `/../sibling`,
		strings.ToUpper(root) + `\..\SIBLING`,
	} {
		requireForbidden(t, app, path)
	}
}

// An extended-length or UNC spelling must never reach outside the root. Both
// spellings of a path inside the root are refused too: the root is compared in
// the form it was given, so the dashboard documents typing that form.
func TestWindowsExtendedAndUNCPathsCannotLeaveTheRoot(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	outside := filepath.Join(parent, "outside")
	for _, dir := range []string{root, outside} {
		if out, err := git.Command("", "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init: %v: %s", err, out)
		}
	}
	app, err := newAppWithRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}

	requireForbidden(t, app, `\\?\`+outside)

	volume := filepath.VolumeName(outside)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skipf("temporary directory %s is not on a drive letter", outside)
	}
	unc := `\\localhost\` + volume[:1] + `$` + outside[len(volume):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("administrative share unavailable: %v", err)
	}
	requireForbidden(t, app, unc)
	inside := `\\localhost\` + volume[:1] + `$` + root[len(volume):]
	if _, err := app.validateRepositoryPath(inside, false); err == nil {
		t.Logf("the UNC spelling of the allowed root %s was accepted", inside)
	} else {
		t.Logf("the UNC spelling of the allowed root %s was refused: %v", inside, err)
	}
}

// Reparse point constants from the Windows SDK (winnt.h, winioctl.h).
const (
	ioReparseTagMountPoint = 0xA0000003
	fsctlSetReparsePoint   = 0x000900A4
)

// createJunction makes link a directory junction to target through the
// Windows API, as `mklink /J` does, without starting a process or a shell
// (ADR-0065 clause 4). The reparse data is a MOUNT_POINT_REPARSE_BUFFER whose
// substitute name is the NT path of target and whose print name is target.
func createJunction(link, target string) error {
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o755); err != nil {
		return err
	}

	substitute := utf16.Encode([]rune(`\??\` + target))
	print := utf16.Encode([]rune(target))
	const nul = 2 // bytes of one UTF-16 terminator
	pathBytes := 2*len(substitute) + nul + 2*len(print) + nul

	buf := make([]byte, 16+pathBytes)
	binary.LittleEndian.PutUint32(buf[0:], ioReparseTagMountPoint)
	binary.LittleEndian.PutUint16(buf[4:], uint16(8+pathBytes))            // ReparseDataLength
	binary.LittleEndian.PutUint16(buf[8:], 0)                              // SubstituteNameOffset
	binary.LittleEndian.PutUint16(buf[10:], uint16(2*len(substitute)))     // SubstituteNameLength
	binary.LittleEndian.PutUint16(buf[12:], uint16(2*len(substitute)+nul)) // PrintNameOffset
	binary.LittleEndian.PutUint16(buf[14:], uint16(2*len(print)))          // PrintNameLength
	offset := 16
	for _, u := range substitute {
		binary.LittleEndian.PutUint16(buf[offset:], u)
		offset += 2
	}
	offset += nul
	for _, u := range print {
		binary.LittleEndian.PutUint16(buf[offset:], u)
		offset += 2
	}

	name, err := syscall.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		_ = os.Remove(link)
		return err
	}
	defer syscall.CloseHandle(handle)

	var returned uint32
	if err := syscall.DeviceIoControl(handle, fsctlSetReparsePoint, &buf[0], uint32(len(buf)), nil, 0, &returned, nil); err != nil {
		_ = os.Remove(link)
		return err
	}
	return nil
}
