//go:build windows

// Package system: native Windows kernel/CPU/uptime/memory/disk/appearance/
// power info via the Win32 API through syscall.LazyDLL — pure Go stdlib, no
// cgo, and no shell subprocess (wmic/ver/reg query/pmset-equivalent CLIs) is
// spawned.
package system

import (
	"syscall"
	"unsafe"
)

// kernel32 is already declared in clipboard_windows.go (same package); reused
// here instead of loading a second LazyDLL handle for it.
var (
	ntdll    = syscall.NewLazyDLL("ntdll.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procRtlGetVersion        = ntdll.NewProc("RtlGetVersion")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
	procGetLogicalDrives     = kernel32.NewProc("GetLogicalDrives")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")

	procRegOpenKeyExW  = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueW = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey    = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyClassesRoot  = 0x80000000
	hkeyCurrentUser  = 0x80000001
	hkeyLocalMachine = 0x80000002
	keyRead          = 0x20019
	regSZ            = 1
	regDWORD         = 4
)

type osVersionInfoExW struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion      uint32
	dwMinorVersion      uint32
	dwBuildNumber       uint32
	dwPlatformId        uint32
	szCSDVersion        [128]uint16
	wServicePackMajor   uint16
	wServicePackMinor   uint16
	wSuiteMask          uint16
	wProductType        uint8
	wReserved           uint8
}

// nativeKernelInfo / nativePlatformInfo replace `cmd /C ver` and
// `wmic os get Caption,Version,BuildNumber,OSArchitecture`, via
// RtlGetVersion (bypasses the GetVersionEx app-compat shim) plus the
// registry ProductName nex already needs no subprocess to read.
func windowsVersion() (osVersionInfoExW, bool) {
	var info osVersionInfoExW
	info.dwOSVersionInfoSize = uint32(unsafe.Sizeof(info))
	r, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	return info, r == 0
}

func nativeKernelInfo() map[string]any {
	out := map[string]any{"sysname": "Windows"}
	if v, ok := windowsVersion(); ok {
		out["release"] = uint32ToString(v.dwMajorVersion) + "." + uint32ToString(v.dwMinorVersion)
		out["build"] = v.dwBuildNumber
	}
	return out
}

func nativePlatformInfo() map[string]any {
	out := map[string]any{"os": "windows"}
	if v, ok := windowsVersion(); ok {
		out["majorVersion"] = v.dwMajorVersion
		out["minorVersion"] = v.dwMinorVersion
		out["buildNumber"] = v.dwBuildNumber
	}
	if name, ok := readRegistryString(hkeyLocalMachine, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName"); ok {
		out["productName"] = name
	}
	if disp, ok := readRegistryString(hkeyLocalMachine, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion"); ok {
		out["displayVersion"] = disp
	}
	return out
}

// nativeCPUInfo replaces `wmic cpu get Name,NumberOfCores,NumberOfLogicalProcessors`.
// Core/thread counts are already covered by runtime.NumCPU() in cpuInfo's
// shared "logical" field; only the brand string is added here.
func nativeCPUInfo() map[string]any {
	out := map[string]any{}
	if name, ok := readRegistryString(hkeyLocalMachine, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"); ok {
		out["model"] = name
	}
	return out
}

// nativeUptimeInfo replaces `wmic os get LastBootUpTime`.
func nativeUptimeInfo() map[string]any {
	r, _, _ := procGetTickCount64.Call()
	ms := uint64(r)
	seconds := ms / 1000
	out := map[string]any{
		"seconds": seconds,
		"human":   msToDuration(ms),
	}
	return out
}

type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// nativeMemoryInfo replaces
// `wmic OS get FreePhysicalMemory,TotalVisibleMemorySize,FreeVirtualMemory,TotalVirtualMemorySize`.
func nativeMemoryInfo() map[string]any {
	var m memoryStatusEx
	m.dwLength = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return map[string]any{}
	}
	return map[string]any{
		"MemTotal":          m.ullTotalPhys,
		"MemFree":           m.ullAvailPhys,
		"MemoryLoadPercent": m.dwMemoryLoad,
		"PageFileTotal":     m.ullTotalPageFile,
		"PageFileFree":      m.ullAvailPageFile,
		"VirtualTotal":      m.ullTotalVirtual,
		"VirtualFree":       m.ullAvailVirtual,
	}
}

