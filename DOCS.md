<p align="center">
  <img src="res/nex.svg" alt="nex" width="150" />
</p>

<p align="center">
  Cross-platform desktop framework: React/Vite frontend + Go backend<br/>
  <sub>Native OS webview · Native APIs · Embedded frontend assets · Garble obfuscation</sub>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0--R070726-blue?style=flat-square" alt="version"/>
  <img src="https://img.shields.io/badge/go-1.26.4-00ADD8?style=flat-square&logo=go" alt="go"/>
  <img src="https://img.shields.io/badge/react-19.2.7-61DAFB?style=flat-square&logo=react&logoColor=white" alt="react"/>
  <img src="https://img.shields.io/badge/react--icons-5.7.0-61DAFB?style=flat-square&logo=react&logoColor=white" alt="react-icons"/>
  <img src="https://img.shields.io/badge/vite-8.1.3-646CFF?style=flat-square&logo=vite&logoColor=white" alt="vite"/>
  <img src="https://img.shields.io/badge/license-proprietary-critical?style=flat-square" alt="license"/>
</p>

---

## Documentation scope

This file is the complete nex framework manual: architecture, development,
build system, environment model, security model, and native API reference.
For the short project entry point, see [README.md](README.md).

---

## What is nex?

nex builds desktop applications with a React/Vite UI and a Go backend. The Vite
build output is embedded into the Go binary with `//go:embed`. At runtime nex
starts a loopback HTTP server, serves the embedded SPA, injects a session token
before page scripts run, and opens a native OS webview.

```
┌─────────────────────────────────────────────────────┐
│  Native OS webview  (WebView2 · WebKit · WebKitGTK) │
│  → http://127.0.0.1:<random-port>                   │
└───────────────────┬─────────────────────────────────┘
                    │  RPC  POST /api/rpc  X-nex-Token
                    │  Events  window.__nexEmit → CustomEvent
┌───────────────────┴─────────────────────────────────┐
│  Go HTTP server  (loopback only)                    │
│   /            → embedded Vite SPA  (embed.FS)      │
│   /api/rpc     → typed RPC dispatch                 │
│   /api/session → session metadata                   │
└─────────────────────────────────────────────────────┘
```

---

## Features

| Category | What nex provides |
|---|---|
| Packaging | Single binary; frontend assets embedded via `//go:embed` |
| Webview | Native: WebView2 (Windows), WebKit (macOS), WebKitGTK (Linux) |
| Icons | `react-icons` icon components for the showcase UI |
| Stack badges | Version badges for nex, Go, React, React Icons, Vite, Zenity, and webview |
| RPC | `POST /api/rpc` with typed JSON responses and errors |
| Events | `app.Emit(event, payload)` to `nex.on(event, cb)` |
| Session security | Per-launch token, sliding expiry, loopback-only, Origin check |
| Dialogs | Open file/folder, Save file, Message box |
| Notifications | Native OS toast notifications with title, text, and icon |
| Filesystem | read, write, list, exists, stat, mkdir, remove, rename |
| Shell | openURL, openPath, showInFolder, exec/run/start native commands |
| Docs UI | Built-in rendered docs modal for `DOCS.md` |
| Brand UI | Reusable `NexLogo` React component with the nex cube and wordmark |
| Window | title, size, info, fullscreen, reload, print, eval |
| OS info | host, user, runtime, process, network, disks, memory, time |
| Desktop/runtime | appearance, power, shortcuts, protocol/file association status, updater check, recent docs |
| Env model | `VITE_*` vars are public; all others stay backend-only |
| Build | `go run build.go` creates obfuscated artifacts |
| Dev mode | `go run dev.go` starts Vite HMR + Go backend |
| Obfuscation | `build`, `release`, and `obfuscate` use garble by default |

---

## Native dialog layer

