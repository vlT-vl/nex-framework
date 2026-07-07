// Package system registers all sys.* RPC handlers: dialogs, filesystem,
// notifications, shell, window, OS info. Cross-platform via stdlib + zenity + exec.
package system

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ncruces/zenity"

	"nex/internal/core"
)

var (
	applicationMenu []byte
	menuMu          sync.RWMutex
	shortcutsMu     sync.RWMutex
	shortcuts       = map[string]shortcutEntry{}
	recentDocsMu    sync.Mutex
)

type shortcutEntry struct {
	ID          string `json:"id"`
	Accelerator string `json:"accelerator"`
	Label       string `json:"label,omitempty"`
	Scope       string `json:"scope"`
	Enabled     bool   `json:"enabled"`
}

func Register(reg core.Registrar) {
	// OS info
	reg.Register("sys.os.info", osInfo)
	reg.Register("sys.os.host", osHost)
	reg.Register("sys.os.user", osUser)
	reg.Register("sys.os.runtime", osRuntime)
	reg.Register("sys.os.process", osProcess)
	reg.Register("sys.os.network", osNetwork)
	reg.Register("sys.os.disks", osDisks)
	reg.Register("sys.os.memory", osMemory)
	reg.Register("sys.os.time", osTime)
	reg.Register("sys.screen.info", screenInfo)
	reg.Register("sys.env.paths", envPaths)

	// Dialogs (must run on main thread via c.OnMain)
	reg.Register("sys.dialog.open", dialogOpen)
	reg.Register("sys.dialog.save", dialogSave)
	reg.Register("sys.dialog.message", dialogMessage)
	reg.Register("sys.dialog.entry", dialogEntry)
	reg.Register("sys.dialog.password", dialogPassword)
	reg.Register("sys.dialog.color", dialogColor)
	reg.Register("sys.dialog.isSupported", dialogIsSupported)

	// Notifications
	reg.Register("sys.notify.toast", notifyToast)
	reg.Register("sys.notify.capabilities", notifyCapabilities)

	// Shell
	reg.Register("sys.shell.openURL", shellOpenURL)
	reg.Register("sys.shell.openPath", shellOpenPath)
	reg.Register("sys.shell.showInFolder", shellShowInFolder)
	reg.Register("sys.shell.exec", shellExec)
	reg.Register("sys.shell.run", shellExec)
	reg.Register("sys.shell.start", shellStart)

	// Window (dispatched to main thread by App)
	reg.Register("sys.window.info", windowInfo)
	reg.Register("sys.window.setTitle", windowSetTitle)
	reg.Register("sys.window.setSize", windowSetSize)
	reg.Register("sys.window.getSize", windowGetSize)
	reg.Register("sys.window.fullscreen", windowFullscreen)
	reg.Register("sys.window.unfullscreen", windowUnfullscreen)
	reg.Register("sys.window.reload", windowReload)
	reg.Register("sys.window.print", windowPrint)
	reg.Register("sys.window.eval", windowEval)

	// Clipboard
	reg.Register("sys.clipboard.readText", clipboardReadText)
	reg.Register("sys.clipboard.writeText", clipboardWriteText)
	reg.Register("sys.clipboard.clear", clipboardClear)
	reg.Register("sys.clipboard.formats", clipboardFormats)
	reg.Register("sys.clipboard.isAvailable", clipboardIsAvailable)

	// Logging
	reg.Register("sys.log.print", logPrint)
	reg.Register("sys.log.trace", logLevel("trace"))
	reg.Register("sys.log.debug", logLevel("debug"))
	reg.Register("sys.log.info", logLevel("info"))
	reg.Register("sys.log.warning", logLevel("warning"))
	reg.Register("sys.log.error", logLevel("error"))

	// Filesystem
	reg.Register("sys.fs.read", fsRead)
	reg.Register("sys.fs.write", fsWrite)
	reg.Register("sys.fs.list", fsList)
	reg.Register("sys.fs.exists", fsExists)
	reg.Register("sys.fs.stat", fsStat)
	reg.Register("sys.fs.mkdir", fsMkdir)
	reg.Register("sys.fs.remove", fsRemove)
	reg.Register("sys.fs.rename", fsRename)

	// App
	reg.Register("sys.app.quit", appQuit)
	reg.Register("sys.app.exit", appExit)
	reg.Register("sys.app.restart", appRestart)
	reg.Register("sys.app.paths", appPaths)
	reg.Register("sys.app.runtime", appRuntime)
	reg.Register("sys.menu.set", menuSet)
	reg.Register("sys.menu.update", menuUpdate)

	// Desktop integration surfaces with concrete runtime data.
	reg.Register("sys.appearance.info", appearanceInfo)
	reg.Register("sys.power.info", powerInfo)
	reg.Register("sys.shortcuts.register", shortcutsRegister)
	reg.Register("sys.shortcuts.unregister", shortcutsUnregister)
	reg.Register("sys.shortcuts.clear", shortcutsClear)
	reg.Register("sys.shortcuts.list", shortcutsList)
	reg.Register("sys.protocol.status", protocolStatus)
	reg.Register("sys.updater.check", updaterCheck)
	reg.Register("sys.secureStorage.isAvailable", secureStorageAvailable)
	reg.Register("sys.fileAssociations.status", fileAssociationsStatus)
	reg.Register("sys.recentDocs.add", recentDocsAdd)
	reg.Register("sys.recentDocs.clear", recentDocsClear)
	reg.Register("sys.recentDocs.list", recentDocsList)
	reg.Register("sys.trash.empty", trashEmpty)
}

// ---- OS Info ----------------------------------------------------------------

func osInfo(_ *core.Context, _ json.RawMessage) (any, error) {
	host := hostInfo()
	paths := pathsInfo()
	return map[string]any{
		"host":        host,
		"user":        userInfo(),
		"runtime":     runtimeInfo(),
		"process":     processInfo(),
		"paths":       paths,
		"network":     networkInfo(),
		"disks":       diskInfo(),
		"memory":      memoryInfo(),
		"time":        timeInfo(),
		"environment": environmentInfo(),

		// Kept for backward compatibility with the original compact response.
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"home":     paths["home"],
		"hostname": host["hostname"],
		"cpus":     runtime.NumCPU(),
	}, nil
}

func osHost(_ *core.Context, _ json.RawMessage) (any, error) {
	return hostInfo(), nil
}

func osUser(_ *core.Context, _ json.RawMessage) (any, error) {
	return userInfo(), nil
}

func osRuntime(_ *core.Context, _ json.RawMessage) (any, error) {
	return runtimeInfo(), nil
}

func osProcess(_ *core.Context, _ json.RawMessage) (any, error) {
	return processInfo(), nil
}

func osNetwork(_ *core.Context, _ json.RawMessage) (any, error) {
	return map[string]any{"interfaces": networkInfo()}, nil
}

func osDisks(_ *core.Context, _ json.RawMessage) (any, error) {
	return map[string]any{"volumes": diskInfo()}, nil
}

func osMemory(_ *core.Context, _ json.RawMessage) (any, error) {
	return memoryInfo(), nil
}

