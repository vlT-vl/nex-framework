//go:build darwin

// Package system: native macOS trash emptying — direct filesystem
// manipulation of the well-known trash directories (pure Go, no cgo, no
// subprocess/AppleScript).
package system

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// nativeEmptyTrash empties the current user's trash on the boot volume
// (~/.Trash) plus, best-effort, the per-user trash on every other mounted
// volume (<volume>/.Trashes/<uid>) — the same locations Finder's "Empty
// Trash" clears. Returns the number of top-level items removed.
func nativeEmptyTrash() (int, error) {
	removed := 0
	var errs []string

	home, err := os.UserHomeDir()
	if err == nil {
		n, rerr := removeDirContents(filepath.Join(home, ".Trash"))
		removed += n
		if rerr != nil {
			errs = append(errs, macOSPermissionHint(rerr))
		}
	}

	uidDir := strconv.Itoa(os.Getuid())
	for _, vol := range nativeDiskInfo() {
		mount, _ := vol["mount"].(string)
		if mount == "" || mount == "/" {
			continue
		}
		trashDir := filepath.Join(mount, ".Trashes", uidDir)
		if _, statErr := os.Stat(trashDir); statErr != nil {
			continue
		}
		n, rerr := removeDirContents(trashDir)
		removed += n
		if rerr != nil {
			errs = append(errs, macOSPermissionHint(rerr))
		}
	}

	if len(errs) > 0 {
		return removed, errors.New(strings.Join(errs, "; "))
	}
	return removed, nil
}

// macOSPermissionHint turns the bare EPERM/EACCES macOS returns for
// TCC-protected folders (~/.Trash included, since Mojave-era privacy
// protections) into an actionable message. There is no code-level
// workaround: TCC access to protected user folders can only be granted by
// the user, in System Settings — this isn't a bug in nativeEmptyTrash.
func macOSPermissionHint(err error) string {
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return err.Error() + " (macOS is blocking access to the trash folder for this app — " +
			"grant it in System Settings → Privacy & Security → Full Disk Access, then try again)"
	}
	return err.Error()
}
