//go:build ignore

// Build orchestrator for nex.
// Usage: go run build.go [command] [flags]
//
//	build         (default) build frontend then all target binaries
//	release       optimized build (-s -w, windowsgui on Windows targets)
//	obfuscate     release + garble code obfuscation
//	requirements  show and validate host requirements for selected targets
//	doctor        alias for requirements
//	clean         remove release/ and frontend/dist contents
//
// Flags:
//
//	--plain  skip garble and use go build directly
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"nex/config"
)

const (
	frontendDir = "frontend"
	mainPkg     = "."
	releaseRoot = "release"
)

var outDir = releaseRoot
var scriptEnvValues = map[string]string{}

type buildTarget struct {
	GOOS   string
	GOARCH string
}

func main() {
	args := os.Args[1:]
	cmd := "build"
	obfuscate := true
	filtered := args[:0]
	for _, a := range args {
		if a == "--plain" || a == "-plain" {
			obfuscate = false
		} else {
			filtered = append(filtered, a)
		}
	}
	args = filtered
	if len(args) > 0 {
		cmd = args[0]
	}
	if cmd == "obfuscate" {
		cmd = "release"
		obfuscate = true
	}

	var err error
	switch cmd {
	case "build":
		err = doBuild(false, obfuscate)
	case "release":
		err = doBuild(true, obfuscate)
	case "requirements", "doctor":
		err = doRequirements()
	case "clean":
		err = doClean()
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		fmt.Fprintln(os.Stderr, "commands: build | release | obfuscate | requirements | doctor | clean")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func doBuild(release, obfuscate bool) error {
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
	appVersion, appBuild, err := appVersionBuildFromVersion()
	if err != nil {
		return err
	}
	releaseName := appVersion + "-" + appBuild
	outDir = filepath.Join(releaseRoot, releaseName)
	targets := buildTargets()
	if err := validateBuildTargets(targets); err != nil {
		return err
	}

	if err := ensureGoModules(); err != nil {
		return err
	}
	if err := buildFrontend(); err != nil {
		return err
	}
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	tool := "go"
	buildPrefix := []string{"build"}
	garbleScope := ""
	if obfuscate {
		garblePath, err := ensureGarble()
		if err != nil {
			return err
		}
		modulePath, err := modulePathFromGoMod()
		if err != nil {
			return err
		}
		garbleScope = modulePath + "," + modulePath + "/..."
		tool = garblePath
		buildPrefix = []string{"-literals", "-seed=random", "build"}
		fmt.Printf("→ using garble for obfuscation (GOGARBLE=%s)\n", garbleScope)
	}

	tmpDir, err := os.MkdirTemp("", "nex-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	for _, target := range targets {
		fmt.Printf("→ target %s/%s\n", target.GOOS, target.GOARCH)
		binaryPath, err := buildTargetBinary(tmpDir, tool, buildPrefix, target, release, appID, appTitle, appIcon, appVersion, garbleScope)
		if err != nil {
			return err
		}
		if err := packagePlatformApp(binaryPath, appTitle, appID, appIcon, appVersion, appBuild, target); err != nil {
			return err
		}
	}
	return nil
}

func ensureGoModules() error {
	return runCmd(".", nil, "go", "mod", "tidy")
}

func ensureGarble() (string, error) {
	if garblePath, err := exec.LookPath("garble"); err == nil {
		return garblePath, nil
	}
	fmt.Println("→ garble not found; installing mvdan.cc/garble@latest")
	if err := runCmd(".", nil, "go", "install", "mvdan.cc/garble@latest"); err != nil {
		return "", err
	}
	if garblePath, err := exec.LookPath("garble"); err == nil {
		return garblePath, nil
	}
	garblePath, err := goBinPath("garble")
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(garblePath); err != nil {
		return "", fmt.Errorf("garble installed but not found at %s", garblePath)
	}
	return garblePath, nil
}

func goBinPath(name string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	gobin, err := goEnv("GOBIN")
	if err != nil {
		return "", err
	}
	if gobin == "" {
		gopath, err := goEnv("GOPATH")
		if err != nil {
			return "", err
		}
		if gopath == "" {
			return "", fmt.Errorf("go env GOPATH is empty")
		}
		gobin = filepath.Join(gopath, "bin")
	}
	return filepath.Join(gobin, name), nil
}

func goEnv(key string) (string, error) {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func modulePathFromGoMod() (string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if modulePath != "" {
				return modulePath, nil
			}
		}
	}
	return "", fmt.Errorf("go.mod module path not found")
}

// goModDependencyVersions parses go.mod's require block for dependency
// versions, keyed by import path (e.g. "github.com/ncruces/zenity").
// Read once at build time, when the working directory is reliably the
// project root — unlike internal/meta.Stack()'s own runtime fallback, which
// reads the same file relative to whatever the *packaged app's* working
// directory happens to be (wrong for a double-clicked .app) and can also be
// undermined by garble rewriting debug.ReadBuildInfo()'s dependency paths.
func goModDependencyVersions() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile("go.mod")
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

func buildFrontend() error {
	// npm install only if node_modules is missing
	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		if err := runCmd(frontendDir, nil, "npm", "install"); err != nil {
			return err
		}
	}
	return runCmd(frontendDir, nil, "npm", "run", "build")
}