nex uses [`github.com/ncruces/zenity`](https://github.com/ncruces/zenity) as
part of the framework for native dialogs and OS notifications. It keeps the
public nex API small while delegating platform-specific dialog/toast behavior to
a focused cross-platform Go library.

The showcase stack badges are resolved from project metadata instead of being
duplicated in UI code: React, React Icons, and Vite come from
`frontend/package.json`; Go, zenity, and webview_go come from
`framework.stack()`.

---

## Requirements

| Tool | Version |
|---|---|
| Go | 1.26.4+ |
| Node | 20.19+ or 22.12+ with npm |
| garble | installed automatically by `build.go` when missing |
| C compiler | required by cgo/webview bindings |
| pkg-config | required for Linux WebKitGTK builds |

Vite 8 requires Node `^20.19.0` or `>=22.12.0`; older Node 18 runtimes are no
longer supported by the frontend toolchain.

### macOS

```bash
xcode-select --install
```

The macOS targets are always built as both `darwin/amd64` and `darwin/arm64`.
When the build runs on macOS, nex produces macOS `amd64`, macOS `arm64`, and
Windows `amd64` artifacts. Linux is intentionally not built from macOS; the
Linux release must be produced from a Linux build host.

### Linux (Debian/Ubuntu)

```bash
sudo apt install build-essential pkg-config \
  libgtk-3-dev libwebkit2gtk-4.0-dev \
  zenity
```

`xdg-utils` (`xdg-open`/`xdg-mime`) is **not** required: `shell.openURL`/
`shell.openPath`/`shell.showInFolder` call GIO directly
(`g_app_info_launch_default_for_uri` / the `org.freedesktop.FileManager1`
D-Bus service), and `protocol.status`/`fileAssociations.status` parse
`mimeapps.list`/`shared-mime-info` globs directly — no subprocess, no CLI
dependency.

Linux artifacts are built from Linux. The host needs the Linux C/C++ compiler
and the GTK/WebKitGTK/GLib development metadata used by the native webview
backend and by the clipboard/screen/launcher native APIs:

```text
gcc
g++
pkg-config packages: gtk+-3.0 webkit2gtk-4.0 gio-2.0
```

When the build runs on Linux, nex produces a Linux artifact for the **host's
own architecture** (`amd64` or `arm64`) plus Windows `amd64`. Cross-arch Linux
builds (e.g. producing `linux/arm64` from an `amd64` host) are not attempted by
default because the cgo/GTK toolchain needs a matching target sysroot this
script does not set up; a Linux `arm64` release is produced by running the
build natively on an `arm64` Linux host (set `NEX_CC_LINUX_ARM64` /
`NEX_CXX_LINUX_ARM64` / `NEX_PKG_CONFIG_*_LINUX_ARM64` if a cross sysroot is
available and cross-building is still desired). macOS artifacts are not built
from Linux because they require the Apple SDK.

### Windows

Windows builds use `mingw-w64` by default. From macOS or Linux the expected
cross compiler names are:

```text
x86_64-w64-mingw32-gcc
x86_64-w64-mingw32-g++
```

When the build runs on Windows, nex produces Windows `amd64` artifacts.

The target machine must have Microsoft Edge WebView2 Runtime available.

### Build host coverage

`build.go` selects targets from the host OS so the single build command does
not attempt native-webview cross builds for a different OS/architecture:

| Build host | Targets produced by `make build` |
|---|---|
| macOS | macOS `amd64`, macOS `arm64`, Windows `amd64` |
| Linux (`amd64` host) | Linux `amd64`, Windows `amd64` |
| Linux (`arm64` host) | Linux `arm64`, Windows `amd64` |
| Windows | Windows `amd64` |

Linux builds are intentionally produced on Linux, targeting the host's own
architecture (no amd64⇄arm64 cross build without a manually configured cross
sysroot — see `NEX_CC_LINUX_ARM64` etc. above). macOS builds are intentionally
produced on macOS. Windows `amd64` can be built from macOS/Linux with
`mingw-w64`, or natively on Windows.

### Build requirements check

Before building every target selected for the current host, nex validates the
host requirements. The same check can be run directly:

```bash
go run build.go requirements
make requirements
```

`doctor` is an alias:

```bash
go run build.go doctor
make doctor
```

If a required cross toolchain uses a custom path/name, set one of these
variables before building:

```bash
NEX_CC_LINUX_AMD64=/path/to/linux-gcc
NEX_CXX_LINUX_AMD64=/path/to/linux-g++
NEX_PKG_CONFIG_LINUX_AMD64=/path/to/pkg-config
NEX_PKG_CONFIG_PATH_LINUX_AMD64=/path/to/linux/pkgconfig
NEX_PKG_CONFIG_LIBDIR_LINUX_AMD64=/path/to/linux/pkgconfig
NEX_PKG_CONFIG_SYSROOT_DIR_LINUX_AMD64=/path/to/linux/sysroot

NEX_CC_WINDOWS_AMD64=/path/to/x86_64-w64-mingw32-gcc
NEX_CXX_WINDOWS_AMD64=/path/to/x86_64-w64-mingw32-g++
```

---

## Project structure

```
nex-framework/
├── .env                  # Backend-only vars + public VITE_* frontend vars
├── config/
│   └── config.go         # .env loader
├── frontend/
│   ├── package.json      # React 19 + Vite 8
│   ├── vite.config.js
│   ├── index.html
│   ├── dist/
│   │   └── index.html    # Static fallback before first Vite build
│   └── src/
│       ├── main.jsx
│       ├── App.jsx
│       ├── index.css
│       ├── components/
│       │   └── NexLogo.jsx # Reusable nex logo; imports CenturyGothic.ttf
│       └── lib/
│           └── nex.js    # JS client bridge
├── internal/
│   ├── app/
│   │   └── app.go        # HTTP server, RPC, session, webview wiring
│   ├── core/
│   │   └── core.go       # Context, HandlerFunc, Host, RPCError
│   ├── meta/
│   │   └── meta.go       # nex framework metadata
│   ├── session/
│   │   └── session.go    # Token generation and sliding expiry
│   └── system/
│       └── system.go     # Built-in sys.* handlers
├── nex/
│   └── nex.go            # Public API facade
├── res/
│   ├── CenturyGothic.ttf # Brand font for the nex wordmark
│   ├── nexcube.svg       # Blue cube symbol used in the React UI
│   ├── nexicon.svg       # App/favicon icon source
│   └── nex.svg           # README/DOCS masthead logo — single SVG, self-switches
│                         #   light/dark ink via an embedded `@media (prefers-color-scheme)`
│                         #   rule, embedded CenturyGothic @font-face, same cube artwork
│                         #   as nexcube.svg
├── build.go              # Build/release/obfuscate/clean
├── dev.go                # Local Vite HMR + Go runner
├── main.go               # App entry point and frontend embed
├── version.go            # Fallback app version metadata
├── .env                  # App identity and runtime environment
├── Makefile
└── go.mod
```

Dependency graph:

```
config  ──┐
session ──┼──► core ──► system ──┐
          │                      ├──► app ──► nex ──► main
meta ────────────────────────────┘
          └──────────────────────┘
webview_go (cgo) ────────────────► app
zenity     (native dialogs) ─────► system
garble     (build tool) ─────────► build/release
```

---

## Brand component

`frontend/src/components/NexLogo.jsx` is the reusable nex logo component used by
the showcase UI. It imports `res/CenturyGothic.ttf` with Vite's `?url` asset
loader at component level, injects the `@font-face`, renders `res/nexcube.svg`,
and applies the animated cube/wordmark entrance from `frontend/src/index.css`.

Basic use:

```jsx
import { NexLogo } from "./components/NexLogo.jsx";

<NexLogo />
<NexLogo animated={false} />
<NexLogo className="brand-intro" />
```

---

## Using the framework

```go
package main

import (
    "embed"
    "encoding/json"

    nexpkg "nex/nex"
)

//go:embed frontend/dist
var distFS embed.FS

func main() {
    a := nexpkg.New(nexpkg.Config{
        Title:        "My App",
        Width:        1100,
        Height:       760,
        Icon:         "res/nexicon.svg",
        Dist:         distFS,
        DistDir:      "frontend/dist",
        DevServerURL: "http://127.0.0.1:5179",
        SingleInstanceID: "com.example.myapp",
        OnDomReady: func(a *nexpkg.App) {
            a.Emit("ready", map[string]any{"ok": true})
        },
        OnSecondInstance: func(a *nexpkg.App, args []string) {
            a.Emit("app.second-instance", map[string]any{"args": args})
        },
    })

    a.Handle("user.greet", func(c *nexpkg.Context, params json.RawMessage) (any, error) {
        var p struct {
            Name string `json:"name"`
        }
        if err := c.Bind(params, &p); err != nil {
            return nil, nexpkg.Errorf("bad_request", "%v", err)
        }
        return map[string]string{"message": "Hello, " + p.Name + "!"}, nil
    })

    if err := a.Run(); err != nil {
        panic(err)
    }
}
```

The `sys.*` namespace is reserved for built-in APIs.

Framework identity is exposed by the public Go facade and is always sourced
from `internal/meta`, never from app config:

```go
nexpkg.Name
nexpkg.FrameworkVersion()
nexpkg.FrameworkBuild()
nexpkg.FrameworkUpdated()
nexpkg.FrameworkAuthor()
```

Frontend calls:

```js
import { call, on, dialog, fs, shell, app, framework } from "./lib/nex.js";

const { message } = await call("user.greet", { name: "Ada" });
const off = on("tick", ({ n, at }) => console.log(n, at));
const info = await app.info();
const fw = await framework.info(); // { id: "nex@0.1.0", ... }
const result = await dialog.open({ title: "Open" });
const entries = await fs.list("/tmp");
const proc = await shell.exec("uname -a", { timeoutMs: 5000 });
const started = await shell.start("echo nex");
off();
```

Theme helpers are available to every frontend screen:

```js
import {
  applyTheme,
  getThemePreference,
  getSystemTheme,
  resolveTheme,
  setThemePreference,
  toggleTheme,
  watchSystemTheme,
} from "./lib/theme.js";

setThemePreference("system"); // follows the OS light/dark mode
setThemePreference("light");
setThemePreference("dark");
```

The template defaults to `system`, applies the resolved theme before React
mounts, and keeps listening to OS theme changes while `system` is selected.

The template also includes a `docs` button in the top bar. It opens a rendered
markdown modal with the bundled `DOCS.md` content. The markdown is imported by
Vite as raw text and rendered by the local no-dependency markdown helper, so the
docs modal works in dev and packaged apps without relying on the current
working directory.

---

## Development

Development uses a dedicated runner so build and dev logic stay separate:

```bash
go run dev.go
```

This first runs `go mod tidy` so `go.sum` is ready, installs frontend packages
if `node_modules/` is missing, stops any previous process listening on
`127.0.0.1:5179` and `127.0.0.1:34115`, starts a fresh Vite server in
`frontend/`, then runs the Go app with:

```text
APP_DEV=1
CGO_ENABLED=1
```

The equivalent Make shortcut is:

```bash
make dev
```

`dev.go` does not create release artifacts. Vite serves the nex frontend on
`http://127.0.0.1:5179`; the Go API server uses `127.0.0.1:34115` in dev mode.
Port `5173` remains free for unrelated frontend-only projects. Each dev run
restarts the dedicated nex Vite server and backend to avoid stale sessions.

The dev runner is platform-aware. It always uses the same frontend/backend
ports and native webview path. On macOS it runs through a temporary
`<Title>-dev.app` bundle so the dock/app switcher uses the app `Icon`, `Title`,
and `SingleInstanceID` resolved from `.env` through `main.go`; Windows and Linux use the native webview
runtime directly during dev and keep the same frontend favicon/brand assets.

---

## Build system

```bash
go run build.go [command]
```

| Command | Action |
|---|---|
| `build` | Tidy Go modules, install frontend packages if missing, run `npm run build`, then obfuscated garble builds for the targets supported by the current host into `release/<version>-<build>/` |
| `release` | Like build, with `-s -w` added on top |
| `obfuscate` | Alias for obfuscated release build |
| `requirements` | Print and validate the host toolchains needed by the selected targets |
| `doctor` | Alias for `requirements` |
| `clean` | Remove `release/` and generated `frontend/dist` assets |

`build.go` and `dev.go` load `.env` first, then resolve app identity through
the `envDefault(...)` values used by `main.go`. `NEX_APP_NAME` drives the
displayed app name and platform packaging names. `NEX_APP_ICON` points to the
app icon source used by dev/build packaging; if omitted, the template falls
back to `res/nexicon.svg`. `NEX_APP_ID` drives the single-instance id and
platform bundle/application identifiers, with a title slug as fallback.
`NEX_APP_VERSION` and `NEX_APP_BUILD` drive release folder naming; the app
version comes exclusively from `.env`. Every build writes into
`release/<version>-<build>/`, for example `release/0.1.0-R070726/`.

One `go run build.go build` / `make build` run builds every target supported
by the current host:

| Build host | Targets | Output |
|---|---|---|
| macOS | macOS `amd64` + `arm64`, Windows `amd64` | `darwin-amd64/<Title>.app`, `darwin-arm64/<Title>.app`, `windows-amd64/<Title>.exe` |
| Linux (`amd64` host) | Linux `amd64`, Windows `amd64` | `linux-amd64/<title-slug>/`, `windows-amd64/<Title>.exe` |
| Linux (`arm64` host) | Linux `arm64`, Windows `amd64` | `linux-arm64/<title-slug>/`, `windows-amd64/<Title>.exe` |
| Windows | Windows `amd64` | `windows-amd64/<Title>.exe` |

Each `GOOS-GOARCH` combination gets its own subdirectory under
`release/<version>-<build>/`, so the packaged app/binary itself always keeps
the plain app name (`<Title>.app`, `<Title>.exe`) instead of a composite name —
the OS/arch distinction lives in the directory, not the artifact name.

Because nex uses cgo and native webview bindings, the machine running the
single build command must have the target C/C++ toolchains available. Linux is
built on Linux with `gcc/g++`, while Windows cross builds use
`x86_64-w64-mingw32-gcc/g++` when no nex override variable is set. The same
mingw-w64 toolchain provides `windres`, used to embed the app icon, version
info and manifest directly into the Windows `.exe` (see the packaging table
below) — no separate compiler install is required for it.

Before deleting or recreating the release folder, `build.go` checks that the
required compilers are available. For Linux it also checks `pkg-config` and the
`gtk+-3.0` / `webkit2gtk-4.0` development metadata required by the native
webview backend. If a toolchain is missing, the command fails early and prints
the exact missing requirement. Custom toolchain paths can be provided per
target:

```bash
NEX_CC_LINUX_AMD64=/path/to/linux-gcc
NEX_CXX_LINUX_AMD64=/path/to/linux-g++
NEX_PKG_CONFIG_LINUX_AMD64=/path/to/pkg-config
NEX_PKG_CONFIG_PATH_LINUX_AMD64=/path/to/linux/pkgconfig
NEX_PKG_CONFIG_LIBDIR_LINUX_AMD64=/path/to/linux/pkgconfig
NEX_PKG_CONFIG_SYSROOT_DIR_LINUX_AMD64=/path/to/linux/sysroot
NEX_CC_WINDOWS_AMD64=/path/to/x86_64-w64-mingw32-gcc
NEX_CXX_WINDOWS_AMD64=/path/to/x86_64-w64-mingw32-g++
```

If `garble` is not on `PATH`, `build.go` installs it automatically with
`go install mvdan.cc/garble@latest` and uses the resulting binary. Garble flags:
`-literals -seed=random`. `build.go` also sets `GOGARBLE` to the current module
path from `go.mod`, for example `nex,nex/...`, so nex code is obfuscated while
native dependencies such as `zenity` and `webview_go` are left intact. `-tiny`
is intentionally excluded because of cgo/native webview compatibility.

After the binary is built, `build.go` runs a platform packaging step:

| OS | Packaging output |
|---|---|
| macOS | `release/<version>-<build>/darwin-<arch>/<Title>.app`, with `.env` app identity in `Info.plist` and `Icon` converted to `nex.icns` |
| Linux | `release/<version>-<build>/linux-<arch>/<app-id>/` with binary, desktop file, and the configured `Icon` installed under hicolor icons |
| Windows | `release/<version>-<build>/windows-<arch>/<Title>.exe` — a single self-contained file. The `Icon` (rasterized to a multi-resolution `.ico` via `sips` on macOS or ImageMagick elsewhere), a Common-Controls v6 manifest, and version info are compiled with `windres` into a `.syso` that Go links in automatically, then deleted. No side-car icon/manifest files. The binary is always linked with `-H windowsgui` (both `build` and `release`), so it never opens a console window — matching the result a framework like Wails produces. |

For diagnostics only, `--plain` skips garble:

```bash
go run build.go build --plain
```

---

## Environment

nex loads `.env` from the project root at app startup, if present. The default
file is safe to leave unchanged: app identity values match the template
fallbacks in `main.go` and `version.go`. A different env file can be selected
with `NEX_ENV_FILE=/path/to/file`.

Variables without the `VITE_` prefix stay backend-only. Variables with the
`VITE_` prefix are exposed to the frontend through `app.info().public`.

`dev.go` and `build.go` also load the same `.env`, so app identity is shared by
runtime APIs, dev packaging, release folder names, and platform metadata. For
`NEX_APP_*` identity values, `.env` is authoritative; process environment
values are used only when a key is missing from the file. Other runtime
variables keep the conservative loader behavior and do not overwrite existing
process environment values.

```env
# App identity. These values belong to the app, not to the nex framework.
NEX_APP_NAME=nex-app-template
NEX_APP_VERSION=0.1.0
NEX_APP_BUILD=R070726
NEX_APP_UPDATED=7 Luglio 2026
NEX_APP_AUTHOR=© 2026 vlT di Veronesi Lorenzo
NEX_APP_ICON=res/nexicon.svg
NEX_APP_ID=dev.vlt.nex-app-template

# Dev runtime.
NEX_DEV_SERVER_URL=http://127.0.0.1:5179

# Backend-only examples. These are visible only to Go code through a.Env() or os.Getenv.
# NEX_ENV=development
# NEX_LOG_LEVEL=debug
# NEX_API_HOST=127.0.0.1

# Frontend-public examples. These are included in app.info().public.
# VITE_APP_TITLE=nex-app-template
# VITE_APP_ICON=res/nexicon.svg
# VITE_NEX_APP=nex-app-template
# VITE_NEX_CHANNEL=development
# VITE_NEX_BRAND=nex
```

| Variable kind | Prefix | Visible to frontend | Read from |
|---|---|---|---|
| App identity | `NEX_APP_*` | no, except through `app.info()` metadata | Go config, dev/build scripts |
| Backend-only | anything except `VITE_` | no | Go: `a.Env().Get("KEY")` or `os.Getenv("KEY")` |
| Frontend-public | `VITE_` | yes | JS: `const pub = await app.info(); pub.public` |
| Env file override | `NEX_ENV_FILE` | no | Process environment before app startup |

Important rules:

| Rule | Reason |
|---|---|
| Keep app identity values explicit in `.env` | Dev, build, native metadata, and `app.info()` stay aligned without stale shell values winning |
| Never put secrets in `VITE_*` variables | `VITE_*` values are intentionally exposed to React |
| Prefer backend-only variables for tokens, paths, hosts, and credentials | They stay inside the Go process |
| Use `NEX_ENV_FILE` for release-specific config files | Keeps local/dev config separate from packaged builds |

At startup `main.go` loads `.env` before creating the app, then passes the same
file path into `nexpkg.Config.EnvFile`. Existing process environment variables
are not overwritten by `.env`.

---

## Security model

| Layer | Mechanism |
|---|---|
| Network | Server binds to `127.0.0.1` only |
| Token | 32-byte random token generated per launch |
| Injection | Token injected via `webview.Init` before frontend scripts |
| Header | Every API call requires `X-nex-Token` |
| Validation | Constant-time comparison with sliding expiry |
| CSRF | Origin header validated against app/dev origins |
| Port | Random in production; fixed `34115` in dev |

### Trusted frontend model

nex is a desktop framework. The React frontend bundled with the app is treated
as trusted application code, the same way Go handlers are trusted application
code. nex protects the local bridge from other origins and non-loopback traffic,
but it intentionally exposes powerful native APIs to the app.

Rules for apps built on nex:

| Rule | Reason |
|---|---|
| Use embedded/local frontend code for the main webview | The main webview receives the bridge token |
| Do not navigate the bridged webview to third-party sites | Remote code would run with access to privileged APIs |
| Validate user input before passing it to `shell`, `fs`, or `win.eval` | These APIs can affect the host system |
| Never log secrets returned from dialogs or secure-storage APIs | The framework cannot know what is sensitive for the app |
| Put app-specific restrictions in `SecurityPolicy` or hooks | Final app security belongs to the app built on nex |

### API privilege levels

The frontend showcase labels APIs with these levels:

| Level | Meaning | Examples |
|---|---|---|
| `safe` | Read-only or low-impact framework calls | `app.info`, `os.info`, `screen.info` |
| `user` | User-mediated calls that open OS UI or handlers | `dialog.open`, `notify.toast`, `shell.openURL` |
| `privileged` | Calls that can read/write files, run code, or change process state | `shell.exec`, `fs.remove`, `win.eval`, `app.exit`, `trash.empty` |

### Optional security policy

By default nex allows every `safe`/`user` API outright. `privileged` calls
(`app.restart/exit/quit`, `shell.exec/run/start`, all `fs.*`, `win.eval`,
`dialog.password`, native `clipboard.readText/writeText/clear`) additionally
require an explicit yes/no from the user through a native (zenity)
confirmation dialog shown by the Go backend itself — not by the frontend —
before the operation runs, unless the app has installed a `SecurityPolicy` or
the matching focused hook below, in which case that takes over the decision
entirely and the built-in dialog does not fire. This default confirmation
can't be bypassed by swapping or scripting the frontend, since it lives in
`internal/app`'s `Authorize()`, not in JS. Applications that need stricter (or
looser, or custom-UI) controls can install a policy without changing the
framework:

