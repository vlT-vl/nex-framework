//go:build windows

// Package system: native Windows clipboard via the user32/kernel32 Win32
// clipboard API, called through syscall.LazyDLL — pure Go stdlib, no cgo and
// no shell subprocess (powershell Get-Clipboard/Set-Clipboard) is spawned.
package system

import (
	"errors"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
	procGlobalFree   = kernel32.NewProc("GlobalFree")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// openClipboardRetry retries OpenClipboard briefly: the clipboard is a single
// systemwide resource and can be transiently held by another process (e.g.
// another app mid-copy) even outside of any real contention in this app.
func openClipboardRetry() bool {
	for attempt := 0; attempt < 10; attempt++ {
		r, _, _ := procOpenClipboard.Call(0)
		if r != 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func nativeClipboardAvailable() (bool, string) {
	return true, "user32"
}

func nativeClipboardReadText() (string, bool, string, error) {
	if !openClipboardRetry() {
		return "", true, "user32", errors.New("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(uintptr(cfUnicodeText))
	if h == 0 {
		// No text on the clipboard — not an error.
		return "", true, "user32", nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", true, "user32", nil
	}
	defer procGlobalUnlock.Call(h)

	var chars []uint16
	for i := uintptr(0); ; i++ {
		c := *(*uint16)(unsafe.Pointer(p + i*2))
		if c == 0 {
			break
		}
		chars = append(chars, c)
	}
	return string(utf16.Decode(chars)), true, "user32", nil
}

func nativeClipboardWriteText(text string) (bool, string, error) {
	if !openClipboardRetry() {
		return false, "user32", errors.New("OpenClipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	encoded := utf16.Encode([]rune(text))
	encoded = append(encoded, 0) // null terminator

	size := uintptr(len(encoded) * 2)
	h, _, err := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return false, "user32", err
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return false, "user32", errors.New("GlobalLock failed")
	}
	for i, c := range encoded {
		*(*uint16)(unsafe.Pointer(p + uintptr(i)*2)) = c
	}
	procGlobalUnlock.Call(h)

	r, _, err := procSetClipboardData.Call(uintptr(cfUnicodeText), h)
	if r == 0 {
		procGlobalFree.Call(h)
		return false, "user32", err
	}
	// SetClipboardData succeeded: the system now owns h, do not free it.
	return true, "user32", nil
}
