<p align="center">
  <img src="res/nex.svg" alt="nex" width="150" />
</p>

<p align="center">
  Cross-platform desktop framework: React/Vite frontend + Go backend<br/>
  <sub>Native OS webview · Native APIs · Embedded frontend assets · Garble obfuscation</sub>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.1.0--R110726-blue?style=flat-square" alt="version"/>
  <img src="https://img.shields.io/badge/go-1.26.4-00ADD8?style=flat-square&logo=go" alt="go"/>
  <img src="https://img.shields.io/badge/react-19.2.7-61DAFB?style=flat-square&logo=react&logoColor=white" alt="react"/>
  <img src="https://img.shields.io/badge/react--icons-5.7.0-61DAFB?style=flat-square&logo=react&logoColor=white" alt="react-icons"/>
  <img src="https://img.shields.io/badge/vite-8.1.3-646CFF?style=flat-square&logo=vite&logoColor=white" alt="vite"/>
  <img src="https://img.shields.io/badge/license-proprietary-critical?style=flat-square" alt="license"/>
</p>

---

## What is nex?

nex is a desktop framework for building native cross-platform apps with a
React/Vite interface and a Go backend. The frontend is embedded into the Go
binary, served through a local loopback server, and connected to Go through the
nex RPC bridge.

The README is intentionally short. The complete framework manual is in
[DOCS.md](DOCS.md).

---

## What it provides

| Area | Included |
|---|---|
| Frontend | React 19 + Vite 8 template |
| Icons | `react-icons` icon components for the showcase UI |
| Stack badges | Version badges for nex, Go, React, React Icons, Vite, Zenity, and webview |
| Backend | Go RPC runtime, embedded assets, event bridge |
| Webview | Native WebView2, WebKit, WebKitGTK backends |
| Native APIs | Dialogs, notifications, shell, filesystem, clipboard, OS/system info, menu, desktop/runtime integrations |
| Docs UI | Built-in rendered docs modal for `DOCS.md` |
| Window APIs | Title, size, fullscreen, reload, print, eval |
| Security | Loopback-only server, per-launch token, Origin check, optional policy hooks |
| Dev | Dedicated `dev.go` runner with fixed nex dev ports |
| Build | Dedicated `build.go` runner with garble obfuscation |

