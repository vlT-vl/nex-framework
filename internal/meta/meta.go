// Package meta contains nex framework-level identity.
// Set at build time with -ldflags "-X nex/internal/meta.Build=R... -X ...".
package meta

import (
	"os"
	"runtime/debug"
	"strings"
)

const Name = "nex"

var (
	Version = "0.1.0"
	Build   = "R070726"
	Updated = "7 Luglio 2026"
	Author  = "© 2026 vlT di Veronesi Lorenzo"

	// GoVersion, ZenityVersion and WebviewVersion are set at build time via
	// -ldflags "-X nex/internal/meta.GoVersion=... -X nex/internal/meta.ZenityVersion=... -X nex/internal/meta.WebviewVersion=..."
	// (see build.go). Runtime introspection (debug.ReadBuildInfo / go.mod
	// parsing, used as a fallback in Stack below) is unreliable in a
	// packaged app: garble deliberately blanks/strips build info (so it
	// doesn't leak pre-obfuscation module paths) — bi.GoVersion and
	// bi.Deps can both come back empty, not just Deps — and go.mod is read
	// via a path relative to the working directory, which only resolves
	// when it happens to be the project root — true for `go run` but not
	// for a double-clicked .app. Left empty here so `go run`/`make dev`
	// (which don't pass these ldflags) keep working via the fallback.
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
		return out // set at build time — skip the fragile runtime introspection below
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
