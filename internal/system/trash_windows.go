//go:build windows

// Package system: native Windows recycle bin emptying via SHEmptyRecycleBinW
// (shell32, already linked by launcher_windows.go) through syscall.LazyDLL —
// pure Go stdlib, no cgo, no subprocess (PowerShell Clear-RecycleBin).
package system

import "fmt"

// shell32 is already declared in launcher_windows.go (same package).
var procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")

const (
	sherbNoConfirmation = 0x00000001
	sherbNoProgressUI   = 0x00000002
	sherbNoSound        = 0x00000004
)

// nativeEmptyTrash empties the recycle bin across every drive (NULL root
// path). SHEmptyRecycleBinW doesn't report an item count, so the returned
// count is always -1 (meaning "not reported by this platform's API").
func nativeEmptyTrash() (int, error) {
	flags := uintptr(sherbNoConfirmation | sherbNoProgressUI | sherbNoSound)
	r, _, _ := procSHEmptyRecycleBinW.Call(0, 0, flags)
	// SHEmptyRecycleBinW returns an HRESULT; negative means failure. This
	// also returns success when the recycle bin is already empty.
	if hr := int32(r); hr < 0 {
		return 0, fmt.Errorf("SHEmptyRecycleBinW failed: HRESULT %#x", uint32(hr))
	}
	return -1, nil
}