Native dialogs and notifications use
[`github.com/ncruces/zenity`](https://github.com/ncruces/zenity) as part of the
framework.

The template stack badges are resolved from project metadata: frontend package
versions come from `frontend/package.json`, while Go/native dependency versions
come from `framework.stack()`.

---

## Requirements

| Tool | Version / note |
|---|---|
| Go | 1.26.4+ |
| Node | 20.19+ or 22.12+ with npm |
| C compiler | Required by cgo/webview bindings |
| garble | Installed automatically by `build.go` when missing |

Vite 8 requires Node `^20.19.0` or `>=22.12.0`; older Node 18 runtimes are no
longer supported by the frontend toolchain.

Platform notes:

| OS | Extra requirements |
|---|---|
| macOS | `xcode-select --install` |
| Linux | `build-essential`, `pkg-config`, `libgtk-3-dev`, `libwebkit2gtk-4.0-dev`, `zenity` |
| Windows | Microsoft Edge WebView2 Runtime |
| Windows cross-build | `mingw-w64` (`x86_64-w64-mingw32-gcc/g++`) |

Full host/target build coverage is documented in [DOCS.md](DOCS.md#build-host-coverage).

---

## Development

Start the development runner:

```bash
go run dev.go
```

or:

```bash
make dev
```

Dev mode starts Vite HMR and the Go backend with:

```text
APP_DEV=1
CGO_ENABLED=1
```

Default dev ports:

| Service | Address |
|---|---|
| Vite | `http://127.0.0.1:5179` |
| Go API | `http://127.0.0.1:34115` |

Port `5173` is intentionally left free for standalone frontend projects.

On macOS, dev mode runs through a temporary `.app` bundle that's ad-hoc
code-signed (`codesign --sign -`) right after each build, before being
launched — see [DOCS.md](DOCS.md#macos) for why.

---

## Build

Production artifacts are handled by `build.go`:

```bash
go run build.go build
```

or:

```bash
make build
```

Useful commands:

| Command | Purpose |
|---|---|
| `make dev` | Run Vite HMR + Go desktop app |
| `make build` | Build host-supported release targets (garble by default) |
| `make release` | Optimized release build (`-s -w`, garble by default) |
| `make obfuscate` | Alias for obfuscated release build |
| `make requirements` | Check host toolchains |
| `make doctor` | Alias for requirements |
| `make clean` | Remove generated release/dist assets |

Release folders use:

```text
release/<NEX_APP_VERSION>-<NEX_APP_BUILD>/
```

Example:

```text
release/0.1.0-R110726/
```

Each target gets its own `<GOOS>-<GOARCH>/` subfolder holding a plain-named
artifact (`<Title>.app`, `<Title>.exe`) — no composite names. Windows builds
are always GUI-subsystem (no console window) with the icon, manifest, and
version info embedded directly in the `.exe` — a single file, no side-cars.

The build host matrix and toolchain overrides are in [DOCS.md](DOCS.md#build-system).

---

## App metadata

Application metadata belongs to the app and is configured from `.env`.
`main.go` and `version.go` only provide fallback values for a fresh template.

```env
NEX_APP_NAME=nex-app-template
NEX_APP_VERSION=0.1.0
NEX_APP_BUILD=R110726
NEX_APP_UPDATED=11 Luglio 2026
NEX_APP_AUTHOR=© 2026 vlT di Veronesi Lorenzo
NEX_APP_ICON=res/nexicon.svg
NEX_APP_ID=dev.vlt.nex-app-template
```

```go
nexpkg.New(nexpkg.Config{
    Title:            envValue(envValues, "NEX_APP_NAME", "nex-app-template"),
    AppVersion:       envValue(envValues, "NEX_APP_VERSION", AppVersion),
    AppBuild:         envValue(envValues, "NEX_APP_BUILD", AppBuild),
    AppUpdated:       envValue(envValues, "NEX_APP_UPDATED", AppUpdated),
    AppAuthor:        envValue(envValues, "NEX_APP_AUTHOR", AppAuthor),
    Icon:             envValue(envValues, "NEX_APP_ICON", "res/nexicon.svg"),
    SingleInstanceID: envValue(envValues, "NEX_APP_ID", "dev.vlt.nex-app-template"),
})
```

`dev.go` and `build.go` load the same `.env`, so the dev app title/icon,
release folder, macOS bundle metadata, Windows manifest metadata, and
`sys.app.info()` stay aligned. For `NEX_APP_*` identity values, `.env` is the
authoritative source; process environment values are used only when the key is
missing from the file.

Framework metadata is separate and available from:

```js
const fw = await framework.info(); // { id: "nex@0.1.0", ... }
```

`app.info()` and `framework.info()` are separate API calls: app metadata comes
from `.env` with Go fallbacks, while framework metadata comes from
`internal/meta`.

From Go, framework identity is exposed by the public facade:

```go
nexpkg.Name
nexpkg.FrameworkVersion()
nexpkg.FrameworkBuild()
nexpkg.FrameworkUpdated()
nexpkg.FrameworkAuthor()
```

---

## Native API overview

The frontend bridge is in `frontend/src/lib/nex.js`.

| Namespace | Examples |
|---|---|
| `framework` | `framework.info()`, `framework.stack()` |
| `app` | `app.info()`, `app.paths()`, `app.runtime()`, `app.quit()` |
| `os` / `env` | `os.info()`, `os.host()`, `env.paths()` |
| `dialog` | `dialog.open()`, `dialog.save()`, `dialog.message()`, `dialog.isSupported()` |
| `notify` | `notify.toast()` |
| `shell` | `shell.openURL()`, `shell.exec()`, `shell.start()` |
| `win` | `win.setTitle()`, `win.setSize()`, `win.fullscreen()` |
| `fs` | `fs.read()`, `fs.write()`, `fs.list()`, `fs.remove()` |
| `clipboard` | `clipboard.readText()`, `clipboard.native.writeText()`, `clipboard.native.isAvailable()`, `clipboard.pasteHandler()` |
| `desktop` | `appearance.info()`, `power.info()`, `shortcuts.register()`, `protocol.status()`, `recentDocs.list()` |
| `trash` | `trash.empty()` |

The full API table is in [DOCS.md](DOCS.md#native-api-call-table).

---

## Template docs modal

The frontend template includes a `docs` button in the top bar. It opens a
rendered markdown modal with the bundled `DOCS.md` content, so the full manual
remains available in dev and in packaged apps without depending on the process
working directory.

---

## Brand component

The nex wordmark used by the template is a reusable React component:
`frontend/src/components/NexLogo.jsx`. It imports `res/CenturyGothic.ttf`
directly at component level and uses `res/nexcube.svg` for the cube mark. The
default animation fades the cube and wordmark in from the center and slides them
into their final positions.

---

## Project structure

```text
nex-framework/
├── config/              # .env loader
├── frontend/            # React/Vite UI, nex JS bridge, reusable components
├── internal/            # framework runtime internals
├── nex/                 # public Go facade
├── res/                 # brand assets, app icon sources, CenturyGothic.ttf
├── .env                 # app identity and runtime environment
├── build.go             # production/release runner
├── dev.go               # development runner
├── main.go              # app entry point
├── version.go           # fallback app version metadata
├── README.md            # quick entry point
└── DOCS.md              # full framework documentation
```

Detailed architecture and flow diagrams are in [DOCS.md](DOCS.md#project-structure).

---

## Documentation

| File | Purpose |
|---|---|
| [README.md](README.md) | Quick project entry point |
| [DOCS.md](DOCS.md) | Complete framework architecture, build, security, and API reference |
| [LICENSE](LICENSE) | Source-available license terms |

---

## License

nex is distributed under the **nex Framework Source-Available License** —
Copyright © 2026 Veronesi Lorenzo (vlT).

See [LICENSE](LICENSE) for the complete terms.
