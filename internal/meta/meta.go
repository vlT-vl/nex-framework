package meta

import (
	"os"
	"runtime/debug"
	"strings"
)

const Name = "nex"

var (
	Version = "0.1.2"
	Build   = "R281026"
	Updated = "28 Ottobre 2026"
	Author  = "© 2026 vlT di Veronesi Lorenzo"

	GoVersion      string
	ZenityVersion  string
	WebviewVersion string
)

func Info() map[string]any {
	return map[string]any{
		"name":    Name,
		"version": Version,
		"build":   Build,
		"updated": Updated,
		"author":  Author,
		"id":      Name + "@" + Version,
		"stack":   Stack(),
	}
}

func Stack() map[string]string {
	out := map[string]string{
		"nex": Name + "@" + Version + "-" + Build,
	}
	if GoVersion != "" {
		out["go"] = GoVersion
	}
	if ZenityVersion != "" {
		out["zenity"] = ZenityVersion
	}
	if WebviewVersion != "" {
		out["webview_go"] = WebviewVersion
	}
	if GoVersion != "" && ZenityVersion != "" && WebviewVersion != "" {
		return out
	}
	bi, ok := debug.ReadBuildInfo()
	if ok && GoVersion == "" && bi.GoVersion != "" {
		out["go"] = strings.TrimPrefix(bi.GoVersion, "go")
	}
	if ok {
		for _, dep := range bi.Deps {
			addDependency(out, dep.Path, dep.Version)
			if dep.Replace != nil {
				addDependency(out, dep.Path, dep.Replace.Version)
			}
		}
	}
	for path, version := range readGoModDependencies("go.mod") {
		addDependency(out, path, version)
	}
	return out
}

func addDependency(out map[string]string, path, version string) {
	if version == "" || version == "(devel)" {
		return
	}
	switch path {
	case "github.com/ncruces/zenity":
		out["zenity"] = version
	case "github.com/webview/webview_go":
		out["webview_go"] = version
	}
}

func readGoModDependencies(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "github.com/") {
			out[fields[0]] = fields[1]
		}
	}
	return out
}