func osTime(_ *core.Context, _ json.RawMessage) (any, error) {
	return timeInfo(), nil
}

// screenInfo reports native monitor geometry via nativeScreenInfo, which is
// implemented per platform (cgo CoreGraphics on darwin, cgo GDK on linux,
// user32 EnumDisplayMonitors via syscall on windows) — no subprocess
// (system_profiler/xrandr/wmic) is spawned.
func screenInfo(_ *core.Context, _ json.RawMessage) (any, error) {
	return map[string]any{
		"platform": runtime.GOOS,
		"screens":  nativeScreenInfo(),
	}, nil
}

func envPaths(_ *core.Context, _ json.RawMessage) (any, error) {
	return pathsInfo(), nil
}

func hostInfo() map[string]any {
	home, _ := os.UserHomeDir()
	host, _ := os.Hostname()
	return map[string]any{
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"compiler":    runtime.Compiler,
		"goVersion":   runtime.Version(),
		"hostname":    host,
		"home":        home,
		"cpus":        runtime.NumCPU(),
		"kernel":      kernelInfo(),
		"platform":    platformInfo(),
		"cpu":         cpuInfo(),
		"uptime":      uptimeInfo(),
		"machineName": host,
	}
}

func pathsInfo() map[string]any {
	home, _ := os.UserHomeDir()
	cfg, _ := os.UserConfigDir()
	cache, _ := os.UserCacheDir()
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	return map[string]any{
		"home": home, "config": cfg, "cache": cache,
		"temp": os.TempDir(), "exe": exe, "cwd": cwd,
	}
}

func userInfo() map[string]any {
	out := map[string]any{}
	u, err := user.Current()
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	out["uid"] = u.Uid
	out["gid"] = u.Gid
	out["username"] = u.Username
	out["name"] = u.Name
	out["homeDir"] = u.HomeDir
	if gids, err := u.GroupIds(); err == nil {
		out["groupIds"] = gids
	}
	return out
}

func runtimeInfo() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	build := map[string]any{}
	if info, ok := debug.ReadBuildInfo(); ok {
		build["goVersion"] = info.GoVersion
		build["path"] = info.Path
		build["main"] = map[string]any{
			"path":    info.Main.Path,
			"version": info.Main.Version,
			"sum":     info.Main.Sum,
		}
		settings := map[string]string{}
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		build["settings"] = settings
	}

	return map[string]any{
		"goos":          runtime.GOOS,
		"goarch":        runtime.GOARCH,
		"compiler":      runtime.Compiler,
		"goVersion":     runtime.Version(),
		"numCPU":        runtime.NumCPU(),
		"gomaxprocs":    runtime.GOMAXPROCS(0),
		"goroutines":    runtime.NumGoroutine(),
		"cgoCalls":      runtime.NumCgoCall(),
		"mem":           memStatsMap(m),
		"build":         build,
		"defaultGOROOT": runtime.GOROOT(),
	}
}

func processInfo() map[string]any {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	return map[string]any{
		"pid":         os.Getpid(),
		"ppid":        os.Getppid(),
		"executable":  exe,
		"cwd":         cwd,
		"args":        os.Args,
		"pageSize":    os.Getpagesize(),
		"tempDir":     os.TempDir(),
		"environment": environmentInfo(),
	}
}

func environmentInfo() map[string]any {
	public := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(k, "VITE_") {
			public[k] = v
		}
	}
	return map[string]any{
		"count":        len(os.Environ()),
		"publicPrefix": "VITE_",
		"public":       public,
	}
}

func networkInfo() []map[string]any {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []map[string]any{{"error": err.Error()}}
	}
	out := make([]map[string]any, 0, len(ifaces))
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		addrList := []string{}
		if err == nil {
			for _, addr := range addrs {
				addrList = append(addrList, addr.String())
			}
		}
		out = append(out, map[string]any{
			"name":         iface.Name,
			"index":        iface.Index,
			"mtu":          iface.MTU,
			"flags":        iface.Flags.String(),
			"hardwareAddr": iface.HardwareAddr.String(),
			"addrs":        addrList,
		})
	}
	return out
}

func memoryInfo() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return map[string]any{
		"go":     memStatsMap(m),
		"system": systemMemoryInfo(),
	}
}

func diskInfo() []map[string]any {
	return nativeDiskInfo()
}

func timeInfo() map[string]any {
	now := time.Now()
	name, offset := now.Zone()
	return map[string]any{
		"local":          now.Format(time.RFC3339Nano),
		"utc":            now.UTC().Format(time.RFC3339Nano),
		"unix":           now.Unix(),
		"unixNano":       now.UnixNano(),
		"timezone":       name,
		"timezoneOffset": offset,
	}
}

func memStatsMap(m runtime.MemStats) map[string]any {
	return map[string]any{
		"alloc":         m.Alloc,
		"totalAlloc":    m.TotalAlloc,
		"sys":           m.Sys,
		"lookups":       m.Lookups,
		"mallocs":       m.Mallocs,
		"frees":         m.Frees,
		"heapAlloc":     m.HeapAlloc,
		"heapSys":       m.HeapSys,
		"heapIdle":      m.HeapIdle,
		"heapInuse":     m.HeapInuse,
		"heapReleased":  m.HeapReleased,
		"heapObjects":   m.HeapObjects,
		"stackInuse":    m.StackInuse,
		"stackSys":      m.StackSys,
		"nextGC":        m.NextGC,
		"lastGC":        m.LastGC,
		"pauseTotalNs":  m.PauseTotalNs,
		"numGC":         m.NumGC,
		"numForcedGC":   m.NumForcedGC,
		"gcCPUFraction": m.GCCPUFraction,
	}
}

// kernelInfo/platformInfo/cpuInfo/uptimeInfo/systemMemoryInfo/diskInfo all
// delegate to nativeXxx, implemented per platform (cgo sysctlbyname on
// darwin, syscall.LazyDLL on windows, /proc+syscall on linux — see
// sysinfo_darwin.go/sysinfo_windows.go/sysinfo_linux.go). No subprocess
// (uname/sysctl/sw_vers/vm_stat/df/wmic/cmd/reg/pmset/defaults CLIs) is
// spawned by any of them.

func kernelInfo() map[string]any {
	return nativeKernelInfo()
}

func platformInfo() map[string]any {
	return nativePlatformInfo()
}

func cpuInfo() map[string]any {
	out := map[string]any{"logical": runtime.NumCPU()}
	for k, v := range nativeCPUInfo() {
		out[k] = v
	}
	return out
}

func uptimeInfo() map[string]any {
	return nativeUptimeInfo()
}

func systemMemoryInfo() map[string]any {
	return nativeMemoryInfo()
}

// ---- Dialogs ----------------------------------------------------------------

type fileFilter struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns"`
}

func toZenityFilters(in []fileFilter) zenity.FileFilters {
	ff := zenity.FileFilters{}
	for _, f := range in {
		ff = append(ff, zenity.FileFilter{
			Name: f.Name, Patterns: f.Patterns, CaseFold: true,
		})
	}
	return ff
}