```go
type denyShell struct{}

func (denyShell) Allow(c *nexpkg.Context, d nexpkg.SecurityDecision) error {
    if d.Category == "shell" && d.Operation == "exec" {
        return nexpkg.Errorf("forbidden", "shell exec is disabled in this app")
    }
    return nil
}

a := nexpkg.New(nexpkg.Config{
    Title:          "my-app",
    SecurityPolicy: denyShell{},
})
```

For focused controls, `nexpkg.Config` also exposes optional hooks:

| Hook | Typical use |
|---|---|
| `OnSecurityDecision` | Audit or deny any privileged/user-mediated operation |
| `OnShellCommand` | Allowlist commands or force working directories |
| `OnFileAccess` | Restrict file APIs to app-owned folders |
| `OnWindowEval` | Disable or audit `win.eval` |
| `OnAppExit` | Confirm/deny `app.exit`, `app.restart`, `app.quit` |
| `OnOpenURL` | Restrict external URLs |

Returning `nil` allows the operation. Returning `nexpkg.Errorf(...)` or any
error denies it and sends an RPC error to the frontend.

---

## API Reference

Frontend code imports helpers from `frontend/src/lib/nex.js`:

```js
import {
  call,
  apiRiskLevel,
  on,
  session,
  framework,
  app,
  os,
  screen,
  env,
  dialog,
  notify,
  shell,
  win,
  fs,
  clipboard,
  log,
  menu,
  dragdrop,
  appearance,
  power,
  shortcuts,
  protocol,
  updater,
  secureStorage,
  fileAssociations,
  recentDocs,
} from "./lib/nex.js";
```

