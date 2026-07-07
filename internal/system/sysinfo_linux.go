//go:build linux

// Package system: the parts of Linux system info that still needed a
// subprocess (`uname -a`, `df -kP`) rewritten as pure Go stdlib (syscall.Uname,
// /proc/self/mountinfo + syscall.Statfs). CPU/memory/power/appearance info on
// Linux already came from /proc, /sys and env vars with no subprocess
// involved (see system.go) and are unchanged.
package system

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// nativeKernelInfo replaces `uname -a`.
func nativeKernelInfo() map[string]any {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return map[string]any{}
	}
	return map[string]any{
		"sysname":  int8sToString(uts.Sysname[:]),
		"nodename": int8sToString(uts.Nodename[:]),
		"release":  int8sToString(uts.Release[:]),
		"version":  int8sToString(uts.Version[:]),
		"machine":  int8sToString(uts.Machine[:]),
	}
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

// nativeDiskInfo replaces `df -kP` with /proc/self/mountinfo (to enumerate
// mount points, pure file read) + the statfs(2) syscall (stdlib, no cgo) per
// mount, the same pair of primitives `df` itself uses internally.
func nativeDiskInfo() []map[string]any {
	mounts := parseMountinfo("/proc/self/mountinfo")
	out := make([]map[string]any, 0, len(mounts))
	seen := map[string]bool{}
	for _, m := range mounts {
		if seen[m.mountPoint] {
			continue
		}
		seen[m.mountPoint] = true
		var stat syscall.Statfs_t
		if err := syscall.Statfs(m.mountPoint, &stat); err != nil {
			continue
		}
		if stat.Blocks == 0 {
			continue
		}
		bsize := uint64(stat.Bsize)
		out = append(out, map[string]any{
			"filesystem": m.source,
			"mount":      m.mountPoint,
			"size":       stat.Blocks * bsize,
			"available":  stat.Bavail * bsize,
			"used":       (stat.Blocks - stat.Bfree) * bsize,
		})
	}
	return out
}

type mountEntry struct {
	mountPoint string
	source     string
}

// parseMountinfo reads the mountinfo(5) format documented in proc(5): fields
// are whitespace-separated up to a literal "-" separator, after which the
// filesystem type and source device are the next two fields.
func parseMountinfo(path string) []mountEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []mountEntry
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		sepIdx := -1
		for i, f := range fields {
			if f == "-" {
				sepIdx = i
				break
			}
		}
		if sepIdx < 0 || sepIdx+2 >= len(fields) || len(fields) < 5 {
			continue
		}
		mountPoint := unescapeMountinfo(fields[4])
		source := unescapeMountinfo(fields[sepIdx+2])
		// Skip common virtual/pseudo filesystems that never carry real
		// usable capacity, matching what `df` normally excludes by default.
		fsType := fields[sepIdx+1]
		switch fsType {
		case "proc", "sysfs", "cgroup", "cgroup2", "devtmpfs", "devpts", "tmpfs",
			"securityfs", "pstore", "bpf", "debugfs", "tracefs", "mqueue", "hugetlbfs",
			"configfs", "fusectl", "autofs", "binfmt_misc", "overlay", "squashfs":
			continue
		}
		out = append(out, mountEntry{mountPoint: mountPoint, source: source})
	}
	return out
}

// unescapeMountinfo decodes the octal escapes (\040 for space, etc.) mountinfo
// uses for paths/devices containing whitespace.
func unescapeMountinfo(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			var n int
			for j := 1; j <= 3; j++ {
				n = n*8 + int(s[i+j]-'0')
			}
			b.WriteByte(byte(n))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// readKeyValueFile parses simple "key<sep>value" files such as
// /etc/os-release, /proc/cpuinfo and /proc/meminfo.
func readKeyValueFile(path, sep string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, sep)
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && out[k] == "" {
			out[k] = v
		}
	}
	return out
}

// nativePlatformInfo replaces `sw_vers`/`wmic os get Caption,...` on their
// respective platforms; on Linux, distro identity was always available from
// /etc/os-release (no subprocess involved).
func nativePlatformInfo() map[string]any {
	out := map[string]any{"os": "linux"}
	for k, v := range readKeyValueFile("/etc/os-release", "=") {
		out[strings.ToLower(k)] = strings.Trim(v, `"`)
	}
	return out
}

// nativeCPUInfo replaces `sysctl`/`wmic cpu get ...` on their respective
// platforms; on Linux, CPU model/core count were always available from
// /proc/cpuinfo (no subprocess involved).
func nativeCPUInfo() map[string]any {
	out := map[string]any{}
	cpu := readKeyValueFile("/proc/cpuinfo", ":")
	if v := cpu["model name"]; v != "" {
		out["model"] = v
	}
	if v := cpu["cpu cores"]; v != "" {
		out["coresPerSocket"] = v
	}
	return out
}

// nativeUptimeInfo replaces `sysctl -n kern.boottime`/`wmic os get
// LastBootUpTime` on their respective platforms; on Linux, uptime was always
// available from /proc/uptime (no subprocess involved).
func nativeUptimeInfo() map[string]any {
	out := map[string]any{}
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return out
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return out
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return out
	}
	out["seconds"] = seconds
	out["human"] = (time.Duration(seconds) * time.Second).String()
	return out
}

// nativeMemoryInfo replaces `sysctl -n hw.memsize`+`vm_stat`/`wmic OS get
// Free.../Total...` on their respective platforms; on Linux, memory stats
// were always available from /proc/meminfo (no subprocess involved).
func nativeMemoryInfo() map[string]any {
	out := map[string]any{}
	for k, v := range readKeyValueFile("/proc/meminfo", ":") {
		fields := strings.Fields(v)
		if len(fields) == 0 {
			continue
		}
		n, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			out[k] = v
			continue
		}
		if len(fields) > 1 && strings.EqualFold(fields[1], "kB") {
			n *= 1024
		}
		out[k] = n
	}
	return out
}

// nativeAppearanceInfo replaces `defaults read`/`reg query` on their
// respective platforms; on Linux there is no single system-wide dark-mode
// API without a desktop-portal round trip, so this reports the same
// GTK_THEME environment heuristic already used before (no subprocess
// involved either way).
func nativeAppearanceInfo() map[string]any {
	out := map[string]any{"theme": "unknown", "source": "environment"}
	if gtk := os.Getenv("GTK_THEME"); gtk != "" {
		out["gtkTheme"] = gtk
		if strings.Contains(strings.ToLower(gtk), "dark") {
			out["theme"] = "dark"
		} else {
			out["theme"] = "light"
		}
	}
	return out
}

// nativePowerInfo replaces `pmset -g batt`/`wmic path Win32_Battery get ...`
// on their respective platforms; on Linux, battery/power-supply info was
// always available from /sys/class/power_supply (no subprocess involved).
func nativePowerInfo() map[string]any {
	supplies, _ := filepath.Glob("/sys/class/power_supply/*")
	items := []map[string]any{}
	for _, dir := range supplies {
		item := map[string]any{"name": filepath.Base(dir)}
		for _, key := range []string{"type", "status", "capacity", "online"} {
			if b, err := os.ReadFile(filepath.Join(dir, key)); err == nil {
				item[key] = strings.TrimSpace(string(b))
			}
		}
		items = append(items, item)
	}
	return map[string]any{"powerSupply": items}
}
