//go:build windows

// Package system: native Windows "open with default app" / "reveal in
// Explorer" via the shell32 Win32 API through syscall.LazyDLL — pure Go
// stdlib, no cgo. No `rundll32 url.dll,FileProtocolHandler` or
// `explorer /select,` subprocess is spawned: ShellExecuteW asks the shell to
// open the target directly, and SHOpenFolderAndSelectItems messages the
// already-running Explorer process over COM instead of launching a new one.
package system

import (
	"errors"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	shell32 = syscall.NewLazyDLL("shell32.dll")
	ole32   = syscall.NewLazyDLL("ole32.dll")

	procShellExecuteW              = shell32.NewProc("ShellExecuteW")
	procSHParseDisplayName         = shell32.NewProc("SHParseDisplayName")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
	procILFree                     = shell32.NewProc("ILFree")
	procCoInitializeEx             = ole32.NewProc("CoInitializeEx")
	procCoUninitialize             = ole32.NewProc("CoUninitialize")
)

const (
	swShowNormal          = 1
	coInitApartmentThread = 0x2
)

// nativeOpenTarget replaces `rundll32 url.dll,FileProtocolHandler <target>`.
func nativeOpenTarget(target string) error {
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	verbPtr, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	r, _, _ := procShellExecuteW.Call(0, uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(targetPtr)), 0, 0, swShowNormal)
	// ShellExecuteW returns a value > 32 on success; <= 32 is an error code.
	if r <= 32 {
		return errors.New("ShellExecuteW failed")
	}
	return nil
}

// nativeRevealInFolder replaces `explorer /select,<path>`. Building/showing a
// PIDL selection requires COM, which is apartment-threaded per OS thread, so
// this locks the calling goroutine to its current OS thread for the
// init/call/uninit sequence.
func nativeRevealInFolder(path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	pathPtr, err := syscall.UTF16PtrFromString(abs)
	if err != nil {
		return err
	}

	procCoInitializeEx.Call(0, coInitApartmentThread)
	defer procCoUninitialize.Call()

	var pidl uintptr
	r, _, _ := procSHParseDisplayName.Call(uintptr(unsafe.Pointer(pathPtr)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
	if r != 0 || pidl == 0 {
		return errors.New("SHParseDisplayName failed")
	}
	defer procILFree.Call(pidl)

	r, _, _ = procSHOpenFolderAndSelectItems.Call(pidl, 0, 0, 0)
	if r != 0 {
		return errors.New("SHOpenFolderAndSelectItems failed")
	}
	return nil
}
