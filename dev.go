//go:build ignore

// Development runner for nex.
// Usage: go run dev.go
//
// Prepares Go modules, restarts Vite HMR, then runs the Go app with APP_DEV=1.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"nex/config"
)

const (
	frontendDir = "frontend"
	mainPkg     = "."
	viteHost    = "127.0.0.1"
	vitePort    = "5179"
	viteAddr    = viteHost + ":" + vitePort
	viteURL     = "http://" + viteAddr
	apiPort     = "34115"
	apiAddr     = viteHost + ":" + apiPort
)

var scriptEnvValues = map[string]string{}

func main() {
	if err := doDev(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func doDev() error {
	envFile := scriptEnvDefault("NEX_ENV_FILE", ".env")
	scriptEnvValues = config.Read(envFile)
	config.Load(envFile)

	appTitle, err := appTitleFromMain()
	if err != nil {
		return err
	}
	appIcon, err := appIconFromMain()
	if err != nil {
		return err
	}
	appID, err := appIDFromMain(appTitle)
	if err != nil {
		return err
	}

	if err := ensureGoModules(); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		if err := runCmd(frontendDir, nil, "npm", "install"); err != nil {
			return err
		}
	}
	if err := stopPreviousOnPort(apiPort, apiAddr, appTitle+" backend"); err != nil {
		return err
	}

	stopVite, err := ensureVite()
	if err != nil {
		return err
	}
	var once sync.Once
	cleanup := func() { once.Do(stopVite) }
	defer cleanup()

	// Kill Vite on SIGINT/SIGTERM even if the Go runtime skips defers on signal exit.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		cleanup()
		os.Exit(0)
	}()

	return runPlatformDevApp(appTitle, appID, appIcon)
}

func runPlatformDevApp(appTitle, appID, appIcon string) error {
	if runtime.GOOS == "darwin" {
		return runDarwinDevApp(appTitle, appID, appIcon)
	}
	appCmd := exec.Command("go", "run", mainPkg)
	appCmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"APP_DEV=1",
		"NEX_DEV_SERVER_URL="+viteURL,
	)
	appCmd.Stdout = os.Stdout
	appCmd.Stderr = os.Stderr
	return appCmd.Run()
}

