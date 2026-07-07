//go:build darwin

// Package system: native macOS system/kernel/CPU/uptime/disk info via
// sysctlbyname(3) (cgo) and getfsstat(2) (stdlib syscall) — no shell
// subprocess (sw_vers/sysctl/uname/vm_stat/df CLIs) is spawned.
package system

/*
#include <sys/sysctl.h>
#include <sys/time.h>
#include <string.h>
#include <stdlib.h>

static char *nexSysctlString(const char *name) {
    size_t size = 0;
    if (sysctlbyname(name, NULL, &size, NULL, 0) != 0) return NULL;
    char *buf = malloc(size + 1);
    if (!buf) return NULL;
    if (sysctlbyname(name, buf, &size, NULL, 0) != 0) {
        free(buf);
        return NULL;
    }
    buf[size] = '\0';
    return buf;
}

static long long nexSysctlInt64(const char *name, int *ok) {
    long long value = 0;
    size_t size = sizeof(value);
    *ok = (sysctlbyname(name, &value, &size, NULL, 0) == 0) ? 1 : 0;
    return value;
}

static int nexSysctlInt32(const char *name, int *ok) {
    int value = 0;
    size_t size = sizeof(value);
    *ok = (sysctlbyname(name, &value, &size, NULL, 0) == 0) ? 1 : 0;
    return value;
}

static long long nexBootTimeUnix(int *ok) {
    struct timeval tv;
    size_t size = sizeof(tv);
    *ok = (sysctlbyname("kern.boottime", &tv, &size, NULL, 0) == 0) ? 1 : 0;
    return (long long)tv.tv_sec;
}
*/
import "C"

import (
	"syscall"
	"time"
	"unsafe"
)

// mntNowait mirrors BSD's MNT_NOWAIT mount(2) flag (<sys/mount.h>), which the
// syscall package does not export by name on darwin.
const mntNowait = 2

func sysctlString(name string) (string, bool) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	cstr := C.nexSysctlString(cname)
	if cstr == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), true
}

func sysctlInt64(name string) (int64, bool) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	var ok C.int
	v := C.nexSysctlInt64(cname, &ok)
	return int64(v), ok != 0
}

func sysctlInt32(name string) (int32, bool) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	var ok C.int
	v := C.nexSysctlInt32(cname, &ok)
	return int32(v), ok != 0
}

func bootTimeUnix() (int64, bool) {
	var ok C.int
	v := C.nexBootTimeUnix(&ok)
	return int64(v), ok != 0
}

// nativeKernelInfo replaces `uname -a` + `sysctl -n kern.osrelease/kern.version`.
func nativeKernelInfo() map[string]any {
	out := map[string]any{"sysname": "Darwin"}
	if v, ok := sysctlString("kern.osrelease"); ok {
		out["release"] = v
	}
	if v, ok := sysctlString("kern.version"); ok {
		out["version"] = v
	}
	if v, ok := sysctlString("hw.machine"); ok {
		out["machine"] = v
	}
	return out
}

// nativePlatformInfo replaces `sw_vers` + `sysctl -n hw.model`.
func nativePlatformInfo() map[string]any {
	out := map[string]any{"os": "darwin"}
	if v, ok := sysctlString("kern.osproductversion"); ok {
		out["productVersion"] = v
	}
	if v, ok := sysctlString("hw.model"); ok {
		out["model"] = v
	}
	return out
}

// nativeCPUInfo replaces `sysctl -n machdep.cpu.brand_string/hw.physicalcpu/hw.logicalcpu`.
func nativeCPUInfo() map[string]any {
	out := map[string]any{}
	if v, ok := sysctlString("machdep.cpu.brand_string"); ok {
		out["model"] = v
	}
	if v, ok := sysctlInt32("hw.physicalcpu"); ok {
		out["physical"] = v
	}
	if v, ok := sysctlInt32("hw.logicalcpu"); ok {
		out["logicalFromSysctl"] = v
	}
	return out
}

// nativeUptimeInfo replaces `sysctl -n kern.boottime`.
func nativeUptimeInfo() map[string]any {
	out := map[string]any{}
	if boot, ok := bootTimeUnix(); ok {
		seconds := time.Now().Unix() - boot
		out["bootUnix"] = boot
		out["seconds"] = seconds
		out["human"] = (time.Duration(seconds) * time.Second).String()
	}
	return out
}

// nativeMemoryInfo replaces `sysctl -n hw.memsize` + `vm_stat`. Detailed page
// statistics (free/active/inactive/wired) require the Mach host_statistics64
// API and are intentionally left out here — only total physical memory
// (unambiguously available via sysctlbyname) is reported.
func nativeMemoryInfo() map[string]any {
	out := map[string]any{}
	if v, ok := sysctlInt64("hw.memsize"); ok {
		out["MemTotal"] = uint64(v)
	}
	return out
}

// nativeDiskInfo replaces `df -kP` with the getfsstat(2) syscall (stdlib,
// no cgo) — the same syscall `df`/`mount` use internally to enumerate
// mounted filesystems.
func nativeDiskInfo() []map[string]any {
	n, err := syscall.Getfsstat(nil, mntNowait)
	if err != nil || n <= 0 {
		return []map[string]any{}
	}
	buf := make([]syscall.Statfs_t, n)
	n, err = syscall.Getfsstat(buf, mntNowait)
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, n)
	for _, fs := range buf[:n] {
		size := fs.Blocks * uint64(fs.Bsize)
		avail := fs.Bavail * uint64(fs.Bsize)
		out = append(out, map[string]any{
			"filesystem": int8sToString(fs.Mntfromname[:]),
			"mount":      int8sToString(fs.Mntonname[:]),
			"size":       size,
			"available":  avail,
			"used":       size - fs.Bfree*uint64(fs.Bsize),
		})
	}
	return out
}

func int8sToString(b []int8) string {
	buf := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}
