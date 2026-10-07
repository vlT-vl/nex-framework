# Changelog

All notable changes to the nex framework are documented here, newest first.
This file lists **what** changed per version/build — technical detail, root
cause, and verification notes live in `handoff.md` only.

## [0.1.1] - R071026 - 2026-10-07

### Changed

- Go toolchain bumped to 1.27.1
- `github.com/ncruces/zenity` bumped to v0.10.15
- Frontend: React/React DOM 19.3.0, Vite 8.3.2, `@vitejs/plugin-react` 6.1.2
  (`react-icons` stays at 5.7.0, already latest stable)
- `github.com/webview/webview_go` confirmed at the latest available commit
  (no newer upstream commit exists)
- Documented explicit macOS build compatibility: Sonoma (14) through Golden
  Gate (27), the last four annual macOS releases

### Fixed

- Packaged `.app` bundles (`dev.go` and `build.go`) now report
  `LSMinimumSystemVersion` `14.0`, aligned with the actual
  `MACOSX_DEPLOYMENT_TARGET` used at build time — previously hardcoded to the
  unrelated `10.13`
- README/DOCS masthead logo no longer relies on a `prefers-color-scheme`
  query embedded inside the SVG itself, which doesn't reliably switch on
  GitHub and other portals. It's now a `<picture>` element with two static
  `res/nex-light.svg`/`res/nex-dark.svg` images, matching GitHub's documented
  approach for theme-adaptive README images

## [0.1.0] - R110726 - 2026-07-11

### Added

- Automatic ad-hoc code signing (`codesign --sign -`) of macOS `.app` bundles,
  in both `dev.go` and `build.go`

### Fixed

- "nex" masthead logo unreadable in the Docs/License modal when the in-app
  theme diverges from the OS theme

## [0.1.0] - R070726 - 2026-07-07

### Added

- Native OS clipboard (NSPasteboard / Win32 `user32` / GTK `GtkClipboard`),
  replacing the previous shell-subprocess implementation
- Native Edit menu on macOS (Cut/Copy/Paste/Select All), moved into the
  framework core — no longer each app's own responsibility
- Native `window.alert/confirm/prompt` support on macOS
- Backend-side confirmation dialog for privileged APIs, independent of the
  frontend
- New `sys.trash.empty` / `trash.empty()` API
- Pure Go/cgo/syscall rewrite of every OS diagnostics collector (host, CPU,
  uptime, memory, disks, screen, appearance, power, protocol/file
  associations) — no CLI subprocess calls left
- Native rewrite of `shell.openURL` / `openPath` / `showInFolder`
  (NSWorkspace / `shell32` / GIO) — no external process launched

### Fixed

- Linux build host now targets its actual architecture (`amd64`/`arm64`)
  instead of always `amd64`
- `sys.app.exit` releases the single-instance lock before `os.Exit`

## [0.1.1] - pre-R070726

- `sys.clipboard.isAvailable` / `dialog.isSupported` capability checks for
  Linux
- `clipboard.pasteHandler(onChange)` helper
- `dialog_unsupported` error code

## [0.1.0] - R060726 - 2026-07-06

### Added

- License button + modal in the showcase UI; `FileModal` generalized from the
  previous `DocsModal`
- Windows build aligned with native desktop app conventions: no console
  window, icon/manifest/version info embedded directly in the `.exe`
- Per-OS/arch release output folders, plain artifact names (no more composite
  `<Title>-darwin-arm64` style names)

### Fixed

- macOS binaries tied to the build host's SDK version — deployment target now
  pinned explicitly
- `app.restart()` never restarting the app (single-instance lock race)
- Missing Edit menu on macOS (Cmd+C/V/X/A not working)
- `internal/meta.Stack()` not reporting go/webview/zenity versions in
  packaged (garble-obfuscated) builds