// nativeDiskInfo replaces
// `wmic logicaldisk get Caption,FileSystem,FreeSpace,Size,VolumeName`.
func nativeDiskInfo() []map[string]any {
	out := []map[string]any{}
	r, _, _ := procGetLogicalDrives.Call()
	mask := uint32(r)
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		root := letter + `:\`
		rootPtr, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		var freeAvail, total, totalFree uint64
		ret, _, _ := procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&freeAvail)),
			uintptr(unsafe.Pointer(&total)),
			uintptr(unsafe.Pointer(&totalFree)),
		)
		if ret == 0 {
			continue
		}
		out = append(out, map[string]any{
			"filesystem": letter + ":",
			"mount":      root,
			"size":       total,
			"available":  freeAvail,
			"used":       total - totalFree,
		})
	}
	return out
}

type systemPowerStatus struct {
	acLineStatus        uint8
	batteryFlag         uint8
	batteryLifePercent  uint8
	reserved1           uint8
	batteryLifeTime     uint32
	batteryFullLifeTime uint32
}

// nativePowerInfo replaces `wmic path Win32_Battery get BatteryStatus,EstimatedChargeRemaining`.
func nativePowerInfo() map[string]any {
	var status systemPowerStatus
	r, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	if r == 0 {
		return map[string]any{}
	}
	out := map[string]any{
		"onACPower": status.acLineStatus == 1,
	}
	// batteryFlag 128 = "no system battery"; 255 = "unknown status".
	hasBattery := status.batteryFlag != 128 && status.batteryFlag != 255
	out["hasBattery"] = hasBattery
	if hasBattery {
		out["charging"] = status.batteryFlag&8 != 0
		if status.batteryLifePercent != 255 {
			out["percent"] = int(status.batteryLifePercent)
		}
	}
	return out
}

// nativeAppearanceInfo replaces
// `reg query HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize /v AppsUseLightTheme`.
func nativeAppearanceInfo() map[string]any {
	const source = "registry AppsUseLightTheme"
	v, ok := readRegistryDWORD(hkeyCurrentUser, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "AppsUseLightTheme")
	if !ok {
		return map[string]any{"theme": "light", "source": source}
	}
	if v == 0 {
		return map[string]any{"theme": "dark", "source": source}
	}
	return map[string]any{"theme": "light", "source": source}
}

func readRegistryString(hkey uintptr, subKey, valueName string) (string, bool) {
	var hKey syscall.Handle
	subKeyPtr, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return "", false
	}
	r, _, _ := procRegOpenKeyExW.Call(hkey, uintptr(unsafe.Pointer(subKeyPtr)), 0, keyRead, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return "", false
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	valueNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return "", false
	}
	var bufLen uint32
	var valType uint32
	r, _, _ = procRegQueryValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(valueNamePtr)), 0,
		uintptr(unsafe.Pointer(&valType)), 0, uintptr(unsafe.Pointer(&bufLen)))
	if r != 0 || valType != regSZ || bufLen == 0 {
		return "", false
	}
	buf := make([]uint16, bufLen/2+1)
	r, _, _ = procRegQueryValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(valueNamePtr)), 0,
		uintptr(unsafe.Pointer(&valType)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufLen)))
	if r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

// registryHasValue reports whether a named value exists under a key,
// regardless of its type/content — used for presence checks like the
// "URL Protocol" marker value on a registered scheme's registry key.
func registryHasValue(hkey uintptr, subKey, valueName string) bool {
	var hKey syscall.Handle
	subKeyPtr, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return false
	}
	r, _, _ := procRegOpenKeyExW.Call(hkey, uintptr(unsafe.Pointer(subKeyPtr)), 0, keyRead, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return false
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	valueNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return false
	}
	var bufLen, valType uint32
	r, _, _ = procRegQueryValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(valueNamePtr)), 0,
		uintptr(unsafe.Pointer(&valType)), 0, uintptr(unsafe.Pointer(&bufLen)))
	return r == 0
}

// nativeProtocolStatus replaces `reg query HKCU\Software\Classes\<scheme> /s`
// (and the HKCR fallback), checking for the standard "URL Protocol" marker
// value Windows uses to register a custom URL scheme handler.
func nativeProtocolStatus(scheme string) (bool, string, string) {
	roots := []struct {
		hkey uintptr
		path string
	}{
		{hkeyCurrentUser, `Software\Classes\` + scheme},
		{hkeyClassesRoot, scheme},
	}
	for _, root := range roots {
		if registryHasValue(root.hkey, root.path, "URL Protocol") {
			return true, root.path, "registry"
		}
	}
	return false, "", "registry"
}

// nativeFileAssociation replaces `reg query HKCU\Software\Classes\<ext> /ve`
// (and the HKCR fallback), reading the key's default (unnamed) value, which
// Windows uses to store the ProgID handling that extension.
func nativeFileAssociation(ext string) (bool, string, string, string) {
	roots := []struct {
		hkey uintptr
		path string
	}{
		{hkeyCurrentUser, `Software\Classes\` + ext},
		{hkeyClassesRoot, ext},
	}
	for _, root := range roots {
		if v, ok := readRegistryString(root.hkey, root.path, ""); ok && v != "" {
			return true, v, "", "registry"
		}
	}
	return false, "", "", "registry"
}

func readRegistryDWORD(hkey uintptr, subKey, valueName string) (uint32, bool) {
	var hKey syscall.Handle
	subKeyPtr, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return 0, false
	}
	r, _, _ := procRegOpenKeyExW.Call(hkey, uintptr(unsafe.Pointer(subKeyPtr)), 0, keyRead, uintptr(unsafe.Pointer(&hKey)))
	if r != 0 {
		return 0, false
	}
	defer procRegCloseKey.Call(uintptr(hKey))

	valueNamePtr, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return 0, false
	}
	var value uint32
	bufLen := uint32(unsafe.Sizeof(value))
	var valType uint32
	r, _, _ = procRegQueryValueW.Call(uintptr(hKey), uintptr(unsafe.Pointer(valueNamePtr)), 0,
		uintptr(unsafe.Pointer(&valType)), uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&bufLen)))
	if r != 0 || valType != regDWORD {
		return 0, false
	}
	return value, true
}

func uint32ToString(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func msToDuration(ms uint64) string {
	seconds := ms / 1000
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	out := ""
	if days > 0 {
		out += uint32ToString(uint32(days)) + "d"
	}
	if hours > 0 || days > 0 {
		out += uint32ToString(uint32(hours)) + "h"
	}
	out += uint32ToString(uint32(minutes)) + "m" + uint32ToString(uint32(secs)) + "s"
	return out
}