type dialogOptionParams struct {
	Title         string
	Filename      string
	Filters       []fileFilter
	ShowHidden    bool
	OKLabel       string
	CancelLabel   string
	ExtraButton   string
	DefaultCancel bool
	Width         uint
	Height        uint
}

func dialogOpts(p dialogOptionParams) []zenity.Option {
	opts := []zenity.Option{}
	if p.Title != "" {
		opts = append(opts, zenity.Title(p.Title))
	}
	if p.Filename != "" {
		opts = append(opts, zenity.Filename(p.Filename))
	}
	if len(p.Filters) > 0 {
		opts = append(opts, toZenityFilters(p.Filters))
	}
	if p.ShowHidden {
		opts = append(opts, zenity.ShowHidden())
	}
	if p.OKLabel != "" {
		opts = append(opts, zenity.OKLabel(p.OKLabel))
	}
	if p.CancelLabel != "" {
		opts = append(opts, zenity.CancelLabel(p.CancelLabel))
	}
	if p.ExtraButton != "" {
		opts = append(opts, zenity.ExtraButton(p.ExtraButton))
	}
	if p.DefaultCancel {
		opts = append(opts, zenity.DefaultCancel())
	}
	if p.Width > 0 {
		opts = append(opts, zenity.Width(p.Width))
	}
	if p.Height > 0 {
		opts = append(opts, zenity.Height(p.Height))
	}
	return opts
}

func dialogOpen(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title            string       `json:"title"`
		Multiple         bool         `json:"multiple"`
		Directory        bool         `json:"directory"`
		StartDir         string       `json:"startDir"`
		DefaultDirectory string       `json:"defaultDirectory"`
		DefaultFilename  string       `json:"defaultFilename"`
		ShowHidden       bool         `json:"showHidden"`
		OKLabel          string       `json:"okLabel"`
		CancelLabel      string       `json:"cancelLabel"`
		Width            uint         `json:"width"`
		Height           uint         `json:"height"`
		Filters          []fileFilter `json:"filters"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	var paths []string
	var err error
	c.OnMain(func() {
		filename := p.StartDir
		if filename == "" {
			filename = p.DefaultDirectory
		}
		if p.DefaultFilename != "" {
			filename = filepath.Join(filename, p.DefaultFilename)
		}
		opts := dialogOpts(dialogOptionParams{
			Title:       p.Title,
			Filename:    filename,
			Filters:     p.Filters,
			ShowHidden:  p.ShowHidden,
			OKLabel:     p.OKLabel,
			CancelLabel: p.CancelLabel,
			Width:       p.Width,
			Height:      p.Height,
		})
		switch {
		case p.Directory:
			var d string
			d, err = zenity.SelectFile(append(opts, zenity.Directory())...)
			if err == nil {
				paths = []string{d}
			}
		case p.Multiple:
			paths, err = zenity.SelectFileMultiple(opts...)
		default:
			var f string
			f, err = zenity.SelectFile(opts...)
			if err == nil {
				paths = []string{f}
			}
		}
	})
	if err == zenity.ErrCanceled {
		return map[string]any{"canceled": true, "paths": []string{}}, nil
	}
	if errors.Is(err, zenity.ErrUnsupported) {
		return nil, core.Errorf("dialog_unsupported", "file dialogs not available: install zenity or enable xdg-desktop-portal")
	}
	if err != nil {
		return nil, core.Errorf("dialog", "%v", err)
	}
	return map[string]any{"canceled": false, "paths": paths}, nil
}

func dialogSave(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title            string       `json:"title"`
		DefaultName      string       `json:"defaultName"`
		DefaultDirectory string       `json:"defaultDirectory"`
		DefaultFilename  string       `json:"defaultFilename"`
		Filters          []fileFilter `json:"filters"`
		ConfirmOverwrite bool         `json:"confirmOverwrite"`
		ShowHidden       bool         `json:"showHidden"`
		OKLabel          string       `json:"okLabel"`
		CancelLabel      string       `json:"cancelLabel"`
		Width            uint         `json:"width"`
		Height           uint         `json:"height"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	var out string
	var err error
	c.OnMain(func() {
		filename := p.DefaultName
		if p.DefaultFilename != "" {
			filename = p.DefaultFilename
		}
		if p.DefaultDirectory != "" && filename != "" {
			filename = filepath.Join(p.DefaultDirectory, filename)
		} else if p.DefaultDirectory != "" {
			filename = p.DefaultDirectory
		}
		opts := dialogOpts(dialogOptionParams{
			Title:       p.Title,
			Filename:    filename,
			Filters:     p.Filters,
			ShowHidden:  p.ShowHidden,
			OKLabel:     p.OKLabel,
			CancelLabel: p.CancelLabel,
			Width:       p.Width,
			Height:      p.Height,
		})
		if p.ConfirmOverwrite {
			opts = append(opts, zenity.ConfirmOverwrite())
		}
		out, err = zenity.SelectFileSave(opts...)
	})
	if err == zenity.ErrCanceled {
		return map[string]any{"canceled": true}, nil
	}
	if errors.Is(err, zenity.ErrUnsupported) {
		return nil, core.Errorf("dialog_unsupported", "file dialogs not available: install zenity or enable xdg-desktop-portal")
	}
	if err != nil {
		return nil, core.Errorf("dialog", "%v", err)
	}
	return map[string]any{"canceled": false, "path": out}, nil
}

func dialogMessage(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Level         string `json:"level"`
		Title         string `json:"title"`
		Text          string `json:"text"`
		OKLabel       string `json:"okLabel"`
		CancelLabel   string `json:"cancelLabel"`
		ExtraButton   string `json:"extraButton"`
		DefaultCancel bool   `json:"defaultCancel"`
		Width         uint   `json:"width"`
		Height        uint   `json:"height"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	var err error
	confirmed := true
	c.OnMain(func() {
		opts := dialogOpts(dialogOptionParams{
			Title:         p.Title,
			OKLabel:       p.OKLabel,
			CancelLabel:   p.CancelLabel,
			ExtraButton:   p.ExtraButton,
			DefaultCancel: p.DefaultCancel,
			Width:         p.Width,
			Height:        p.Height,
		})
		switch p.Level {
		case "error":
			err = zenity.Error(p.Text, opts...)
		case "warning":
			err = zenity.Warning(p.Text, opts...)
		case "question":
			err = zenity.Question(p.Text, opts...)
			if err == zenity.ErrCanceled {
				confirmed = false
				err = nil
			}
		default:
			err = zenity.Info(p.Text, opts...)
		}
	})
	if err != nil && err != zenity.ErrCanceled {
		if err == zenity.ErrExtraButton {
			return map[string]any{"confirmed": false, "button": p.ExtraButton}, nil
		}
		return nil, core.Errorf("dialog", "%v", err)
	}
	button := p.OKLabel
	if button == "" {
		button = "ok"
	}
	if !confirmed {
		button = p.CancelLabel
		if button == "" {
			button = "cancel"
		}
	}
	return map[string]any{"confirmed": confirmed, "button": button}, nil
}

func dialogEntry(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title         string `json:"title"`
		Text          string `json:"text"`
		DefaultText   string `json:"defaultText"`
		HideText      bool   `json:"hideText"`
		OKLabel       string `json:"okLabel"`
		CancelLabel   string `json:"cancelLabel"`
		ExtraButton   string `json:"extraButton"`
		DefaultCancel bool   `json:"defaultCancel"`
		Width         uint   `json:"width"`
		Height        uint   `json:"height"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	var value string
	var err error
	c.OnMain(func() {
		opts := dialogOpts(dialogOptionParams{
			Title:         p.Title,
			OKLabel:       p.OKLabel,
			CancelLabel:   p.CancelLabel,
			ExtraButton:   p.ExtraButton,
			DefaultCancel: p.DefaultCancel,
			Width:         p.Width,
			Height:        p.Height,
		})
		if p.DefaultText != "" {
			opts = append(opts, zenity.EntryText(p.DefaultText))
		}
		if p.HideText {
			opts = append(opts, zenity.HideText())
		}
		value, err = zenity.Entry(p.Text, opts...)
	})
	if err == zenity.ErrCanceled {
		return map[string]any{"canceled": true, "value": ""}, nil
	}
	if err == zenity.ErrExtraButton {
		return map[string]any{"canceled": false, "button": p.ExtraButton, "value": value}, nil
	}
	if err != nil {
		return nil, core.Errorf("dialog", "%v", err)
	}
	return map[string]any{"canceled": false, "value": value}, nil
}

