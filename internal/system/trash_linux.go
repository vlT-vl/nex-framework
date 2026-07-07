//go:build linux

// Package system: native Linux trash emptying — direct filesystem
// manipulation of the XDG Trash spec directories (pure Go, no cgo, no
// subprocess).
package system

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// nativeEmptyTrash empties the current user's home trash
// ($XDG_DATA_HOME/Trash, defaulting to ~/.local/share/Trash), per the
// freedesktop.org Trash specification: trashed items live under files/ with
// matching *.trashinfo metadata under info/. Per-mount-point trash
// directories (.Trash-<uid> on other filesystems) are not covered by this
// first pass. Returns the number of top-level files/ entries removed.
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