func buildTargets() []buildTarget {
	switch runtime.GOOS {
	case "darwin":
		return []buildTarget{
			{GOOS: "darwin", GOARCH: "amd64"},
			{GOOS: "darwin", GOARCH: "arm64"},
			{GOOS: "windows", GOARCH: "amd64"},
		}
	case "linux":
		// Linux targets the host's own architecture (amd64 or arm64) rather
		// than a hardcoded amd64: cross-arch cgo/GTK builds need a matching
		// sysroot this script doesn't set up, so a Linux amd64/arm64 release
		// is produced by running the build natively on that architecture.
		return []buildTarget{
			{GOOS: "linux", GOARCH: runtime.GOARCH},
			{GOOS: "windows", GOARCH: "amd64"},
		}
	case "windows":
		return []buildTarget{
			{GOOS: "windows", GOARCH: "amd64"},
		}
	default:
		return []buildTarget{
			{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		}
	}
}

func doRequirements() error {
	targets := buildTargets()
	fmt.Println("nex build requirements")
	fmt.Printf("host: %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("targets:")
	for _, target := range targets {
		fmt.Printf("  - %s/%s\n", target.GOOS, target.GOARCH)
	}
	fmt.Println()
	fmt.Println("expected host tools:")
	fmt.Println("  - Go 1.26.4+")
	fmt.Println("  - Node 20.19+ or 22.12+ with npm")
	fmt.Println("  - garble on PATH, or network access for: go install mvdan.cc/garble@latest")
	fmt.Println("  - cgo toolchains for every target selected for this host")
	fmt.Println()
	fmt.Println("target toolchains:")
	for _, target := range targets {
		fmt.Printf("  - %s/%s\n", target.GOOS, target.GOARCH)
		switch target.GOOS {
		case "darwin":
			fmt.Println("      macOS host with Xcode Command Line Tools")
			fmt.Println("      tools: xcrun, sips, iconutil")
		case "linux":
			cc, cxx := compilersForTarget(target)
			pkgConfig := pkgConfigForTarget(target)
			fmt.Printf("      CC=%s\n", emptyDefault(cc, "host default"))
			fmt.Printf("      CXX=%s\n", emptyDefault(cxx, "host default"))
			fmt.Printf("      PKG_CONFIG=%s\n", emptyDefault(pkgConfig, "pkg-config"))
			fmt.Println("      pkg-config packages: gtk+-3.0 webkit2gtk-4.0 gio-2.0")
		case "windows":
			cc, cxx := compilersForTarget(target)
			fmt.Printf("      CC=%s\n", emptyDefault(cc, "host default"))
			fmt.Printf("      CXX=%s\n", emptyDefault(cxx, "host default"))
			fmt.Println("      toolchain: mingw-w64 (gcc/g++/windres)")
			fmt.Println("      icon: sips (macOS) or ImageMagick magick/convert, unless the app Icon is already .ico")
		}
	}
	fmt.Println()

	missing := collectMissingBuildRequirements(targets)
	if len(missing) == 0 {
		fmt.Println("OK: build requirements found")
		return nil
	}
	fmt.Fprintln(os.Stderr, formatMissingRequirements(missing))
	return errors.New("requirements not satisfied")
}

func buildTargetBinary(tmpDir, tool string, buildPrefix []string, target buildTarget, release bool, appID, appTitle, appIcon, appVersion, garbleScope string) (string, error) {
	targetDir := filepath.Join(tmpDir, target.GOOS+"-"+target.GOARCH)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	outName := appID
	if target.GOOS == "windows" {
		outName += ".exe"
	}
	out := filepath.Join(targetDir, outName)

	ldParts := []string{}
	if release {
		ldParts = append(ldParts, "-s", "-w")
	}
	// Bake the actual toolchain/dependency versions in at build time — see
	// internal/meta.GoVersion/ZenityVersion/WebviewVersion for why runtime
	// introspection of these isn't reliable in a packaged app (garble
	// blanks debug.ReadBuildInfo() to avoid leaking pre-obfuscation module
	// paths, so the "Runtime" stack row in the About/Framework modal came
	// back completely empty, not even "go").
	if modulePath, err := modulePathFromGoMod(); err == nil {
		ldParts = append(ldParts, "-X", modulePath+"/internal/meta.GoVersion="+strings.TrimPrefix(runtime.Version(), "go"))
		deps := goModDependencyVersions()
		if v := deps["github.com/ncruces/zenity"]; v != "" {
			ldParts = append(ldParts, "-X", modulePath+"/internal/meta.ZenityVersion="+v)
		}
		if v := deps["github.com/webview/webview_go"]; v != "" {
			ldParts = append(ldParts, "-X", modulePath+"/internal/meta.WebviewVersion="+v)
		}
	}
	if target.GOOS == "windows" {
		// GUI subsystem: no console window ever, matching what Wails produces.
		ldParts = append(ldParts, "-H", "windowsgui")
		sysoName, err := writeWindowsResource(targetDir, target, appID, appIcon, appTitle, appVersion)
		if err != nil {
			return "", fmt.Errorf("windows resource (icon/manifest): %w", err)
		}
		defer os.Remove(sysoName)
	}
	ldflags := strings.Join(ldParts, " ")

	buildArgs := append([]string{}, buildPrefix...)
	buildArgs = append(buildArgs,
		"-trimpath",
		"-ldflags", ldflags,
		"-o", out,
		mainPkg,
	)

	fmt.Printf("→ %s/%s %s %s\n", target.GOOS, target.GOARCH, filepath.Base(tool), strings.Join(buildArgs, " "))
	if err := runCmd(".", targetEnv(target, garbleScope), tool, buildArgs...); err != nil {
		return "", fmt.Errorf("build %s/%s: %w", target.GOOS, target.GOARCH, err)
	}
	return out, nil
}

// macOSDeploymentTarget is the minimum macOS version the darwin binaries must
// run on. Without this, clang/ld default to the deployment target of the SDK
// installed on the build machine, so a binary built on a newer macOS host
// silently refuses to launch on older ones (LaunchServices shows "you can't
// use this version of the app with this version of macOS").
const macOSDeploymentTarget = "14.0"

func targetEnv(target buildTarget, garbleScope string) []string {
	env := cleanEnv(os.Environ(), "GOOS", "GOARCH", "CGO_ENABLED", "CC", "CXX", "PKG_CONFIG", "GOGARBLE", "MACOSX_DEPLOYMENT_TARGET", "CGO_CFLAGS", "CGO_LDFLAGS")
	env = append(env,
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
		"CGO_ENABLED=1",
	)
	if garbleScope != "" {
		env = append(env, "GOGARBLE="+garbleScope)
	}
	if target.GOOS == "darwin" {
		env = append(env,
			"MACOSX_DEPLOYMENT_TARGET="+macOSDeploymentTarget,
			"CGO_CFLAGS=-mmacosx-version-min="+macOSDeploymentTarget,
			"CGO_LDFLAGS=-mmacosx-version-min="+macOSDeploymentTarget,
		)
	}
	cc, cxx := compilersForTarget(target)
	pkgConfig := pkgConfigForTarget(target)
	if cc != "" {
		env = append(env, "CC="+cc)
	}
	if cxx != "" {
		env = append(env, "CXX="+cxx)
	}
	if pkgConfig != "" {
		env = append(env, "PKG_CONFIG="+pkgConfig)
	}
	env = append(env, pkgConfigEnvForTarget(target)...)
	return env
}

// writeWindowsResource embeds the app icon, version info and a Common-Controls
// v6 manifest into a single .syso next to main.go. `go build` (and garble,
// which shells out to the same toolchain) auto-links any *_GOOS_GOARCH.syso
// file found in the main package directory, so the result is a single .exe
// with icon/manifest/version info baked in — no side-car files, no console
// window. This mirrors how Wails packages Windows builds.
func writeWindowsResource(workDir string, target buildTarget, appID, appIcon, appTitle, appVersion string) (string, error) {
	icoPath := filepath.Join(workDir, "app.ico")
	if err := prepareWindowsIcon(icoPath, appIcon); err != nil {
		return "", err
	}
	manifestPath := filepath.Join(workDir, "app.manifest")
	if err := os.WriteFile(manifestPath, []byte(windowsManifestXML(appID, appTitle, appVersion)), 0o644); err != nil {
		return "", err
	}
	rcPath := filepath.Join(workDir, "app.rc")
	rcSource := windowsResourceScript(icoPath, manifestPath, appTitle, appVersion)
	if err := os.WriteFile(rcPath, []byte(rcSource), 0o644); err != nil {
		return "", err
	}

	windres, err := windresPath(target)
	if err != nil {
		return "", err
	}
	sysoName := fmt.Sprintf("nexres_%s_%s.syso", target.GOOS, target.GOARCH)
	if err := runCmd(".", nil, windres, "-i", rcPath, "-O", "coff", "-o", sysoName); err != nil {
		return "", fmt.Errorf("windres: %w", err)
	}
	return sysoName, nil
}

func windresPath(target buildTarget) (string, error) {
	key := targetEnvKey(target)
	if v := os.Getenv("NEX_WINDRES_" + key); v != "" {
		return v, nil
	}
	name := "x86_64-w64-mingw32-windres"
	if runtime.GOOS == "windows" {
		name = "windres"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found; install mingw-w64 (bundles windres) or set NEX_WINDRES_%s", name, key)
}

// prepareWindowsIcon produces a multi-resolution .ico. If appIcon is already
// an .ico it's used as-is; otherwise it's rasterized at standard Windows
// icon sizes and each size is embedded as a PNG frame (supported by Windows
// Vista+ for any ICO frame size, so no BMP/DIB encoding is needed).
func prepareWindowsIcon(icoPath, appIcon string) error {
	if strings.EqualFold(filepath.Ext(appIcon), ".ico") {
		return copyFile(appIcon, icoPath)
	}
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	workDir := filepath.Dir(icoPath)
	frames := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		pngPath := filepath.Join(workDir, fmt.Sprintf("icon-%d.png", size))
		if err := rasterizeIconPNG(appIcon, pngPath, size); err != nil {
			return err
		}
		data, err := os.ReadFile(pngPath)
		if err != nil {
			return err
		}
		frames = append(frames, data)
	}
	return writeICO(icoPath, sizes, frames)
}

func rasterizeIconPNG(source, out string, size int) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("sips", "-s", "format", "png", "-z", strconv.Itoa(size), strconv.Itoa(size), source, "--out", out).Run()
	default:
		if path, err := exec.LookPath("magick"); err == nil {
			return exec.Command(path, source, "-resize", fmt.Sprintf("%dx%d", size, size), out).Run()
		}
		if path, err := exec.LookPath("convert"); err == nil {
			return exec.Command(path, "-background", "none", source, "-resize", fmt.Sprintf("%dx%d", size, size), out).Run()
		}
		return fmt.Errorf("no icon rasterizer found for the Windows .ico (need sips on macOS, or ImageMagick 'magick'/'convert' elsewhere); alternatively set the app Icon to a pre-made .ico")
	}
}