func dialogPassword(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title       string `json:"title"`
		Username    bool   `json:"username"`
		OKLabel     string `json:"okLabel"`
		CancelLabel string `json:"cancelLabel"`
		ExtraButton string `json:"extraButton"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "dialog", Operation: "password", Params: params,
	}); err != nil {
		return nil, err
	}
	var username, password string
	var err error
	c.OnMain(func() {
		opts := dialogOpts(dialogOptionParams{
			Title:       p.Title,
			OKLabel:     p.OKLabel,
			CancelLabel: p.CancelLabel,
			ExtraButton: p.ExtraButton,
		})
		if p.Username {
			opts = append(opts, zenity.Username())
		}
		username, password, err = zenity.Password(opts...)
	})
	if err == zenity.ErrCanceled {
		return map[string]any{"canceled": true}, nil
	}
	if err == zenity.ErrExtraButton {
		return map[string]any{"canceled": false, "button": p.ExtraButton, "username": username}, nil
	}
	if err != nil {
		return nil, core.Errorf("dialog", "%v", err)
	}
	return map[string]any{"canceled": false, "username": username, "password": password}, nil
}

func dialogColor(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title       string `json:"title"`
		ShowPalette bool   `json:"showPalette"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	var picked color.Color
	var err error
	c.OnMain(func() {
		opts := dialogOpts(dialogOptionParams{Title: p.Title})
		if p.ShowPalette {
			opts = append(opts, zenity.ShowPalette())
		}
		picked, err = zenity.SelectColor(opts...)
	})
	if err == zenity.ErrCanceled {
		return map[string]any{"canceled": true}, nil
	}
	if err != nil {
		return nil, core.Errorf("dialog", "%v", err)
	}
	r, g, b, a := picked.RGBA()
	rr, gg, bb, aa := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
	return map[string]any{
		"canceled": false,
		"r":        rr,
		"g":        gg,
		"b":        bb,
		"a":        aa,
		"hex":      fmt.Sprintf("#%02x%02x%02x", rr, gg, bb),
		"rgba":     fmt.Sprintf("rgba(%d,%d,%d,%.3f)", rr, gg, bb, float64(aa)/255),
	}, nil
}

func dialogIsSupported(_ *core.Context, _ json.RawMessage) (any, error) {
	available := true
	method := ""
	switch runtime.GOOS {
	case "darwin":
		method = "native"
	case "windows":
		method = "native"
	default:
		// zenity resolves: XDG portal > libzenity.so.0 > zenity binary.
		// XDG portal requires D-Bus and is not probed here; check binary/lib as heuristic.
		if _, err := exec.LookPath("zenity"); err == nil {
			method = "zenity"
		} else {
			libPaths := []string{
				"/usr/lib/libzenity.so.0",
				"/usr/lib/x86_64-linux-gnu/libzenity.so.0",
				"/usr/lib/aarch64-linux-gnu/libzenity.so.0",
			}
			for _, p := range libPaths {
				if _, statErr := os.Stat(p); statErr == nil {
					method = "libzenity"
					break
				}
			}
			if method == "" {
				available = false
				method = "none"
			}
		}
	}
	return map[string]any{
		"available": available,
		"method":    method,
		"platform":  runtime.GOOS,
	}, nil
}

// ---- Notifications ----------------------------------------------------------

func notifyToast(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title string `json:"title"`
		Text  string `json:"text"`
		Icon  string `json:"icon"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if p.Text == "" {
		return nil, core.Errorf("bad_request", "text is required")
	}
	if p.Title == "" {
		p.Title = "nex"
	}
	opts := []zenity.Option{zenity.Title(p.Title)}
	icon := p.Icon
	if icon == "" {
		icon = "info"
	}
	opts = append(opts, notifyIcon(icon))
	var err error
	c.OnMain(func() {
		err = zenity.Notify(p.Text, opts...)
	})
	if err != nil {
		return nil, core.Errorf("notify", "%v", err)
	}
	return map[string]any{
		"ok":       true,
		"title":    p.Title,
		"text":     p.Text,
		"icon":     icon,
		"platform": runtime.GOOS,
	}, nil
}

func notifyCapabilities(_ *core.Context, _ json.RawMessage) (any, error) {
	return map[string]any{
		"platform": runtime.GOOS,
		"toast":    true,
		"actions":  false,
		"click":    false,
		"close":    false,
		"sound":    false,
		"notes":    "zenity-backed notifications are fire-and-forget on the supported platforms",
	}, nil
}

func notifyIcon(icon string) zenity.Option {
	switch strings.ToLower(strings.TrimSpace(icon)) {
	case "", "info":
		return zenity.Icon(zenity.InfoIcon)
	case "error":
		return zenity.Icon(zenity.ErrorIcon)
	case "warning", "warn":
		return zenity.Icon(zenity.WarningIcon)
	case "question":
		return zenity.Icon(zenity.QuestionIcon)
	case "password":
		return zenity.Icon(zenity.PasswordIcon)
	case "none", "noicon", "no-icon":
		return zenity.Icon(zenity.NoIcon)
	default:
		return zenity.Icon(icon)
	}
}

// ---- Shell ------------------------------------------------------------------

func shellOpenURL(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		URL string `json:"url"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "shell", Operation: "openURL", URL: p.URL, Params: params,
	}); err != nil {
		return nil, err
	}
	if err := openWithOS(p.URL); err != nil {
		return nil, core.Errorf("shell", "%v", err)
	}
	return map[string]any{"ok": true}, nil
}