All backend RPC calls use `POST /api/rpc` with the session header
`X-nex-Token`. The `sys.*` namespace is reserved for built-in framework APIs;
application handlers should use custom names such as `user.greet`.

### Core

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `call(method, params?)` | any registered RPC | `method: string`, `params?: object` | RPC result or throws `nexError` |
| `apiRiskLevel(id)` | JS helper | API id | `safe`, `user`, or `privileged` |
| `session()` | `GET /api/session` | none | `{ id, expiresAt }` |
| `on(event, handler)` | backend event bridge | `event: string`, callback | unsubscribe function |

Backend events are emitted from Go:

```go
a.Emit("data-updated", map[string]any{"count": 42})
```

and received in JS:

```js
const off = on("data-updated", (payload) => console.log(payload));
```

### framework

Framework metadata is intentionally separate from application metadata. Use this
when the UI needs to show the nex runtime itself, for example `nex@0.1.0`.

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `framework.info()` | `sys.framework.info` | none | `{ name, version, build, updated, author, id, stack }` |
| `framework.stack()` | `sys.framework.stack` | none | Resolved framework stack versions, including Go, zenity, and webview_go |

### app

Application metadata comes from `.env` through `nexpkg.Config`, with
`main.go` and `version.go` acting as fallbacks for a fresh template. These
values belong to the app being built, not to the nex framework. `app.info()`
returns only app metadata, runtime, paths, and public env values. Framework
metadata is available only through `framework.info()`.

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `app.info()` | `sys.app.info` | none | `{ app, runtime, paths, public }` plus top-level app aliases |
| `app.paths()` | `sys.app.paths` | none | home/config/cache/temp/executable/cwd |
| `app.runtime()` | `sys.app.runtime` | none | dev/packaged mode, pid, executable, OS/arch |
| `app.restart()` | `sys.app.restart` | none | starts current executable again, then exits |
| `app.exit(code?)` | `sys.app.exit` | `{ code? }` | exits process after response |
| `app.quit()` | `sys.app.quit` | none | `{ ok: true }` and closes the app |