// writeICO packs pre-encoded PNG frames into a valid ICO container.
func writeICO(path string, sizes []int, pngs [][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	n := len(sizes)
	header := make([]byte, 6)
	binary.LittleEndian.PutUint16(header[2:], 1) // type: icon
	binary.LittleEndian.PutUint16(header[4:], uint16(n))
	if _, err := f.Write(header); err != nil {
		return err
	}

	offset := 6 + 16*n
	for i, size := range sizes {
		entry := make([]byte, 16)
		dim := byte(size)
		if size >= 256 {
			dim = 0 // 0 means 256 per the ICO spec
		}
		entry[0] = dim
		entry[1] = dim
		binary.LittleEndian.PutUint16(entry[4:], 1)  // color planes
		binary.LittleEndian.PutUint16(entry[6:], 32) // bits per pixel
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(pngs[i])))
		binary.LittleEndian.PutUint32(entry[12:], uint32(offset))
		offset += len(pngs[i])
		if _, err := f.Write(entry); err != nil {
			return err
		}
	}
	for _, data := range pngs {
		if _, err := f.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func windowsResourceScript(icoPath, manifestPath, appTitle, appVersion string) string {
	verComma := strings.ReplaceAll(windowsManifestVersion(appVersion), ".", ",")
	return fmt.Sprintf(`1 ICON "%s"
1 24 "%s"

1 VERSIONINFO
FILEVERSION     %s
PRODUCTVERSION  %s
FILEFLAGSMASK   0x3fL
FILEFLAGS       0x0L
FILEOS          0x40004L
FILETYPE        0x1L
FILESUBTYPE     0x0L
BEGIN
    BLOCK "StringFileInfo"
    BEGIN
        BLOCK "040904b0"
        BEGIN
            VALUE "FileDescription", "%s"
            VALUE "FileVersion", "%s"
            VALUE "ProductName", "%s"
            VALUE "ProductVersion", "%s"
        END
    END
    BLOCK "VarFileInfo"
    BEGIN
        VALUE "Translation", 0x409, 1200
    END
END
`, filepath.ToSlash(icoPath), filepath.ToSlash(manifestPath), verComma, verComma,
		rcEscape(appTitle), rcEscape(appVersion), rcEscape(appTitle), rcEscape(appVersion))
}

func rcEscape(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func windowsManifestXML(appID, appTitle, appVersion string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity version="%s" processorArchitecture="*" name="%s" type="win32"/>
  <description>%s</description>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
</assembly>
`, windowsManifestVersion(appVersion), xmlEscape(appID), xmlEscape(appTitle))
}

func validateBuildTargets(targets []buildTarget) error {
	missing := collectMissingBuildRequirements(targets)
	if len(missing) == 0 {
		return nil
	}
	return errors.New(formatMissingRequirements(missing))
}

func compilersForTarget(target buildTarget) (string, string) {
	key := strings.ToUpper(target.GOOS + "_" + target.GOARCH)
	key = strings.ReplaceAll(key, "-", "_")
	cc := os.Getenv("NEX_CC_" + key)
	cxx := os.Getenv("NEX_CXX_" + key)
	if cc == "" || cxx == "" {
		defaultCC, defaultCXX := defaultCrossCompilers(target)
		if cc == "" {
			cc = defaultCC
		}
		if cxx == "" {
			cxx = defaultCXX
		}
	}
	return cc, cxx
}

func pkgConfigForTarget(target buildTarget) string {
	key := targetEnvKey(target)
	if v := os.Getenv("NEX_PKG_CONFIG_" + key); v != "" {
		return v
	}
	if target.GOOS == "linux" {
		return "pkg-config"
	}
	return ""
}

func pkgConfigEnvForTarget(target buildTarget) []string {
	key := targetEnvKey(target)
	var env []string
	for _, name := range []string{"PKG_CONFIG_PATH", "PKG_CONFIG_LIBDIR", "PKG_CONFIG_SYSROOT_DIR"} {
		if v := os.Getenv("NEX_" + name + "_" + key); v != "" {
			env = append(env, name+"="+v)
		}
	}
	return env
}

func targetEnvKey(target buildTarget) string {
	key := strings.ToUpper(target.GOOS + "_" + target.GOARCH)
	return strings.ReplaceAll(key, "-", "_")
}

type missingRequirement struct {
	Target buildTarget
	Name   string
	Hint   string
}

func collectMissingBuildRequirements(targets []buildTarget) []missingRequirement {
	var missing []missingRequirement
	seen := map[string]bool{}
	add := func(target buildTarget, name, hint string) {
		key := target.GOOS + "/" + target.GOARCH + ":" + name
		if seen[key] {
			return
		}
		seen[key] = true
		missing = append(missing, missingRequirement{Target: target, Name: name, Hint: hint})
	}

	for _, target := range targets {
		switch target.GOOS {
		case "darwin":
			if runtime.GOOS != "darwin" {
				add(target, "macOS SDK/Xcode", "darwin targets require a macOS host with Xcode Command Line Tools in this build script")
				continue
			}
			for _, tool := range []string{"xcrun", "sips", "iconutil"} {
				if _, err := exec.LookPath(tool); err != nil {
					add(target, tool, "install Xcode Command Line Tools with: xcode-select --install")
				}
			}
		case "linux":
			cc, cxx := compilersForTarget(target)
			key := targetEnvKey(target)
			ccHint := "install a Linux " + target.GOARCH + " cross compiler or set NEX_CC_" + key
			cxxHint := "install a Linux " + target.GOARCH + " C++ cross compiler or set NEX_CXX_" + key
			if runtime.GOOS == "linux" && target.GOARCH == runtime.GOARCH {
				ccHint = "install build-essential/gcc or set NEX_CC_" + key
				cxxHint = "install build-essential/g++ or set NEX_CXX_" + key
			}
			checkPath(target, cc, ccHint, add)
			checkPath(target, cxx, cxxHint, add)
			pkgConfig := pkgConfigForTarget(target)
			if checkPath(target, pkgConfig, "install pkg-config or set NEX_PKG_CONFIG_"+key, add) {
				if !pkgConfigHasLinuxWebviewPackages(target, pkgConfig) {
					add(target, "pkg-config gtk+-3.0 webkit2gtk-4.0 gio-2.0", "install the Linux target GTK/WebKitGTK/GLib development packages or set NEX_PKG_CONFIG_PATH_"+key+" / NEX_PKG_CONFIG_LIBDIR_"+key+" / NEX_PKG_CONFIG_SYSROOT_DIR_"+key)
				}
			}
		case "windows":
			cc, cxx := compilersForTarget(target)
			checkPath(target, cc, "install mingw-w64 and keep its compiler on PATH, or set NEX_CC_WINDOWS_AMD64", add)
			checkPath(target, cxx, "install mingw-w64 and keep its compiler on PATH, or set NEX_CXX_WINDOWS_AMD64", add)
			windresName := "x86_64-w64-mingw32-windres"
			if runtime.GOOS == "windows" {
				windresName = "windres"
			}
			checkPath(target, windresName, "install mingw-w64 (bundles windres, needed to embed the icon/manifest) or set NEX_WINDRES_WINDOWS_AMD64", add)
		}
	}
	return missing
}

func checkPath(target buildTarget, name, hint string, add func(buildTarget, string, string)) bool {
	if name == "" {
		return true
	}
	if _, err := exec.LookPath(name); err != nil {
		add(target, name, hint)
		return false
	}
	return true
}

func pkgConfigHasLinuxWebviewPackages(target buildTarget, pkgConfig string) bool {
	// gio-2.0 backs internal/system/launcher_linux.go (native open/reveal-in-
	// folder); it's a transitive dependency of gtk+-3.0 on every distro that
	// ships libgtk-3-dev, but checked explicitly here since it's a distinct
	// pkg-config module.
	cmd := exec.Command(pkgConfig, "--exists", "gtk+-3.0", "webkit2gtk-4.0", "gio-2.0")
	cmd.Env = append(os.Environ(), pkgConfigEnvForTarget(target)...)
	return cmd.Run() == nil
}

func formatMissingRequirements(missing []missingRequirement) string {
	var b strings.Builder
	b.WriteString("missing build requirement(s):\n")
	for _, m := range missing {
		fmt.Fprintf(&b, "  - %s/%s: %s\n", m.Target.GOOS, m.Target.GOARCH, m.Name)
		if m.Hint != "" {
			fmt.Fprintf(&b, "    %s\n", m.Hint)
		}
	}
	b.WriteString("\nRun `go run build.go requirements` for the complete target checklist.")
	return b.String()
}

func emptyDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func cleanEnv(env []string, keys ...string) []string {
	block := map[string]bool{}
	for _, key := range keys {
		block[key] = true
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		key, _, ok := strings.Cut(kv, "=")
		if !ok || block[key] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func defaultCrossCompilers(target buildTarget) (string, string) {
	if target.GOARCH != "amd64" {
		return "", ""
	}
	switch target.GOOS {
	case "windows":
		if runtime.GOOS == "windows" {
			return "gcc", "g++"
		}
		return "x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-g++"
	case "linux":
		if runtime.GOOS == "linux" {
			return "gcc", "g++"
		}
	}
	return "", ""
}

func packagePlatformApp(binaryPath, appTitle, appID, appIcon, appVersion, appBuild string, target buildTarget) error {
	switch target.GOOS {
	case "darwin":
		return packageDarwinApp(binaryPath, appTitle, appID, appIcon, appVersion, appBuild, target)
	case "linux":
		return packageLinuxApp(binaryPath, appTitle, appID, appIcon, target)
	case "windows":
		return packageWindowsApp(binaryPath, appTitle, target)
	default:
		return nil
	}
}

// targetOutDir isolates each GOOS/GOARCH combination in its own directory so
// the packaged app/binary itself keeps a clean, non-composite app name.
func targetOutDir(target buildTarget) string {
	return filepath.Join(outDir, target.GOOS+"-"+target.GOARCH)
}

func packageDarwinApp(binaryPath, appTitle, appID, appIcon, appVersion, appBuild string, target buildTarget) error {
	targetDir := targetOutDir(target)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	appPath := filepath.Join(targetDir, appTitle+".app")
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
	if err := copyFile(binaryPath, filepath.Join(macosDir, appID)); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(macosDir, appID), 0o755); err != nil {
		return err
	}
	if err := writeDarwinInfoPlist(appPath, appTitle, appID, appVersion, appBuild); err != nil {
		return err
	}
	if err := writeDarwinIcon(resDir, appIcon); err != nil {
		fmt.Printf("→ app icon skipped: %v\n", err)
	}
	fmt.Printf("→ packaged %s\n", appPath)
	return nil
}

func packageLinuxApp(binaryPath, appTitle, appID, appIcon string, target buildTarget) error {
	appDir := filepath.Join(targetOutDir(target), appID)
	binDir := filepath.Join(appDir, "bin")
	iconExt := iconExtension(appIcon)
	iconSizeDir := "scalable"
	if iconExt != ".svg" {
		iconSizeDir = "256x256"
	}
	iconsDir := filepath.Join(appDir, "share", "icons", "hicolor", iconSizeDir, "apps")
	appsDir := filepath.Join(appDir, "share", "applications")
	if err := os.RemoveAll(appDir); err != nil {
		return err
	}
	for _, dir := range []string{binDir, iconsDir, appsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	binPath := filepath.Join(binDir, appID)
	if err := copyFile(binaryPath, binPath); err != nil {
		return err
	}
	if err := os.Chmod(binPath, 0o755); err != nil {
		return err
	}
	if err := copyFile(appIcon, filepath.Join(iconsDir, appID+iconExt)); err != nil {
		return err
	}
	desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=%s desktop app
Exec=%s
Icon=%s
Terminal=false
Categories=Development;
`, appTitle, appTitle, appID, appID)
	if err := os.WriteFile(filepath.Join(appsDir, appID+".desktop"), []byte(desktop), 0o644); err != nil {
		return err
	}
	fmt.Printf("→ packaged %s\n", appDir)
	return nil
}

// packageWindowsApp produces a single self-contained .exe — icon, version
// info and manifest are already embedded at compile time by
// writeWindowsResource, so there is nothing else to place next to it.
func packageWindowsApp(binaryPath, appTitle string, target buildTarget) error {
	targetDir := targetOutDir(target)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(targetDir, appTitle+".exe")
	if err := copyFile(binaryPath, dest); err != nil {
		return err
	}
	fmt.Printf("→ packaged %s\n", dest)
	return nil
}

func writeDarwinInfoPlist(appPath, appTitle, executable, appVersion, appBuild string) error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>%s</string>
  <key>CFBundleDisplayName</key><string>%s</string>
  <key>CFBundleIdentifier</key><string>%s</string>
  <key>CFBundleVersion</key><string>%s</string>
  <key>CFBundleShortVersionString</key><string>%s</string>
  <key>CFBundleExecutable</key><string>%s</string>
  <key>CFBundleIconFile</key><string>nex</string>
  <key>LSMinimumSystemVersion</key><string>10.13</string>
</dict>
</plist>
`, plistEscape(appTitle), plistEscape(appTitle), plistEscape(executable), plistEscape(appBuild), plistEscape(appVersion), plistEscape(executable))
	return os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), []byte(plist), 0o644)
}

func writeDarwinIcon(resourcesDir, source string) error {
	if strings.EqualFold(filepath.Ext(source), ".icns") {
		return copyFile(source, filepath.Join(resourcesDir, "nex.icns"))
	}
	iconset := filepath.Join(os.TempDir(), "nex-build.iconset")
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

func appVersionBuildFromVersion() (string, string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "version.go", nil, 0)
	if err != nil {
		return "", "", fmt.Errorf("reading release version from version.go: %w", err)
	}
	values := map[string]string{}
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if name.Name != "AppVersion" && name.Name != "AppBuild" {
					continue
				}
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				v, err := strconv.Unquote(lit.Value)
				if err == nil {
					values[name.Name] = strings.TrimSpace(v)
				}
			}
		}
	}
	version := sanitizeReleasePart(scriptEnvDefault("NEX_APP_VERSION", values["AppVersion"]))
	build := sanitizeReleasePart(scriptEnvDefault("NEX_APP_BUILD", values["AppBuild"]))
	if version == "" || build == "" {
		return "", "", fmt.Errorf("version.go must define string constants AppVersion and AppBuild")
	}
	return version, build, nil
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

func sanitizeReleasePart(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_'
		if ok {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), ".-_")
}

func iconExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".svg", ".png", ".ico", ".icns":
		return ext
	default:
		return ".svg"
	}
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

func plistEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return strings.ReplaceAll(s, "'", "&apos;")
}

func xmlEscape(s string) string {
	return plistEscape(s)
}

func windowsManifestVersion(version string) string {
	parts := strings.Split(version, ".")
	out := []string{"0", "0", "0", "0"}
	for i := 0; i < len(parts) && i < len(out); i++ {
		var b strings.Builder
		for _, r := range parts[i] {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			out[i] = b.String()
		}
	}
	return strings.Join(out, ".")
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

func doClean() error {
	_ = os.RemoveAll(releaseRoot)
	// Remove dist contents but keep dist/index.html placeholder
	entries, _ := os.ReadDir(filepath.Join(frontendDir, "dist"))
	placeholder := []string{"index.html"}
	for _, e := range entries {
		if !slices.Contains(placeholder, e.Name()) {
			_ = os.RemoveAll(filepath.Join(frontendDir, "dist", e.Name()))
		}
	}
	fmt.Println("cleaned release/ and frontend/dist (kept placeholder)")
	return nil
}

func runCmd(dir string, env []string, name string, args ...string) error {
	fmt.Printf("→ (%s) %s %s\n", dir, name, strings.Join(args, " "))
	c := exec.Command(name, args...)
	c.Dir = dir
	if env != nil {
		c.Env = env
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