func shellOpenPath(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "shell", Operation: "openPath", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	if err := openWithOS(p.Path); err != nil {
		return nil, core.Errorf("shell", "%v", err)
	}
	return map[string]any{"ok": true}, nil
}

func shellShowInFolder(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "shell", Operation: "showInFolder", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	if err := showInFolderOS(p.Path); err != nil {
		return nil, core.Errorf("shell", "%v", err)
	}
	return map[string]any{"ok": true}, nil
}

func shellExec(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Command   string            `json:"command"`
		Cwd       string            `json:"cwd"`
		Env       map[string]string `json:"env"`
		TimeoutMS int               `json:"timeoutMs"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if p.Command == "" {
		return nil, core.Errorf("bad_request", "command is required")
	}
	operation := "exec"
	if c.Method == "sys.shell.run" {
		operation = "run"
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "shell", Operation: operation, Command: p.Command, Path: p.Cwd, Params: params,
	}); err != nil {
		return nil, err
	}
	timeout := time.Duration(p.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(c.Ctx, timeout)
	defer cancel()

	name, args, shellName := nativeShellCommand(p.Command)
	cmd := exec.CommandContext(ctx, name, args...)
	if p.Cwd != "" {
		cmd.Dir = p.Cwd
	}
	if len(p.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range p.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	timedOut := ctx.Err() == context.DeadlineExceeded
	exitCode := 0
	if err != nil {
		exitCode = -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return map[string]any{
		"ok":         err == nil,
		"command":    p.Command,
		"shell":      shellName,
		"exitCode":   exitCode,
		"stdout":     stdout.String(),
		"stderr":     stderr.String(),
		"timedOut":   timedOut,
		"durationMs": duration.Milliseconds(),
	}, nil
}

func shellStart(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Command string            `json:"command"`
		Cwd     string            `json:"cwd"`
		Env     map[string]string `json:"env"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if p.Command == "" {
		return nil, core.Errorf("bad_request", "command is required")
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "shell", Operation: "start", Command: p.Command, Path: p.Cwd, Params: params,
	}); err != nil {
		return nil, err
	}
	name, args, shellName := nativeShellCommand(p.Command)
	cmd := exec.Command(name, args...)
	if p.Cwd != "" {
		cmd.Dir = p.Cwd
	}
	if len(p.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range p.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	if err := cmd.Start(); err != nil {
		return nil, core.Errorf("shell", "%v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return map[string]any{
		"ok":      true,
		"command": p.Command,
		"shell":   shellName,
		"pid":     pid,
	}, nil
}

func nativeShellCommand(command string) (string, []string, string) {
	if runtime.GOOS == "windows" {
		shell := os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		return shell, []string{"/C", command}, "cmd"
	}
	return "/bin/sh", []string{"-lc", command}, "sh"
}

// openWithOS/showInFolderOS delegate to nativeOpenTarget/nativeRevealInFolder,
// implemented per platform (NSWorkspace cgo on darwin, ShellExecuteW +
// SHOpenFolderAndSelectItems via syscall on windows, GIO cgo on linux — see
// launcher_darwin.go, launcher_windows.go, launcher_linux.go). No subprocess
// (open/rundll32/explorer/xdg-open) is spawned by either.

func openWithOS(target string) error {
	return nativeOpenTarget(target)
}

func showInFolderOS(path string) error {
	return nativeRevealInFolder(path)
}

// ---- Window -----------------------------------------------------------------

func windowInfo(c *core.Context, _ json.RawMessage) (any, error) {
	return c.WindowInfo(), nil
}

func windowSetTitle(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Title string `json:"title"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	c.SetTitle(p.Title)
	return map[string]any{"ok": true}, nil
}

func windowSetSize(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Hint   string `json:"hint"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if p.Width <= 0 || p.Height <= 0 {
		return nil, core.Errorf("bad_request", "width/height must be positive")
	}
	c.SetSize(p.Width, p.Height, p.Hint)
	return map[string]any{"ok": true}, nil
}

func windowGetSize(c *core.Context, _ json.RawMessage) (any, error) {
	info := c.WindowInfo()
	return map[string]any{"width": info["width"], "height": info["height"]}, nil
}

func windowFullscreen(c *core.Context, _ json.RawMessage) (any, error) {
	c.Eval(`document.documentElement.requestFullscreen&&document.documentElement.requestFullscreen().catch(function(){});`)
	return map[string]any{"ok": true, "mode": "dom-fullscreen"}, nil
}

func windowUnfullscreen(c *core.Context, _ json.RawMessage) (any, error) {
	c.Eval(`document.exitFullscreen&&document.fullscreenElement&&document.exitFullscreen().catch(function(){});`)
	return map[string]any{"ok": true, "mode": "dom-fullscreen"}, nil
}

func windowReload(c *core.Context, _ json.RawMessage) (any, error) {
	c.Eval(`window.location.reload();`)
	return map[string]any{"ok": true}, nil
}

func windowPrint(c *core.Context, _ json.RawMessage) (any, error) {
	c.Eval(`window.print();`)
	return map[string]any{"ok": true}, nil
}

func windowEval(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		JS string `json:"js"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if p.JS == "" {
		return nil, core.Errorf("bad_request", "js is required")
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "window", Operation: "eval", JS: p.JS, Params: params,
	}); err != nil {
		return nil, err
	}
	c.Eval(p.JS)
	return map[string]any{"ok": true}, nil
}

// ---- Logging ----------------------------------------------------------------

func logPrint(_ *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Message string `json:"message"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	log.Print(p.Message)
	return map[string]any{"ok": true}, nil
}

func logLevel(level string) core.HandlerFunc {
	return func(_ *core.Context, params json.RawMessage) (any, error) {
		var p struct {
			Message string `json:"message"`
		}
		if err := pBind(params, &p); err != nil {
			return nil, core.Errorf("bad_request", "%v", err)
		}
		log.Printf("nex [%s] %s", level, p.Message)
		return map[string]any{"ok": true, "level": level}, nil
	}
}

func pBind(params json.RawMessage, v any) error {
	if len(params) == 0 {
		return nil
	}
	return json.Unmarshal(params, v)
}

// ---- Filesystem -------------------------------------------------------------

func fsRead(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path     string `json:"path"`
		Encoding string `json:"encoding"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "read", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	enc := p.Encoding
	if enc == "" {
		enc = "utf8"
	}
	out := string(data)
	if enc == "base64" {
		out = base64.StdEncoding.EncodeToString(data)
	}
	return map[string]any{"data": out, "encoding": enc}, nil
}

func fsWrite(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path     string `json:"path"`
		Data     string `json:"data"`
		Encoding string `json:"encoding"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "write", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	var raw []byte
	if p.Encoding == "base64" {
		b, err := base64.StdEncoding.DecodeString(p.Data)
		if err != nil {
			return nil, core.Errorf("bad_request", "invalid base64: %v", err)
		}
		raw = b
	} else {
		raw = []byte(p.Data)
	}
	if err := os.WriteFile(p.Path, raw, 0o644); err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"ok": true, "bytes": len(raw)}, nil
}

