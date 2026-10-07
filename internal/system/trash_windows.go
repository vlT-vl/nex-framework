//go:build windows

package system

import "fmt"

var procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")

const (
	sherbNoConfirmation = 0x00000001
	sherbNoProgressUI   = 0x00000002
	sherbNoSound        = 0x00000004
)

func nativeEmptyTrash() (int, error) {
	flags := uintptr(sherbNoConfirmation | sherbNoProgressUI | sherbNoSound)
	r, _, _ := procSHEmptyRecycleBinW.Call(0, 0, flags)

	if hr := int32(r); hr < 0 {
		return 0, fmt.Errorf("SHEmptyRecycleBinW failed: HRESULT %#x", uint32(hr))
	}
	return -1, nil
}
