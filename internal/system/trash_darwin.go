//go:build darwin

package system

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

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

func macOSPermissionHint(err error) string {
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return err.Error() + " (macOS is blocking access to the trash folder for this app — " +
			"grant it in System Settings → Privacy & Security → Full Disk Access, then try again)"
	}
	return err.Error()
}