func fsList(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "list", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	type entry struct {
		Name  string `json:"name"`
		IsDir bool   `json:"isDir"`
		Size  int64  `json:"size"`
	}
	out := make([]entry, 0, len(entries))
	for _, e := range entries {
		var size int64
		if info, err := e.Info(); err == nil {
			size = info.Size()
		}
		out = append(out, entry{Name: e.Name(), IsDir: e.IsDir(), Size: size})
	}
	return map[string]any{"entries": out}, nil
}

func fsExists(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "exists", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	info, err := os.Stat(p.Path)
	if os.IsNotExist(err) {
		return map[string]any{"exists": false}, nil
	}
	if err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"exists": true, "isDir": info.IsDir()}, nil
}

func fsStat(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "stat", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	info, err := os.Stat(p.Path)
	if err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{
		"name":    info.Name(),
		"size":    info.Size(),
		"isDir":   info.IsDir(),
		"modTime": info.ModTime().Format(time.RFC3339),
	}, nil
}

func fsMkdir(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "mkdir", Path: p.Path, Params: params,
	}); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(p.Path, 0o755); err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"ok": true}, nil
}

func fsRemove(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "remove", Path: p.Path, Recursive: p.Recursive, Params: params,
	}); err != nil {
		return nil, err
	}
	var err error
	if p.Recursive {
		err = os.RemoveAll(p.Path)
	} else {
		err = os.Remove(p.Path)
	}
	if err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"ok": true}, nil
}

func fsRename(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := c.Bind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "rename", From: p.From, To: p.To, Params: params,
	}); err != nil {
		return nil, err
	}
	if err := os.Rename(p.From, p.To); err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"ok": true}, nil
}

// ---- Clipboard --------------------------------------------------------------
//
// Clipboard access is native per platform — no shell subprocess is spawned.
// nativeClipboardReadText / nativeClipboardWriteText / nativeClipboardAvailable
// are implemented in clipboard_darwin.go (cgo + NSPasteboard),
// clipboard_windows.go (syscall + user32, no cgo), and clipboard_linux.go
// (cgo + GTK, reusing the gtk+-3.0 linkage already required by webview_go's
// GTK/WebKitGTK backend on Linux). Reads/writes are dispatched via c.OnMain
// so they run on the same thread as the webview/GTK main loop.

func clipboardReadText(c *core.Context, params json.RawMessage) (any, error) {
	if err := c.Authorize(core.SecurityDecision{
		Category: "clipboard", Operation: "readText", Params: params,
	}); err != nil {
		return nil, err
	}
	var text, tool string
	var ok bool
	var err error
	c.OnMain(func() {
		text, ok, tool, err = nativeClipboardReadText()
	})
	if err != nil {
		return nil, core.Errorf("clipboard", "%v", err)
	}
	if !ok {
		return nil, core.Errorf("unavailable", "native clipboard text read is not available on this platform")
	}
	return map[string]any{"text": text, "tool": tool, "platform": runtime.GOOS}, nil
}

func clipboardWriteText(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Text string `json:"text"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "clipboard", Operation: "writeText", Params: params,
	}); err != nil {
		return nil, err
	}
	var ok bool
	var tool string
	var err error
	c.OnMain(func() {
		ok, tool, err = nativeClipboardWriteText(p.Text)
	})
	if err != nil {
		return nil, core.Errorf("clipboard", "%v", err)
	}
	if !ok {
		return nil, core.Errorf("unavailable", "native clipboard text write is not available on this platform")
	}
	return map[string]any{"ok": true, "tool": tool, "platform": runtime.GOOS}, nil
}

func clipboardClear(c *core.Context, params json.RawMessage) (any, error) {
	if err := c.Authorize(core.SecurityDecision{
		Category: "clipboard", Operation: "clear", Params: params,
	}); err != nil {
		return nil, err
	}
	var ok bool
	var tool string
	var err error
	c.OnMain(func() {
		ok, tool, err = nativeClipboardWriteText("")
	})
	if err != nil {
		return nil, core.Errorf("clipboard", "%v", err)
	}
	if !ok {
		return nil, core.Errorf("unavailable", "native clipboard clear is not available on this platform")
	}
	return map[string]any{"ok": true, "tool": tool, "platform": runtime.GOOS}, nil
}

func clipboardFormats(_ *core.Context, _ json.RawMessage) (any, error) {
	supported, tool := nativeClipboardAvailable()
	return map[string]any{
		"text":      supported,
		"image":     false,
		"files":     false,
		"html":      false,
		"tool":      tool,
		"platform":  runtime.GOOS,
		"clipboard": "text",
	}, nil
}

func clipboardIsAvailable(_ *core.Context, _ json.RawMessage) (any, error) {
	available, tool := nativeClipboardAvailable()
	return map[string]any{
		"available": available,
		"tool":      tool,
		"platform":  runtime.GOOS,
	}, nil
}

// ---- App --------------------------------------------------------------------

func appQuit(c *core.Context, params json.RawMessage) (any, error) {
	if err := c.Authorize(core.SecurityDecision{
		Category: "app", Operation: "quit", Params: params,
	}); err != nil {
		return nil, err
	}
	c.Quit()
	return map[string]any{"ok": true}, nil
}

func appExit(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Code int `json:"code"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := c.Authorize(core.SecurityDecision{
		Category: "app", Operation: "exit", Params: params,
	}); err != nil {
		return nil, err
	}
	go func(code int) {
		time.Sleep(120 * time.Millisecond)
		// os.Exit skips all deferred cleanup (including the deferred
		// stopSingleInstance in App.Run), so release the single-instance
		// lock explicitly here to avoid leaving a stale lock/secret file
		// behind a hard exit.
		c.ReleaseSingleInstance()
		os.Exit(code)
	}(p.Code)
	return map[string]any{"ok": true, "code": p.Code}, nil
}

func appRestart(c *core.Context, params json.RawMessage) (any, error) {
	if err := c.Authorize(core.SecurityDecision{
		Category: "app", Operation: "restart", Params: params,
	}); err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, core.Errorf("app", "%v", err)
	}
	// Release the single-instance lock before spawning the replacement
	// process. Without this, the new process can start (and probe the lock)
	// before this one actually exits, find the port still held, decide it's
	// a second instance, and quit immediately — the app appears to just
	// close instead of restarting.
	c.ReleaseSingleInstance()
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, core.Errorf("app", "%v", err)
	}
	// Quit through the same path as sys.app.quit (wv.Terminate -> Run()
	// returns -> OnShutdown -> main() returns) instead of a raw, timed
	// os.Exit. A fixed sleep-then-kill never gave OnShutdown a chance to
	// run at all, so any app-registered cleanup (e.g. explicitly tearing
	// down native WKWebViews on macOS) was skipped on restart — leaving
	// the WebKit-owned data-store process with no explicit signal to let
	// go of its files before a caller (e.g. an import/restore flow) writes
	// new ones to the same path out from under it.
	c.Quit()
	return map[string]any{"ok": true, "pid": cmd.Process.Pid, "executable": exe}, nil
}

func appPaths(_ *core.Context, _ json.RawMessage) (any, error) {
	return pathsInfo(), nil
}