### os and env

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `os.info()` | `sys.os.info` | none | Full snapshot: host, user, runtime, process, paths, network, disks, memory, time, environment summary |
| `os.host()` | `sys.os.host` | none | OS, arch, compiler, Go version, hostname, kernel, platform, CPU, uptime |
| `os.user()` | `sys.os.user` | none | Current user id, group id, username, display name, home dir, group ids |
| `os.runtime()` | `sys.os.runtime` | none | Go runtime, build metadata, goroutines, GOMAXPROCS, cgo calls, allocator stats |
| `os.process()` | `sys.os.process` | none | PID, PPID, executable, cwd, args, page size, temp dir, environment summary |
| `os.network()` | `sys.os.network` | none | Network interfaces with index, MTU, flags, MAC address, assigned addresses |
| `os.disks()` | `sys.os.disks` | none | Mounted volumes/logical disks with size/free/used data when available |
| `os.memory()` | `sys.os.memory` | none | System memory details plus Go runtime memory stats |
| `os.time()` | `sys.os.time` | none | Local/UTC time, Unix timestamps, timezone and offset |
| `os.paths()` | `sys.env.paths` | none | `{ home, config, cache, temp, exe, cwd }` |
| `env.paths()` | `sys.env.paths` | none | same as `os.paths()` |

Environment details intentionally expose only counts and `VITE_*` public values;
non-public process variables can contain secrets and are not returned by these
system info APIs.

### dialog

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `dialog.open(opts?)` | `sys.dialog.open` | `{ title?, multiple?, directory?, startDir?, filters? }` | `{ canceled, paths }` |
| `dialog.save(opts?)` | `sys.dialog.save` | `{ title?, defaultName?, filters?, confirmOverwrite? }` | `{ canceled, path? }` |
| `dialog.message(opts)` | `sys.dialog.message` | `{ level?, title?, text }` | `{ confirmed }` |
| `dialog.entry(opts?)` | `sys.dialog.entry` | `{ title?, text?, defaultText?, hideText? }` | `{ canceled, value?, button? }` |
| `dialog.password(opts?)` | `sys.dialog.password` | `{ title?, username? }` | `{ canceled, username?, password? }` |
| `dialog.color(opts?)` | `sys.dialog.color` | `{ title?, showPalette? }` | `{ canceled, hex?, rgba?, r?, g?, b?, a? }` |
| `dialog.isSupported()` | `sys.dialog.isSupported` | none | `{ available, method, platform }` |

`filters` is an array of `{ name, patterns }`, for example:

```js
await dialog.open({
  title: "Open image",
  filters: [{ name: "Images", patterns: ["*.png", "*.jpg", "*.jpeg"] }],
});
```

`level` can be `info`, `warning`, `error`, or `question`.

`dialog.isSupported()` returns `{ available: true, method: "native" }` on macOS and Windows. On Linux it checks for the `zenity` binary or `libzenity.so.0`; it returns `{ available: false, method: "none" }` when neither is found (XDG portal is not probed since it requires D-Bus). When `available` is `false`, calls to `dialog.open` and `dialog.save` will reject with error code `dialog_unsupported`.

### notify

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `notify.toast(opts)` | `sys.notify.toast` | `{ title?, text, icon? }` | `{ ok, title, text, icon, platform }` |
| `notify.capabilities()` | `sys.notify.capabilities` | none | notification backend feature matrix |

`icon` accepts a stock icon name (`info`, `warning`, `error`, `question`,
`none`) or a file path / platform icon name supported by the OS notification
backend.

```js
await notify.toast({
  title: "nex",
  text: "Native toast from nex",
  icon: "info",
});
```