func runDarwinDevApp(appTitle, appID, appIcon string) error {
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	appPath := filepath.Join(os.TempDir(), appTitle+"-dev.app")
	if err := os.RemoveAll(appPath); err != nil {
		return err
	}
	macosDir := filepath.Join(appPath, "Contents", "MacOS")
	resDir := filepath.Join(appPath, "Contents", "Resources")
	if err := os.MkdirAll(macosDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		return err
	}
	if err := writeDarwinInfoPlist(appPath, appTitle, appID, map[string]string{
		"APP_DEV":            "1",
		"NEX_DEV_SERVER_URL": viteURL,
		"NEX_ENV_FILE":       filepath.Join(projectDir, ".env"),
	}); err != nil {
		return err
	}
	if err := writeDarwinIcon(resDir, appIcon); err != nil {
		fmt.Printf("-> app icon skipped: %v\n", err)
	}
	out := filepath.Join(macosDir, appID)
	if err := runCmd(".", append([]string{"CGO_ENABLED=1"}, os.Environ()...), "go", "build", "-o", out, mainPkg); err != nil {
		return err
	}
	cmd := exec.Command("open", "-W", appPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func writeDarwinInfoPlist(appPath, appTitle, executable string, env map[string]string) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>`)
	b.WriteString(plistEscape(appTitle))
	b.WriteString(`</string>
  <key>CFBundleDisplayName</key><string>`)
	b.WriteString(plistEscape(appTitle))
	b.WriteString(`</string>
  <key>CFBundleIdentifier</key><string>`)
	b.WriteString(plistEscape(executable))
	b.WriteString(`</string>
  <key>CFBundleVersion</key><string>dev</string>
  <key>CFBundleShortVersionString</key><string>dev</string>
  <key>CFBundleExecutable</key><string>`)
	b.WriteString(plistEscape(executable))
	b.WriteString(`</string>
  <key>CFBundleIconFile</key><string>nex</string>
  <key>LSMinimumSystemVersion</key><string>10.13</string>
  <key>LSEnvironment</key>
  <dict>
`)
	for k, v := range env {
		b.WriteString("    <key>")
		b.WriteString(plistEscape(k))
		b.WriteString("</key><string>")
		b.WriteString(plistEscape(v))
		b.WriteString("</string>\n")
	}
	b.WriteString(`  </dict>
</dict>
</plist>
`)
	return os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), []byte(b.String()), 0o644)
}

func writeDarwinIcon(resourcesDir, source string) error {
	if strings.EqualFold(filepath.Ext(source), ".icns") {
		return copyFile(source, filepath.Join(resourcesDir, "nex.icns"))
	}
	iconset := filepath.Join(os.TempDir(), "nex-dev.iconset")
	if err := os.RemoveAll(iconset); err != nil {
		return err
	}
	if err := os.MkdirAll(iconset, 0o755); err != nil {
		return err
	}
	icons := []struct {
		name string
		size string
	}{
		{"icon_16x16.png", "16"}, {"icon_16x16@2x.png", "32"},
		{"icon_32x32.png", "32"}, {"icon_32x32@2x.png", "64"},
		{"icon_128x128.png", "128"}, {"icon_128x128@2x.png", "256"},
		{"icon_256x256.png", "256"}, {"icon_256x256@2x.png", "512"},
		{"icon_512x512.png", "512"}, {"icon_512x512@2x.png", "1024"},
	}
	for _, icon := range icons {
		out := filepath.Join(iconset, icon.name)
		if err := exec.Command("sips", "-s", "format", "png", "-z", icon.size, icon.size, source, "--out", out).Run(); err != nil {
			return err
		}
	}
	return exec.Command("iconutil", "-c", "icns", iconset, "-o", filepath.Join(resourcesDir, "nex.icns")).Run()
}

func plistEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return strings.ReplaceAll(s, "'", "&apos;")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func appTitleFromMain() (string, error) {
	title, err := configStringFromMain("Title")
	if err != nil {
		return "", err
	}
	if title == "" {
		return "", fmt.Errorf("main.go nexpkg.Config Title is missing or not resolvable")
	}
	return title, nil
}

func appIconFromMain() (string, error) {
	icon, err := configStringFromMain("Icon")
	if err != nil {
		return "", err
	}
	if icon == "" {
		icon = filepath.Join("res", "nexicon.svg")
	}
	if _, err := os.Stat(icon); err != nil {
		return "", fmt.Errorf("main.go nexpkg.Config Icon %q is not readable: %w", icon, err)
	}
	return icon, nil
}

func appIDFromMain(appTitle string) (string, error) {
	id, err := configStringFromMain("SingleInstanceID")
	if err != nil {
		return "", err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return appIDFromTitle(appTitle), nil
	}
	return id, nil
}

func configStringFromMain(field string) (string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		return "", fmt.Errorf("reading %s from main.go: %w", field, err)
	}
	locals := localStringValues(f)
	value := ""
	ast.Inspect(f, func(n ast.Node) bool {
		if value != "" {
			return false
		}
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != field {
			return true
		}
		if resolved, ok := stringExprValue(kv.Value, locals); ok {
			value = strings.TrimSpace(resolved)
		}
		return false
	})
	return value, nil
}

func localStringValues(f *ast.File) map[string]string {
	locals := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			if i >= len(assign.Rhs) {
				continue
			}
			name, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			if value, ok := stringExprValue(assign.Rhs[i], locals); ok {
				locals[name.Name] = strings.TrimSpace(value)
			}
		}
		return true
	})
	return locals
}

func stringExprValue(expr ast.Expr, locals map[string]string) (string, bool) {
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false
		}
		return unquoted, true
	case *ast.Ident:
		value, ok := locals[v.Name]
		return value, ok
	case *ast.CallExpr:
		fun, ok := v.Fun.(*ast.Ident)
		if !ok {
			return "", false
		}
		switch fun.Name {
		case "envDefault":
			if len(v.Args) != 2 {
				return "", false
			}
			key, ok := stringExprValue(v.Args[0], locals)
			if !ok {
				return "", false
			}
			fallback, ok := stringExprValue(v.Args[1], locals)
			if !ok {
				return "", false
			}
			return scriptEnvDefault(key, fallback), true
		case "envValue":
			if len(v.Args) != 3 {
				return "", false
			}
			key, ok := stringExprValue(v.Args[1], locals)
			if !ok {
				return "", false
			}
			fallback, ok := stringExprValue(v.Args[2], locals)
			if !ok {
				return "", false
			}
			return scriptEnvDefault(key, fallback), true
		default:
			return "", false
		}
	default:
		return "", false
	}
}

func scriptEnvDefault(key, fallback string) string {
	if v := strings.TrimSpace(scriptEnvValues[key]); v != "" {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func appIDFromTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	dash := false
	for _, r := range title {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "nex"
	}
	return out
}

func ensureGoModules() error {
	return runCmd(".", nil, "go", "mod", "tidy")
}

func ensureVite() (func(), error) {
	if err := stopPreviousOnPort(vitePort, viteAddr, "vite server"); err != nil {
		return nil, err
	}

	vite := exec.Command("npm", "run", "dev")
	vite.Dir = frontendDir
	vite.Env = append(os.Environ(),
		"NEX_VITE_HOST="+viteHost,
		"NEX_VITE_PORT="+vitePort,
	)
	vite.Stdout = os.Stdout
	vite.Stderr = os.Stderr
	if err := vite.Start(); err != nil {
		return nil, fmt.Errorf("starting vite: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- vite.Wait() }()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			if err != nil {
				return nil, fmt.Errorf("vite exited early: %w", err)
			}
			return nil, fmt.Errorf("vite exited early")
		default:
			if nexViteReady(viteURL) {
				return func() {
					_ = vite.Process.Kill()
					<-done
				}, nil
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	_ = vite.Process.Kill()
	<-done
	return nil, fmt.Errorf("vite did not become ready at %s", viteURL)
}

func stopPreviousOnPort(port, addr, label string) error {
	pids, err := pidsOnPort(port)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	fmt.Printf("-> stopping previous %s on %s (%s)\n", label, addr, strings.Join(pids, ", "))
	for _, pid := range pids {
		desc, ok := stoppableDevProcess(pid, label)
		if !ok {
			return fmt.Errorf("refusing to stop process %s on %s: %s", pid, addr, desc)
		}
		if err := killPID(pid); err != nil {
			return err
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !portOpen(addr) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("port %s is still in use after stopping previous %s", addr, label)
}

func pidsOnPort(port string) ([]string, error) {
	if runtime.GOOS == "windows" {
		return windowsPIDsOnPort(port)
	}
	out, err := exec.Command("lsof", "-ti", "tcp:"+port).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) == 0 && len(out) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("finding process on port %s: %w", port, err)
	}
	return splitLines(string(out)), nil
}

func windowsPIDsOnPort(port string) ([]string, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return nil, fmt.Errorf("finding process on port %s: %w", port, err)
	}
	seen := map[string]bool{}
	var pids []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		local := fields[1]
		state := fields[3]
		pid := fields[4]
		if strings.HasSuffix(local, ":"+port) && state == "LISTENING" && !seen[pid] {
			seen[pid] = true
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func stoppableDevProcess(pid, _ string) (string, bool) {
	desc := processDescription(pid)
	lower := strings.ToLower(desc)
	allowed := []string{
		"vite",
		"npm",
		"node",
		"go run",
		"nex",
		"-dev.app",
		"app_dev=1",
		"nex_dev_server_url",
	}
	for _, marker := range allowed {
		if strings.Contains(lower, marker) {
			return desc, true
		}
	}
	if desc == "" {
		desc = "process details unavailable"
	}
	return desc, false
}

func processDescription(pid string) string {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("wmic", "process", "where", "ProcessId="+pid, "get", "Name,CommandLine", "/format:list").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	out, err := exec.Command("ps", "-p", pid, "-o", "comm=", "-o", "args=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func killPID(pid string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("taskkill", "/PID", pid, "/T", "/F")
	} else {
		cmd = exec.Command("kill", pid)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping process %s: %w", pid, err)
	}
	return nil
}

func nexViteReady(rawURL string) bool {
	client := http.Client{Timeout: 300 * time.Millisecond}
	res, err := client.Get(rawURL + "/src/lib/nex.js")
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil {
		return false
	}
	return strings.Contains(string(body), "nex client bridge")
}

func portOpen(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func runCmd(dir string, env []string, name string, args ...string) error {
	fmt.Printf("-> (%s) %s %s\n", dir, name, strings.Join(args, " "))
	c := exec.Command(name, args...)
	c.Dir = dir
	if env != nil {
		c.Env = env
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