func appRuntime(_ *core.Context, _ json.RawMessage) (any, error) {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	return map[string]any{
		"dev":        os.Getenv("APP_DEV") == "1",
		"packaged":   os.Getenv("APP_DEV") != "1",
		"pid":        os.Getpid(),
		"ppid":       os.Getppid(),
		"executable": exe,
		"cwd":        cwd,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}, nil
}

// ---- Menu -------------------------------------------------------------------

func menuSet(c *core.Context, params json.RawMessage) (any, error) {
	if len(params) == 0 || string(params) == "null" {
		return nil, core.Errorf("bad_request", "menu definition is required")
	}
	var definition any
	if err := json.Unmarshal(params, &definition); err != nil {
		return nil, core.Errorf("bad_request", "invalid menu definition: %v", err)
	}
	menuMu.Lock()
	applicationMenu = append(applicationMenu[:0], params...)
	menuMu.Unlock()
	c.Emit("menu.changed", definition)
	return map[string]any{
		"ok":       true,
		"renderer": "webview",
		"menu":     definition,
	}, nil
}

func menuUpdate(c *core.Context, _ json.RawMessage) (any, error) {
	menuMu.RLock()
	raw := append([]byte(nil), applicationMenu...)
	menuMu.RUnlock()
	if len(raw) == 0 {
		return map[string]any{"ok": true, "renderer": "webview", "menu": nil}, nil
	}
	var definition any
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, core.Errorf("menu", "stored menu definition is invalid: %v", err)
	}
	c.Emit("menu.changed", definition)
	return map[string]any{
		"ok":       true,
		"renderer": "webview",
		"menu":     definition,
	}, nil
}

// ---- Desktop integration ----------------------------------------------------

// appearanceInfo/powerInfo delegate to nativeAppearanceInfo/nativePowerInfo,
// implemented per platform (cgo NSUserDefaults/IOKit on darwin, registry/
// GetSystemPowerStatus via syscall on windows, /proc+env on linux — see
// appearance_darwin.go, power_darwin.go, sysinfo_windows.go,
// sysinfo_linux.go). No subprocess (defaults/reg query/pmset/wmic) is
// spawned by any of them.

func appearanceInfo(_ *core.Context, _ json.RawMessage) (any, error) {
	out := map[string]any{"platform": runtime.GOOS}
	for k, v := range nativeAppearanceInfo() {
		out[k] = v
	}
	return out, nil
}

func powerInfo(_ *core.Context, _ json.RawMessage) (any, error) {
	out := map[string]any{"platform": runtime.GOOS, "preventSleep": false}
	for k, v := range nativePowerInfo() {
		out[k] = v
	}
	return out, nil
}

func shortcutsRegister(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		ID          string `json:"id"`
		Accelerator string `json:"accelerator"`
		Label       string `json:"label"`
		Scope       string `json:"scope"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	p.Accelerator = strings.TrimSpace(p.Accelerator)
	if p.Accelerator == "" {
		return nil, core.Errorf("bad_request", "accelerator is required")
	}
	if p.ID == "" {
		p.ID = shortcutID(p.Accelerator)
	}
	if p.Scope == "" {
		p.Scope = "window"
	}
	enabled := true
	if p.Enabled != nil {
		enabled = *p.Enabled
	}
	entry := shortcutEntry{
		ID:          p.ID,
		Accelerator: p.Accelerator,
		Label:       p.Label,
		Scope:       p.Scope,
		Enabled:     enabled,
	}
	shortcutsMu.Lock()
	shortcuts[p.ID] = entry
	list := shortcutListLocked()
	shortcutsMu.Unlock()
	c.Emit("shortcuts.changed", map[string]any{"shortcuts": list})
	return map[string]any{"ok": true, "shortcut": entry, "shortcuts": list}, nil
}

func shortcutsUnregister(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		ID          string `json:"id"`
		Accelerator string `json:"accelerator"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	id := p.ID
	if id == "" && p.Accelerator != "" {
		id = shortcutID(p.Accelerator)
	}
	if id == "" {
		return nil, core.Errorf("bad_request", "id or accelerator is required")
	}
	shortcutsMu.Lock()
	delete(shortcuts, id)
	list := shortcutListLocked()
	shortcutsMu.Unlock()
	c.Emit("shortcuts.changed", map[string]any{"shortcuts": list})
	return map[string]any{"ok": true, "id": id, "shortcuts": list}, nil
}

func shortcutsClear(c *core.Context, _ json.RawMessage) (any, error) {
	shortcutsMu.Lock()
	shortcuts = map[string]shortcutEntry{}
	shortcutsMu.Unlock()
	c.Emit("shortcuts.changed", map[string]any{"shortcuts": []shortcutEntry{}})
	return map[string]any{"ok": true, "shortcuts": []shortcutEntry{}}, nil
}

func shortcutsList(_ *core.Context, _ json.RawMessage) (any, error) {
	shortcutsMu.RLock()
	defer shortcutsMu.RUnlock()
	return map[string]any{
		"scope":     "window",
		"shortcuts": shortcutListLocked(),
	}, nil
}

func shortcutListLocked() []shortcutEntry {
	out := make([]shortcutEntry, 0, len(shortcuts))
	for _, entry := range shortcuts {
		out = append(out, entry)
	}
	return out
}

func shortcutID(accelerator string) string {
	id := strings.ToLower(strings.TrimSpace(accelerator))
	replacer := strings.NewReplacer(" ", "", "+", "-", "_", "-", ".", "-", "/", "-")
	return replacer.Replace(id)
}

func protocolStatus(_ *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Scheme string `json:"scheme"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	scheme := strings.Trim(strings.ToLower(strings.TrimSpace(p.Scheme)), ":/")
	if scheme == "" {
		return nil, core.Errorf("bad_request", "scheme is required")
	}
	registered, handler, source := lookupProtocolHandler(scheme)
	return map[string]any{
		"scheme":     scheme,
		"registered": registered,
		"handler":    handler,
		"platform":   runtime.GOOS,
		"source":     source,
	}, nil
}

func updaterCheck(_ *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		URL     string `json:"url"`
		Current string `json:"current"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	// Backend env var takes precedence; frontend param accepted only as fallback.
	if envURL := os.Getenv("NEX_UPDATE_URL"); envURL != "" {
		p.URL = envURL
	}
	if p.Current == "" {
		p.Current = os.Getenv("NEX_APP_VERSION")
	}
	if p.Current == "" {
		p.Current = os.Getenv("APP_VERSION")
	}
	out := map[string]any{
		"configured":       p.URL != "",
		"current":          p.Current,
		"updateAvailable":  false,
		"provider":         p.URL,
		"checkedAt":        time.Now().Format(time.RFC3339),
		"providerProtocol": "",
	}
	if p.URL == "" {
		return out, nil
	}
	// Validate URL to prevent SSRF: require https and reject loopback/private hosts.
	parsed, err := url.Parse(p.URL)
	if err != nil {
		return nil, core.Errorf("bad_request", "invalid update url: %v", err)
	}
	if parsed.Scheme != "https" {
		return nil, core.Errorf("bad_request", "update url must use https")
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return nil, core.Errorf("bad_request", "update url host is not allowed")
		}
	} else {
		lc := strings.ToLower(host)
		if lc == "localhost" || strings.HasSuffix(lc, ".local") || strings.HasSuffix(lc, ".internal") {
			return nil, core.Errorf("bad_request", "update url host is not allowed")
		}
	}
	req, err := http.NewRequest(http.MethodGet, p.URL, nil)
	if err != nil {
		return nil, core.Errorf("bad_request", "invalid update url: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	client := http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, core.Errorf("network", "%v", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, core.Errorf("http", "update provider returned HTTP %d", res.StatusCode)
	}
	var payload struct {
		Version   string `json:"version"`
		URL       string `json:"url"`
		Notes     string `json:"notes"`
		Mandatory bool   `json:"mandatory"`
	}
	// Limit response body to 512 KiB to prevent memory exhaustion.
	if err := json.NewDecoder(io.LimitReader(res.Body, 512*1024)).Decode(&payload); err != nil {
		return nil, core.Errorf("bad_response", "invalid update JSON: %v", err)
	}
	out["latest"] = payload.Version
	out["url"] = payload.URL
	out["notes"] = payload.Notes
	out["mandatory"] = payload.Mandatory
	out["providerProtocol"] = "json"
	out["updateAvailable"] = compareVersion(payload.Version, p.Current) > 0
	return out, nil
}