### shell

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `shell.openURL(url)` | `sys.shell.openURL` | `{ url }` | `{ ok: true }` |
| `shell.openPath(path)` | `sys.shell.openPath` | `{ path }` | `{ ok: true }` |
| `shell.showInFolder(path)` | `sys.shell.showInFolder` | `{ path }` | `{ ok: true }` |
| `shell.exec(command, opts?)` | `sys.shell.exec` | `{ command, cwd?, env?, timeoutMs? }` | process result |
| `shell.run(command, opts?)` | `sys.shell.run` | same as `shell.exec` | process result |
| `shell.start(command, opts?)` | `sys.shell.start` | `{ command, cwd?, env? }` | `{ ok, command, shell, pid }` |

`shell.exec` and `shell.run` execute through the native shell:

| OS | Shell |
|---|---|
| macOS/Linux | `/bin/sh -lc` |
| Windows | `%COMSPEC% /C`, falling back to `cmd.exe /C` |

Process result:

```js
{
  ok: true,
  command: "uname -a",
  shell: "sh",
  exitCode: 0,
  stdout: "...",
  stderr: "",
  timedOut: false,
  durationMs: 12
}
```

Default timeout is 30 seconds. Maximum timeout is capped at 10 minutes.

### window

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `win.info()` | `sys.window.info` | none | last known `{ title, width, height, hint }` |
| `win.setTitle(title)` | `sys.window.setTitle` | `{ title }` | `{ ok: true }` |
| `win.setSize(width, height, hint?)` | `sys.window.setSize` | `{ width, height, hint }` | `{ ok: true }` |
| `win.getSize()` | `sys.window.getSize` | none | last known `{ width, height }` |
| `win.fullscreen()` | `sys.window.fullscreen` | none | enters DOM fullscreen |
| `win.unfullscreen()` | `sys.window.unfullscreen` | none | exits DOM fullscreen |
| `win.reload()` | `sys.window.reload` | none | reloads the frontend document |
| `win.print()` | `sys.window.print` | none | opens the print dialog |
| `win.eval(js)` | `sys.window.eval` | `{ js }` | evaluates JavaScript asynchronously |

`hint` can be `none`, `min`, `max`, or `fixed`.

Window operations in the current contract are the ones listed above. Lower-level
window-handle controls such as `maximize`, `minimize`, `show`, `hide`, `center`,
`position`, and `alwaysOnTop` are tracked separately from the callable API.

### screen, log, menu, dragdrop

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `screen.info()` | `sys.screen.info` + DOM | none | browser screen metrics plus host display data |
| `screen.native()` | `sys.screen.info` | none | native monitor list (`x,y,width,height,primary`), read directly via CoreGraphics/GDK/user32 — no subprocess, no platform-tool dependency |
| `log.print/info/warning/error(message)` | `sys.log.*` | `{ message }` | writes through Go logger |
| `menu.set(definition)` | `sys.menu.set` | JSON menu definition | stores, emits, and renders the webview runtime menu |
| `menu.update()` | `sys.menu.update` | none | replays the stored menu definition |
| `dragdrop.onFileDrop(cb, target?)` | DOM | callback | listens for file/folder drops in the webview |

`dragdrop.onFileDrop` returns metadata for dropped files. Absolute paths are
included only when the underlying webview/browser exposes them; otherwise the
browser-safe file metadata is returned.

### fs

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `fs.read(path, encoding?)` | `sys.fs.read` | `{ path, encoding }` | `{ data, encoding }` |
| `fs.write(path, data, encoding?)` | `sys.fs.write` | `{ path, data, encoding }` | `{ ok, bytes }` |
| `fs.list(path)` | `sys.fs.list` | `{ path }` | `{ entries }` |
| `fs.exists(path)` | `sys.fs.exists` | `{ path }` | `{ exists, isDir? }` |
| `fs.stat(path)` | `sys.fs.stat` | `{ path }` | `{ name, size, isDir, modTime }` |
| `fs.mkdir(path)` | `sys.fs.mkdir` | `{ path }` | `{ ok: true }` |
| `fs.remove(path, recursive?)` | `sys.fs.remove` | `{ path, recursive }` | `{ ok: true }` |
| `fs.rename(from, to)` | `sys.fs.rename` | `{ from, to }` | `{ ok: true }` |

`encoding` can be `utf8` or `base64`. `fs.list` entries are:

```js
{ name, isDir, size }
```

### clipboard

Clipboard uses the browser/webview DOM API and does not call the Go backend.

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `clipboard.writeText(text)` | DOM `navigator.clipboard` | `text: string` | promise |
| `clipboard.readText()` | DOM `navigator.clipboard` | none | promise resolving to text |
| `clipboard.pasteHandler(onChange)` | JS helper | `onChange: (text) => void` | `{ onKeyDown, onPaste }` spread-ready handler pair |
| `clipboard.native.readText()` | `sys.clipboard.readText` | none | reads native clipboard text |
| `clipboard.native.writeText(text)` | `sys.clipboard.writeText` | `{ text }` | writes text via the native OS clipboard |
| `clipboard.native.clear()` | `sys.clipboard.clear` | none | clears native clipboard text |
| `clipboard.native.formats()` | `sys.clipboard.formats` | none | supported native clipboard formats |
| `clipboard.native.isAvailable()` | `sys.clipboard.isAvailable` | none | `{ available, tool, platform }` |

Native text clipboard access is implemented natively per platform, with no
shell subprocess spawned: `NSPasteboard` via cgo on macOS (`internal/system/clipboard_darwin.go`),
the `user32`/`kernel32` Win32 clipboard API via `syscall` (no cgo) on Windows
(`internal/system/clipboard_windows.go`), and GTK's `GtkClipboard` via cgo on
Linux (`internal/system/clipboard_linux.go`), reusing the `gtk+-3.0` linkage
already required by the webview's GTK/WebKitGTK backend. Because of this,
`available` is always `true` on all three supported platforms — there is no
"install xclip/wl-clipboard" fallback path anymore. `tool` reports which native
API served the call (`NSPasteboard`, `user32`, or `gtk_clipboard`). The public
native clipboard API is text-only; richer clipboard formats are intentionally
left out of the callable API surface until a full native backend exists.

`clipboard.pasteHandler(onChange)` returns a `{ onKeyDown, onPaste }` object ready to spread onto any `<input>` or `<textarea>`. It is the idiomatic nex solution for password and secret fields inside native webviews, where `navigator.clipboard.readText()` may be blocked or unreliable:

```js
import { clipboard } from "./lib/nex.js";

function SecretInput({ value, onChange }) {
  return <input type="password" value={value} onChange={e => onChange(e.target.value)}
    {...clipboard.pasteHandler(onChange)} />;
}
```

`onPaste` handles right-click / Edit-menu paste and is always reliable. `onKeyDown` intercepts Cmd/Ctrl+V and calls `clipboard.native.readText()` (Go backend, bypasses the webview permission model), falling back to DOM clipboard if the backend call fails.

