//go:build windows

package system

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32Screen            = syscall.NewLazyDLL("user32.dll")
	procEnumDisplayMonitors = user32Screen.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32Screen.NewProc("GetMonitorInfoW")
	screenMu                sync.Mutex
	screenEnumResult        []map[string]any
)

type winRect struct {
	left, top, right, bottom int32
}

type monitorInfoExW struct {
	cbSize    uint32
	rcMonitor winRect
	rcWork    winRect
	dwFlags   uint32
	szDevice  [32]uint16
}

const monitorInfoFPrimary = 0x1

func monitorEnumProc(hMonitor, _hdcMonitor uintptr, lprcMonitor uintptr, _dwData uintptr) uintptr {
	r := (*winRect)(unsafe.Pointer(lprcMonitor))

	var mi monitorInfoExW
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))
	primary := mi.dwFlags&monitorInfoFPrimary != 0

	screenEnumResult = append(screenEnumResult, map[string]any{
		"x":       int(r.left),
		"y":       int(r.top),
		"width":   int(r.right - r.left),
		"height":  int(r.bottom - r.top),
		"primary": primary,
	})
	return 1 // continue enumeration
}

// nativeScreenInfo replaces
// `wmic desktopmonitor get Name,ScreenWidth,ScreenHeight`.
func nativeScreenInfo() []map[string]any {
	screenMu.Lock()
	defer screenMu.Unlock()
	screenEnumResult = nil
	cb := syscall.NewCallback(monitorEnumProc)
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	out := screenEnumResult
	screenEnumResult = nil
	if out == nil {
		out = []map[string]any{}
	}
	return out
}