func secureStorageAvailable(_ *core.Context, _ json.RawMessage) (any, error) {
	backend := ""
	available := false
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("security"); err == nil {
			backend = "macos-keychain"
			available = true
		}
	case "windows":
		if _, err := exec.LookPath("powershell"); err == nil {
			backend = "windows-dpapi"
			available = true
		} else if _, err := exec.LookPath("pwsh"); err == nil {
			backend = "windows-dpapi"
			available = true
		}
	default:
		if _, err := exec.LookPath("secret-tool"); err == nil {
			backend = "libsecret"
			available = true
		}
	}
	return map[string]any{
		"available": available,
		"platform":  runtime.GOOS,
		"backend":   backend,
	}, nil
}

func fileAssociationsStatus(_ *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Extension string `json:"extension"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	ext := strings.ToLower(strings.TrimSpace(p.Extension))
	if ext == "" {
		return nil, core.Errorf("bad_request", "extension is required")
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	registered, handler, mime, source := lookupFileAssociation(ext)
	return map[string]any{
		"extension":  ext,
		"registered": registered,
		"handler":    handler,
		"mime":       mime,
		"platform":   runtime.GOOS,
		"source":     source,
	}, nil
}

// ---- Trash ------------------------------------------------------------------
//
// trashEmpty delegates to nativeEmptyTrash, implemented per platform (pure Go
// on macOS/Linux — direct filesystem manipulation of the well-known trash
// directories; syscall SHEmptyRecycleBinW on Windows, no cgo). No subprocess
// (osascript/PowerShell Clear-RecycleBin/gio trash) is spawned by any of
// them.

func trashEmpty(c *core.Context, params json.RawMessage) (any, error) {
	if err := c.Authorize(core.SecurityDecision{
		Category: "filesystem", Operation: "trashEmpty", Params: params,
	}); err != nil {
		return nil, err
	}
	removed, err := nativeEmptyTrash()
	if err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	out := map[string]any{"ok": true, "platform": runtime.GOOS}
	if removed >= 0 {
		out["removed"] = removed
	}
	return out, nil
}

// removeDirContents deletes every entry inside dir (not dir itself), best
// effort: it keeps going on a per-entry error and returns the count actually
// removed plus the first error encountered, if any. Shared by the macOS and
// Linux native trash implementations, both of which just need "empty this
// well-known folder".
func removeDirContents(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	removed := 0
	var firstErr error
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		removed++
	}
	return removed, firstErr
}

func recentDocsAdd(_ *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := pBind(params, &p); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	p.Path = strings.TrimSpace(p.Path)
	if p.Path == "" {
		return nil, core.Errorf("bad_request", "path is required")
	}
	if abs, err := filepath.Abs(p.Path); err == nil {
		p.Path = abs
	}
	recentDocsMu.Lock()
	defer recentDocsMu.Unlock()
	docs := loadRecentDocs()
	docs = append([]string{p.Path}, removeString(docs, p.Path)...)
	if len(docs) > 100 {
		docs = docs[:100]
	}
	if err := saveRecentDocs(docs); err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"ok": true, "path": p.Path, "documents": docs}, nil
}

func recentDocsClear(_ *core.Context, _ json.RawMessage) (any, error) {
	recentDocsMu.Lock()
	defer recentDocsMu.Unlock()
	if err := saveRecentDocs([]string{}); err != nil {
		return nil, core.Errorf("io", "%v", err)
	}
	return map[string]any{"ok": true, "documents": []string{}}, nil
}

func recentDocsList(_ *core.Context, _ json.RawMessage) (any, error) {
	recentDocsMu.Lock()
	defer recentDocsMu.Unlock()
	return map[string]any{"documents": loadRecentDocs()}, nil
}

// lookupProtocolHandler/lookupFileAssociation delegate to nativeProtocolStatus/
// nativeFileAssociation, implemented per platform (own Info.plist read on
// darwin, registry via syscall on windows, XDG mimeapps.list/globs parsing on
// linux — see associations_darwin.go, sysinfo_windows.go, associations_linux.go).
// No subprocess (reg query/xdg-mime) is spawned by any of them.

func lookupProtocolHandler(scheme string) (bool, string, string) {
	return nativeProtocolStatus(scheme)
}

func lookupFileAssociation(ext string) (bool, string, string, string) {
	return nativeFileAssociation(ext)
}

func currentBundleInfoPlist() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for {
		if strings.HasSuffix(dir, ".app/Contents/MacOS") {
			return filepath.Join(filepath.Dir(dir), "Info.plist")
		}
		if strings.HasSuffix(dir, ".app/Contents") {
			return filepath.Join(dir, "Info.plist")
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
		dir = filepath.Dir(dir)
	}
}

func recentDocsPath() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "nex", "recent-docs.json")
}

func loadRecentDocs() []string {
	b, err := os.ReadFile(recentDocsPath())
	if err != nil {
		return []string{}
	}
	var docs []string
	if err := json.Unmarshal(b, &docs); err != nil {
		return []string{}
	}
	return docs
}

func saveRecentDocs(docs []string) error {
	path := recentDocsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func removeString(items []string, value string) []string {
	out := items[:0]
	for _, item := range items {
		if item != value {
			out = append(out, item)
		}
	}
	return out
}

func compareVersion(a, b string) int {
	as := versionParts(a)
	bs := versionParts(b)
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimLeft(strings.TrimSpace(v), "vV")
	fields := strings.FieldsFunc(v, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == '+'
	})
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		n, _ := strconv.Atoi(strings.TrimFunc(field, func(r rune) bool { return r < '0' || r > '9' }))
		out = append(out, n)
	}
	return out
}
