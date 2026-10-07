//go:build linux

package system

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func nativeEmptyTrash() (int, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return 0, err
		}
		base = filepath.Join(home, ".local", "share")
	}
	trash := filepath.Join(base, "Trash")

	filesRemoved, filesErr := removeDirContents(filepath.Join(trash, "files"))
	_, infoErr := removeDirContents(filepath.Join(trash, "info"))

	var errs []string
	if filesErr != nil {
		errs = append(errs, filesErr.Error())
	}
	if infoErr != nil {
		errs = append(errs, infoErr.Error())
	}
	if len(errs) > 0 {
		return filesRemoved, errors.New(strings.Join(errs, "; "))
	}
	return filesRemoved, nil
}