`clipboard.native.isAvailable()` reports whether the native path is usable — it is always `true` on macOS, Windows, and Linux, since clipboard access no longer depends on an optional external tool being installed. `clipboard.pasteHandler(onChange)` remains safe to spread unconditionally either way: `onPaste` (right-click/Edit-menu paste) always works regardless of the native path.

### desktop integration

These APIs expose desktop/runtime integrations that nex provides without adding
external framework dependencies. OS-level status calls inspect host registries,
bundle metadata, or common desktop tools when those facilities exist.

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `appearance.info()` | `sys.appearance.info` | none | OS theme/dark-mode data exposed by the host |
| `power.info()` | `sys.power.info` | none | battery and power data exposed by the host |
| `shortcuts.register(accelerator, handler?, opts?)` | `sys.shortcuts.register` + DOM | accelerator/options | registers a webview-window shortcut |
| `shortcuts.unregister(idOrAccelerator)` | `sys.shortcuts.unregister` + DOM | id or accelerator | unregisters a runtime shortcut |
| `shortcuts.clear()` | `sys.shortcuts.clear` + DOM | none | clears runtime shortcuts |
| `shortcuts.list()` | `sys.shortcuts.list` | none | registered runtime shortcuts |
| `protocol.status(scheme)` | `sys.protocol.status` | `{ scheme }` | OS protocol/deep-link registration evidence |
| `updater.check(opts?)` | `sys.updater.check` | `{ url?, current? }` | checks a JSON update provider; `url` must use `https` and cannot point to loopback or private hosts |
| `secureStorage.isAvailable()` | `sys.secureStorage.isAvailable` | none | secure storage backend availability |
| `fileAssociations.status(ext)` | `sys.fileAssociations.status` | `{ extension }` | OS file association evidence |
| `recentDocs.add(path)` | `sys.recentDocs.add` | `{ path }` | persists a recent document path |
| `recentDocs.list()` | `sys.recentDocs.list` | none | persisted recent documents |
| `recentDocs.clear()` | `sys.recentDocs.clear` | none | clears persisted recent documents |

### trash

| Helper | Backend | Params | Returns |
|---|---|---|---|
| `trash.empty()` | `sys.trash.empty` | none | `{ ok, platform, removed? }` |

Permanently empties the native OS trash/recycle bin — **no undo**. Native per
platform, no subprocess: direct filesystem manipulation of `~/.Trash` (macOS)
or the XDG `~/.local/share/Trash` directories (Linux), both pure Go; native
`SHEmptyRecycleBinW` (Windows, `syscall`, no cgo). `removed` (item count) is
only reported on macOS/Linux — the Windows API doesn't return one. This is a
`privileged` call: unless the app installs its own `SecurityPolicy`/
`OnFileAccess` hook, the backend shows its own native confirmation dialog
before running it (see [Optional security policy](#optional-security-policy)).

### Custom RPC

Register handlers in Go:

```go
a.Handle("user.greet", func(c *nexpkg.Context, params json.RawMessage) (any, error) {
    var p struct {
        Name string `json:"name"`
    }
    if err := c.Bind(params, &p); err != nil {
        return nil, nexpkg.Errorf("bad_request", "%v", err)
    }
    return map[string]string{"message": "ciao " + p.Name}, nil
})
```

Call them from JS:

```js
const result = await call("user.greet", { name: "ada" });
```

Inside handlers, `Context` provides:

| Go method/field | Purpose |
|---|---|
| `c.Bind(params, &value)` | Decode JSON params |
| `c.Emit(event, payload)` | Send event to frontend |
| `c.OnMain(fn)` | Run function on UI thread |
| `c.SetTitle(title)` | Set window title |
| `c.SetSize(w, h, hint)` | Resize window |
| `c.Quit()` | Close app |
| `c.Session` | Current session metadata |
| `c.Request` | Raw HTTP request |
| `c.Ctx` | Request context |

Return typed errors with:

```go
return nil, nexpkg.Errorf("bad_request", "message")
```

---

## Native API Call Table

These are the native APIs callable from the frontend through the nex
bridge.

| Area | JS helper | RPC method | Purpose |
|---|---|---|---|
| Core | `call(method, params?)` | any registered method | Call a backend RPC handler |
| Core | `session()` | `/api/session` | Read current session metadata |
| Core | `on(event, handler)` | event bridge | Subscribe to backend events |
| Framework | `framework.info()` | `sys.framework.info` | Read nex framework metadata |
| Framework | `framework.stack()` | `sys.framework.stack` | Read resolved framework dependency versions |
| App | `app.info()` | `sys.app.info` | Read app metadata and public env |
| App | `app.paths()` | `sys.app.paths` | Read home, config, cache, temp, executable and cwd paths |
| App | `app.runtime()` | `sys.app.runtime` | Read runtime mode, pid, executable, OS and architecture |
| App | `app.restart()` | `sys.app.restart` | Start a new process for the current executable and exit this one |
| App | `app.exit(code?)` | `sys.app.exit` | Exit the current process with an explicit code |
| App | `app.quit()` | `sys.app.quit` | Close the application |
| OS | `os.info()` | `sys.os.info` | Read full system snapshot |
| OS | `os.host()` | `sys.os.host` | Read host, OS, kernel, platform, CPU, uptime |
| OS | `os.user()` | `sys.os.user` | Read current OS user information |
| OS | `os.runtime()` | `sys.os.runtime` | Read Go runtime and build information |
| OS | `os.process()` | `sys.os.process` | Read current process information |
| OS | `os.network()` | `sys.os.network` | Read network interface information |
| OS | `os.disks()` | `sys.os.disks` | Read mounted volume/logical disk information |
| OS | `os.memory()` | `sys.os.memory` | Read system and Go runtime memory information |
| OS | `os.time()` | `sys.os.time` | Read time and timezone information |
| Screen | `screen.info()` | `sys.screen.info` + DOM | Read display/screen information |
| Env | `env.paths()` | `sys.env.paths` | Read standard paths |
| Dialog | `dialog.open(opts?)` | `sys.dialog.open` | Open native file/folder picker |
| Dialog | `dialog.save(opts?)` | `sys.dialog.save` | Open native save dialog |
| Dialog | `dialog.message(opts)` | `sys.dialog.message` | Show native message dialog |
| Dialog | `dialog.entry(opts?)` | `sys.dialog.entry` | Show native text entry dialog |
| Dialog | `dialog.password(opts?)` | `sys.dialog.password` | Show native password/username dialog |
| Dialog | `dialog.color(opts?)` | `sys.dialog.color` | Show native color picker |
| Dialog | `dialog.isSupported()` | `sys.dialog.isSupported` | Check if native file dialogs are available on this platform |
| Notify | `notify.toast(opts)` | `sys.notify.toast` | Show native OS toast notification |
| Notify | `notify.capabilities()` | `sys.notify.capabilities` | Read notification backend capabilities |
| Shell | `shell.openURL(url)` | `sys.shell.openURL` | Open URL with system browser |
| Shell | `shell.openPath(path)` | `sys.shell.openPath` | Open path with default app |
| Shell | `shell.showInFolder(path)` | `sys.shell.showInFolder` | Reveal path in file manager |
| Shell | `shell.exec(command, opts?)` | `sys.shell.exec` | Execute native shell command and capture output |
| Shell | `shell.run(command, opts?)` | `sys.shell.run` | Alias for `shell.exec` |
| Shell | `shell.start(command, opts?)` | `sys.shell.start` | Start native shell command and return PID |
| Window | `win.info()` | `sys.window.info` | Read tracked window state |
| Window | `win.setTitle(title)` | `sys.window.setTitle` | Set native window title |
| Window | `win.setSize(width, height, hint?)` | `sys.window.setSize` | Resize native window |
| Window | `win.getSize()` | `sys.window.getSize` | Read last tracked window size |
| Window | `win.fullscreen()` | `sys.window.fullscreen` | Enter DOM fullscreen |
| Window | `win.unfullscreen()` | `sys.window.unfullscreen` | Exit DOM fullscreen |
| Window | `win.reload()` | `sys.window.reload` | Reload frontend document |
| Window | `win.print()` | `sys.window.print` | Open print dialog |
| Window | `win.eval(js)` | `sys.window.eval` | Evaluate JavaScript in the webview |
| Log | `log.print(message)` | `sys.log.print` | Write a raw message to the Go logger |
| Log | `log.trace(message)` | `sys.log.trace` | Write a trace message to the Go logger |
| Log | `log.debug(message)` | `sys.log.debug` | Write a debug message to the Go logger |
| Log | `log.info(message)` | `sys.log.info` | Write an info message to the Go logger |
| Log | `log.warning(message)` | `sys.log.warning` | Write a warning message to the Go logger |
| Log | `log.error(message)` | `sys.log.error` | Write an error message to the Go logger |
| Menu | `menu.set(definition)` | `sys.menu.set` | Set and render the runtime menu |
| Menu | `menu.update()` | `sys.menu.update` | Replay the current runtime menu |
| Dragdrop | `dragdrop.onFileDrop(cb)` | DOM | Listen for dropped files/folders |
| Filesystem | `fs.read(path, encoding?)` | `sys.fs.read` | Read file as `utf8` or `base64` |
| Filesystem | `fs.write(path, data, encoding?)` | `sys.fs.write` | Write file as `utf8` or `base64` |
| Filesystem | `fs.list(path)` | `sys.fs.list` | List directory entries |
| Filesystem | `fs.exists(path)` | `sys.fs.exists` | Check path existence |
| Filesystem | `fs.stat(path)` | `sys.fs.stat` | Read path metadata |
| Filesystem | `fs.mkdir(path)` | `sys.fs.mkdir` | Create directory recursively |
| Filesystem | `fs.remove(path, recursive?)` | `sys.fs.remove` | Remove file or directory |
| Filesystem | `fs.rename(from, to)` | `sys.fs.rename` | Rename or move path |
| Clipboard | `clipboard.writeText(text)` | DOM clipboard | Write text to clipboard |
| Clipboard | `clipboard.readText()` | DOM clipboard | Read text from clipboard |
| Clipboard | `clipboard.pasteHandler(onChange)` | JS helper | `{ onKeyDown, onPaste }` handler pair for password/secret inputs using native backend on Cmd/Ctrl+V |
| Clipboard | `clipboard.native.readText()` | `sys.clipboard.readText` | Read text via the native OS clipboard (NSPasteboard/user32/GtkClipboard) |
| Clipboard | `clipboard.native.writeText(text)` | `sys.clipboard.writeText` | Write text via the native OS clipboard (NSPasteboard/user32/GtkClipboard) |
| Clipboard | `clipboard.native.clear()` | `sys.clipboard.clear` | Clear native text clipboard |
| Clipboard | `clipboard.native.formats()` | `sys.clipboard.formats` | Read native clipboard format support |
| Clipboard | `clipboard.native.isAvailable()` | `sys.clipboard.isAvailable` | Always `true` on macOS/Windows/Linux (no external tool dependency) |
| Appearance | `appearance.info()` | `sys.appearance.info` | Read system theme/dark-mode data |
| Power | `power.info()` | `sys.power.info` | Read battery/power information |
| Shortcuts | `shortcuts.register(accelerator, handler?, opts?)` | `sys.shortcuts.register` | Register a webview-window keyboard shortcut |
| Shortcuts | `shortcuts.unregister(idOrAccelerator)` | `sys.shortcuts.unregister` | Unregister a runtime shortcut |
| Shortcuts | `shortcuts.clear()` | `sys.shortcuts.clear` | Clear runtime shortcuts |
| Shortcuts | `shortcuts.list()` | `sys.shortcuts.list` | List runtime shortcuts |
| Protocol | `protocol.status(scheme)` | `sys.protocol.status` | Inspect protocol/deep-link registration |
| Updater | `updater.check(opts?)` | `sys.updater.check` | Check a JSON update provider (`url` must be `https`, no loopback/private hosts) |
| Secure storage | `secureStorage.isAvailable()` | `sys.secureStorage.isAvailable` | Detect secure storage backend availability |
| File associations | `fileAssociations.status(ext)` | `sys.fileAssociations.status` | Inspect file association registration |
| Recent docs | `recentDocs.add(path)` | `sys.recentDocs.add` | Persist a recent document path |
| Recent docs | `recentDocs.list()` | `sys.recentDocs.list` | List persisted recent documents |
| Recent docs | `recentDocs.clear()` | `sys.recentDocs.clear` | Clear persisted recent documents |
| Trash | `trash.empty()` | `sys.trash.empty` | Permanently empty the native trash/recycle bin (no undo) |

Outside the current callable API scope: lower-level native window handles for
minimize/maximize/position/always-on-top, system tray rendering, installer-time
protocol/file association declaration, secure storage read/write helpers,
provider-specific updater adapters, global shortcuts, and cross-platform sleep
prevention.

---

## License

nex is distributed under the **nex Framework Source-Available License** —
Copyright © 2026 Veronesi Lorenzo (vlT).

The framework may be read, studied, forked, modified, and redistributed as a
standalone framework under the terms in [LICENSE](LICENSE). Incorporating,
embedding, vendoring, bundling, or otherwise using nex inside another project,
product, service, template, SDK, framework, or distributed software requires
prior express written consent from Veronesi Lorenzo (vlT).

For licensing inquiries or written permission requests:
[veronesilorenzo@outlook.com](mailto:veronesilorenzo@outlook.com)
