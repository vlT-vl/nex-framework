import { useEffect, useMemo, useRef, useState } from "react";
import {
  FiBookOpen,
  FiCpu,
  FiExternalLink,
  FiFileText,
  FiInfo,
  FiLayers,
  FiMoon,
  FiMonitor,
  FiPlay,
  FiShield,
  FiSun,
  FiTerminal,
  FiTrash2,
  FiX,
  FiZap,
} from "react-icons/fi";
import * as nex from "./lib/nex.js";
import {
  applyTheme,
  getThemePreference,
  resolveTheme,
  setThemePreference,
  watchSystemTheme,
} from "./lib/theme.js";
import { renderMarkdown } from "./lib/markdown.js";
import { NexLogo } from "./components/NexLogo.jsx";
import frontendPkg from "../package.json";
import docsMarkdown from "../../DOCS.md?raw";
import licenseText from "../../LICENSE?raw";
import nexMastheadSvg from "../../res/nex.svg?raw";

function pkgVersion(name) {
  const version = frontendPkg.dependencies?.[name] ?? frontendPkg.devDependencies?.[name] ?? "…";
  return String(version).replace(/^[~^]/, "");
}

const stackBadges = [
  { key: "nex", label: "nex", value: "…", tone: "blue" },
  { key: "go", label: "go", value: "1.22", tone: "cyan" },
  { label: "react", value: pkgVersion("react"), tone: "sky" },
  { label: "react-icons", value: pkgVersion("react-icons"), tone: "sky" },
  { label: "vite", value: pkgVersion("vite"), tone: "violet" },
  { key: "zenity", label: "zenity", value: "…", tone: "green" },
  { key: "webview_go", label: "webview", value: "…", tone: "slate" },
];

const docsFile = {
  title: "DOCS.md",
  description: "Complete framework manual.",
  content: docsMarkdown,
};

const licenseFile = {
  title: "LICENSE",
  description: "nex Framework Source-Available License.",
  content: licenseText,
};

// ── sub-components ────────────────────────────────────────────────────────────

function ThemeSwitch({ preference, theme, onChange }) {
  const modes = [
    { id: "system", label: "system", icon: FiMonitor },
    { id: "light", label: "light", icon: FiSun },
    { id: "dark", label: "dark", icon: FiMoon },
  ];
  return (
    <div className="theme-switch" role="group" aria-label="color mode">
      {modes.map((mode) => {
        const Icon = mode.icon;
        return (
          <button
            className={`theme-option${preference === mode.id ? " active" : ""}`}
            key={mode.id}
            onClick={() => onChange(mode.id)}
            title={mode.id === "system" ? `system (${theme})` : mode.label}
            type="button"
          >
            <Icon aria-hidden="true" className="ui-icon" />
            <span>{mode.label}</span>
          </button>
        );
      })}
    </div>
  );
}

function StatusBar({ info, sess, beat, theme, themePreference, onThemeChange, onDocsOpen, onLicenseOpen }) {
  return (
    <header className="topbar">
      <div className="brand">
        <span className="brand-name">{info?.name ?? "nex"}</span>
        <span className="brand-ver">v{info?.version ?? "—"}</span>
      </div>
      <div className="top-actions">
        <button className="top-button" onClick={onDocsOpen} type="button">
          <FiBookOpen aria-hidden="true" className="ui-icon" />
          <span>docs</span>
        </button>
        <button className="top-button" onClick={onLicenseOpen} type="button">
          <FiShield aria-hidden="true" className="ui-icon" />
          <span>license</span>
        </button>
        <ThemeSwitch preference={themePreference} theme={theme} onChange={onThemeChange} />
        <div className="bridge">
          <span>{sess ? `session ${sess.id.slice(0, 8)}` : "connecting…"}</span>
          {/* key={beat} restarts CSS animation on every tick */}
          <span
            key={beat}
            className={`dot${sess ? " online" : ""}${beat > 0 ? " beat" : ""}`}
            title="backend ⇄ frontend bridge"
          />
        </div>
      </div>
    </header>
  );
}

function FileModal({ open, onClose, onLog, file, kicker, format = "markdown" }) {
  const renderedContent = useMemo(() => {
    if (format !== "markdown") return null;
    const html = renderMarkdown(file.content);
    // The masthead <img src="res/nex.svg"> only tracks the OS-level
    // prefers-color-scheme; it goes invisible when the in-app theme switch
    // (data-theme) diverges from the OS setting (e.g. dark theme picked
    // while the OS is in light mode). Inlining the SVG lets .docs-content's
    // CSS override .wordmark's fill from data-theme instead.
    return html.replace(
      /<img[^>]*\bsrc="\/nex\.svg"[^>]*>/,
      `<span class="masthead-logo">${nexMastheadSvg}</span>`
    );
  }, [file.content, format]);
  const contentRef = useRef(null);
  const titleId = `${file.title}-modal-title`;

  useEffect(() => {
    if (!open) return undefined;

    function onKeyDown(event) {
      if (event.key === "Escape") onClose();
    }

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open, onClose]);

  function openAsFile() {
    const mime = format === "markdown" ? "text/markdown;charset=utf-8" : "text/plain;charset=utf-8";
    const blob = new Blob([file.content], { type: mime });
    const url = URL.createObjectURL(blob);
    const opened = window.open(url, "_blank");
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    if (opened) {
      onLog?.(`opened ${file.title}`, "ok");
    } else {
      onLog?.(`popup blocked for ${file.title}`, "err");
    }
  }

  function onContentClick(event) {
    if (format !== "markdown") return;
    const link = event.target.closest?.("a[href]");
    if (!link || !contentRef.current?.contains(link)) return;

    const href = link.getAttribute("href") || "";
    if (href.startsWith("#")) {
      event.preventDefault();
      const id = href.slice(1);
      const target = Array.from(contentRef.current.querySelectorAll("[id]"))
        .find((node) => node.id === id);
      target?.scrollIntoView({ block: "start" });
      return;
    }

    if (/^(https?:|mailto:)/.test(href)) {
      event.preventDefault();
      window.open(href, "_blank");
    }
  }

  if (!open) return null;

  return (
    <div
      aria-labelledby={titleId}
      aria-modal="true"
      className="modal-layer"
      role="dialog"
    >
      <button
        aria-label={`close ${file.title}`}
        className="modal-backdrop"
        onClick={onClose}
        type="button"
      />
      <div className="docs-modal">
        <div className="docs-head">
          <div>
            <span className="docs-kicker">{kicker}</span>
            <h2 id={titleId}>{file.title}</h2>
            <p>{file.description}</p>
          </div>
          <button className="modal-close" onClick={onClose} type="button" aria-label="close">
            <FiX aria-hidden="true" />
          </button>
        </div>

        <div className="docs-actions">
          <span className="docs-file">
            <FiFileText aria-hidden="true" className="ui-icon" />
            {file.title}
          </span>
          <button className="docs-open" onClick={openAsFile} type="button">
            <FiExternalLink aria-hidden="true" className="ui-icon" />
            <span>open file</span>
          </button>
        </div>

        {format === "markdown" ? (
          <div
            className="docs-content"
            onClick={onContentClick}
            ref={contentRef}
            dangerouslySetInnerHTML={{ __html: renderedContent }}
          />
        ) : (
          <div className="docs-content" ref={contentRef}>
            <pre>
              <code>{file.content}</code>
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}

function ApiExplorer({ demos, onRun }) {
  return (
    <div className="panel">
      {nex.apiCatalog.map((group) => (
        <div className="group" key={group.namespace}>
          <div className="group-head">{group.namespace}</div>
          {group.methods.map((m) => {
            const level = nex.apiRiskLevel(m.id);
            return (
              <div className="api-row" key={m.id}>
                <span className="api-sig">{m.sig}</span>
                <span className={`api-risk ${level}`}>{level}</span>
                <span className="api-desc">{m.desc}</span>
                {demos[m.id] ? (
                  <button className="btn" onClick={() => onRun(m.id, demos[m.id])}>
                    <FiPlay aria-hidden="true" className="ui-icon" />
                    <span>Run</span>
                  </button>
                ) : (
                  <button className="btn" disabled>—</button>
                )}
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}

const CONSOLE_HEIGHT_KEY = "nex-console-height";
const CONSOLE_MIN_HEIGHT = 118;
const CONSOLE_DEFAULT_HEIGHT = 156;

function clampConsoleHeight(value) {
  const viewportHeight = typeof window === "undefined" ? 760 : window.innerHeight;
  const max = Math.max(CONSOLE_MIN_HEIGHT, Math.floor(viewportHeight * 0.72));
  return Math.min(max, Math.max(CONSOLE_MIN_HEIGHT, value));
}

function initialConsoleHeight() {
  try {
    const saved = Number(localStorage.getItem(CONSOLE_HEIGHT_KEY));
    if (Number.isFinite(saved) && saved > 0) return clampConsoleHeight(saved);
  } catch {
    // Keep default height when storage is unavailable.
  }
  return clampConsoleHeight(CONSOLE_DEFAULT_HEIGHT);
}

function Console({ lines, onClear }) {
  const consoleRef = useRef(null);
  const dragRef = useRef(null);
  const [height, setHeight] = useState(initialConsoleHeight);

  useEffect(() => {
    if (lines.length === 0 || !consoleRef.current) return;
    consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
  }, [lines]);

  useEffect(() => {
    try {
      localStorage.setItem(CONSOLE_HEIGHT_KEY, String(height));
    } catch {
      // Height persistence is optional.
    }
  }, [height]);

  useEffect(() => {
    function onResize() {
      setHeight((current) => clampConsoleHeight(current));
    }
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  function startResize(e) {
    e.preventDefault();
    const startY = e.clientY;
    const startHeight = height;
    dragRef.current?.setPointerCapture?.(e.pointerId);

    function onMove(moveEvent) {
      const nextHeight = startHeight + (startY - moveEvent.clientY);
      setHeight(clampConsoleHeight(nextHeight));
    }

    function onUp() {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    }

    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp, { once: true });
  }

  function setPreset(nextHeight) {
    setHeight(clampConsoleHeight(nextHeight));
  }

  return (
    <div className="terminal">
      <button
        className="terminal-resize"
        onPointerDown={startResize}
        ref={dragRef}
        title="resize console"
        type="button"
      />
      <div className="terminal-bar">
        <span className="terminal-dots" aria-hidden="true">
          <span className="terminal-dot red" />
          <span className="terminal-dot yellow" />
          <span className="terminal-dot green" />
        </span>
        <FiTerminal aria-hidden="true" className="terminal-icon" />
        <span className="terminal-label">nex console</span>
        <div className="terminal-size-actions" aria-label="console size">
          <button type="button" onClick={() => setPreset(156)}>fit</button>
          <button type="button" onClick={() => setPreset(280)}>tall</button>
          <button type="button" onClick={() => setPreset(window.innerHeight * 0.72)}>max</button>
        </div>
        <button className="terminal-clear" onClick={onClear} disabled={lines.length === 0}>
          <FiTrash2 aria-hidden="true" className="ui-icon" />
          <span>clear</span>
        </button>
      </div>
      <div className="console terminal-pre" ref={consoleRef} style={{ height }}>
        {lines.length === 0 ? (
          <div className="empty">(nessun output)</div>
        ) : (
          lines.map((l, i) => (
            <div className="log-line" key={i}>
              <span className="ts">{l.time}  </span>
              <span className={l.kind === "err" ? "err-text" : l.kind === "ok" ? "ok-text" : ""}>
                {l.text}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

// ── main app ──────────────────────────────────────────────────────────────────

export default function App() {
  const [info, setInfo] = useState(null);
  const [frameworkInfo, setFrameworkInfo] = useState(null);
  const [frameworkStack, setFrameworkStack] = useState({});
  const [osData, setOsData] = useState(null);
  const [sess, setSess] = useState(null);
  const [beat, setBeat] = useState(0);
  const [lines, setLines] = useState([]);
  const [docsOpen, setDocsOpen] = useState(false);
  const [licenseOpen, setLicenseOpen] = useState(false);
  const [themePreference, setThemePreferenceState] = useState(getThemePreference);
  const [theme, setTheme] = useState(() => resolveTheme(getThemePreference()));

  function log(text, kind = "info") {
    setLines((prev) =>
      [...prev, { time: new Date().toLocaleTimeString(), text, kind }].slice(-200)
    );
  }

  useEffect(() => {
    if (!nex.isReady()) {
      log("nex bridge not injected; backend APIs are disabled outside the native webview.", "info");
      return nex.dragdrop.onFileDrop((payload) =>
        log(`file drop: ${JSON.stringify(payload, null, 2)}`, "ok")
      );
    }

    nex.app.info().then(setInfo).catch((e) => log(`app.info: ${e.message}`, "err"));
    nex.framework.info().then(setFrameworkInfo).catch((e) => log(`framework.info: ${e.message}`, "err"));
    nex.framework.stack().then(setFrameworkStack).catch((e) => log(`framework.stack: ${e.message}`, "err"));
    nex.os.info().then(setOsData).catch(() => {});
    nex.session().then(setSess).catch(() => {});

    // live beat from Go backend
    const off = nex.on("tick", () => setBeat((b) => b + 1));
    const offSecond = nex.on("app.second-instance", (payload) =>
      log(`second instance: ${JSON.stringify(payload)}`, "info")
    );
    const offDrop = nex.dragdrop.onFileDrop((payload) =>
      log(`file drop: ${JSON.stringify(payload, null, 2)}`, "ok")
    );
    return () => {
      off();
      offSecond();
      offDrop();
    };
  }, []);

  useEffect(() => {
    const applied = applyTheme(themePreference);
    setTheme(applied.theme);
    if (themePreference !== "system") return undefined;
    return watchSystemTheme(() => {
      const next = applyTheme("system");
      setTheme(next.theme);
    });
  }, [themePreference]);

  function changeThemePreference(nextPreference) {
    const applied = setThemePreference(nextPreference);
    setThemePreferenceState(applied.preference);
    setTheme(applied.theme);
  }

  // Interactive demos for showcase APIs.
  const demos = useMemo(
    () => {
      const tempRoot = String(
        osData?.paths?.temp ?? (osData?.os === "windows" ? "C:/Windows/Temp" : "/tmp")
      ).replace(/[\\/]+$/, "");
      const demoFile = `${tempRoot}/nex-showcase.txt`;
      const demoRenamedFile = `${tempRoot}/nex-showcase-renamed.txt`;
      const demoDir = `${tempRoot}/nex-showcase-dir`;

      return {
        "sys.framework.info": () => nex.framework.info(),
        "sys.framework.stack": () => nex.framework.stack(),
        "sys.app.info": () => nex.app.info(),
        "sys.app.paths": () => nex.app.paths(),
        "sys.app.runtime": () => nex.app.runtime(),
        "sys.app.restart": () => nex.app.restart(),
        "sys.app.exit": () => nex.app.exit(0),
        "sys.app.quit": () => nex.app.quit(),
        "sys.os.info": () => nex.os.info(),
        "sys.os.host": () => nex.os.host(),
        "sys.os.user": () => nex.os.user(),
        "sys.os.runtime": () => nex.os.runtime(),
        "sys.os.process": () => nex.os.process(),
        "sys.os.network": () => nex.os.network(),
        "sys.os.disks": () => nex.os.disks(),
        "sys.os.memory": () => nex.os.memory(),
        "sys.os.time": () => nex.os.time(),
        "sys.screen.info": () => nex.screen.info(),
        "sys.env.paths": () => nex.env.paths(),
        "sys.dialog.open": () => nex.dialog.open({ title: "Open a file" }),
        "sys.dialog.save": () => nex.dialog.save({ title: "Save as…", defaultName: "new.txt" }),
        "sys.dialog.message": () =>
          nex.dialog.message({ level: "question", title: "Confirm", text: "Continue?" }),
        "sys.dialog.entry": () =>
          nex.dialog.entry({ title: "Input", text: "Type something", defaultText: "nex" }),
        "sys.dialog.password": () =>
          nex.dialog.password({ title: "Password", username: true }),
        "sys.dialog.color": () =>
          nex.dialog.color({ title: "Pick color", showPalette: true }),
        "sys.dialog.isSupported": () => nex.dialog.isSupported(),
        "sys.notify.toast": () =>
          nex.notify.toast({ title: "nex", text: "Native toast from nex", icon: "info" }),
        "sys.notify.capabilities": () => nex.notify.capabilities(),
        "sys.shell.openURL": () => nex.shell.openURL("https://go.dev"),
        "sys.shell.openPath": () => nex.shell.openPath(tempRoot),
        "sys.shell.showInFolder": async () => {
          await nex.fs.write(demoFile, "nex shell demo");
          return nex.shell.showInFolder(demoFile);
        },
        "sys.shell.exec": () =>
          nex.shell.exec(osData?.os === "windows" ? "ver" : "uname -a", { timeoutMs: 5000 }),
        "sys.shell.run": () =>
          nex.shell.run(osData?.os === "windows" ? "echo nex" : "printf nex", { timeoutMs: 5000 }),
        "sys.shell.start": () =>
          nex.shell.start(osData?.os === "windows" ? "echo nex" : "printf nex"),
        "sys.window.info": () => nex.win.info(),
        "sys.window.setTitle": () => nex.win.setTitle(`nex · ${new Date().toLocaleTimeString()}`),
        "sys.window.setSize": () => nex.win.setSize(1040, 760),
        "sys.window.getSize": () => nex.win.getSize(),
        "sys.window.fullscreen": () => nex.win.fullscreen(),
        "sys.window.unfullscreen": () => nex.win.unfullscreen(),
        "sys.window.reload": () => nex.win.reload(),
        "sys.window.print": () => nex.win.print(),
        "sys.window.eval": () =>
          nex.win.eval("window.dispatchEvent(new CustomEvent('nex:frontend-eval-demo',{detail:{ok:true}}))"),
        "sys.appearance.info": () => nex.appearance.info(),
        "sys.power.info": () => nex.power.info(),
        "sys.shortcuts.register": () =>
          nex.shortcuts.register("Mod+Shift+N", () => nex.log.info("shortcut Mod+Shift+N")),
        "sys.shortcuts.unregister": () => nex.shortcuts.unregister("Mod+Shift+N"),
        "sys.shortcuts.clear": () => nex.shortcuts.clear(),
        "sys.shortcuts.list": () => nex.shortcuts.list(),
        "sys.protocol.status": () => nex.protocol.status("nex"),
        "sys.updater.check": () => nex.updater.check(),
        "sys.secureStorage.isAvailable": () => nex.secureStorage.isAvailable(),
        "sys.fileAssociations.status": () => nex.fileAssociations.status(".nex"),
        "sys.recentDocs.add": () => nex.recentDocs.add(osData?.home ?? "/tmp"),
        "sys.recentDocs.list": () => nex.recentDocs.list(),
        "sys.recentDocs.clear": () => nex.recentDocs.clear(),
        "sys.trash.empty": () => nex.trash.empty(),
        "sys.log.print": () => nex.log.print("hello from nex frontend"),
        "sys.log.trace": () => nex.log.trace("trace from nex frontend"),
        "sys.log.debug": () => nex.log.debug("debug from nex frontend"),
        "sys.log.info": () => nex.log.info("info from nex frontend"),
        "sys.log.warning": () => nex.log.warning("warning from nex frontend"),
        "sys.log.error": () => nex.log.error("error from nex frontend"),
        "sys.menu.set": () => nex.menu.set({ items: [{ label: "File", items: [{ label: "Reload", role: "reload", accelerator: "Mod+R" }, { label: "Print", role: "print" }, { label: "Quit", role: "quit" }] }] }),
        "sys.menu.update": () => nex.menu.update(),
        "clipboard.write": async () => { await nex.clipboard.writeText("nex"); return { wrote: "nex" }; },
        "clipboard.read": () => nex.clipboard.readText(),
        "sys.clipboard.writeText": () => nex.clipboard.native.writeText("nex native clipboard"),
        "sys.clipboard.readText": () => nex.clipboard.native.readText(),
        "sys.clipboard.clear": () => nex.clipboard.native.clear(),
        "sys.clipboard.formats": () => nex.clipboard.native.formats(),
        "sys.clipboard.isAvailable": () => nex.clipboard.native.isAvailable(),
        "sys.fs.read": async () => {
          await nex.fs.write(demoFile, "nex fs demo");
          return nex.fs.read(demoFile);
        },
        "sys.fs.write": () => nex.fs.write(demoFile, `nex fs demo ${new Date().toISOString()}`),
        "sys.fs.list": () => nex.fs.list(tempRoot),
        "sys.fs.exists": async () => {
          await nex.fs.write(demoFile, "nex fs demo");
          return nex.fs.exists(demoFile);
        },
        "sys.fs.stat": async () => {
          await nex.fs.write(demoFile, "nex fs demo");
          return nex.fs.stat(demoFile);
        },
        "sys.fs.mkdir": () => nex.fs.mkdir(demoDir),
        "sys.fs.remove": async () => {
          await nex.fs.mkdir(demoDir);
          await nex.fs.write(`${demoDir}/delete-me.txt`, "nex fs remove demo");
          return nex.fs.remove(demoDir, true);
        },
        "sys.fs.rename": async () => {
          await nex.fs.remove(demoRenamedFile).catch(() => {});
          await nex.fs.write(demoFile, "nex fs rename demo");
          return nex.fs.rename(demoFile, demoRenamedFile);
        },
      };
    },
    [osData]
  );

  async function onRun(id, fn) {
    // Privileged APIs are gated backend-side (internal/app/app.go,
    // Authorize -> confirmPrivileged): the Go framework itself shows a
    // native confirmation dialog before running them, so every frontend
    // (this showcase or any other) gets the same guarantee without having
    // to reimplement it in JS.
    log(`→ ${id}`);
    try {
      const r = await fn();
      const safeResult = maskDemoResult(id, r);
      const s = JSON.stringify(safeResult, null, 2);
      log(s.length > 600 ? s.slice(0, 600) + "\n…" : s, "ok");
    } catch (e) {
      log(`✗ ${e.message}`, "err");
    }
  }

  function maskDemoResult(id, result) {
    if (id !== "sys.dialog.password" || !result || typeof result !== "object") {
      return result;
    }
    return {
      ...result,
      password: result.password ? "***" : result.password,
    };
  }

  const appMeta = info?.app ?? info;
  const pub = info?.public ?? {};
  const appPlatform = osData ? `${osData.os}/${osData.arch} · ${osData.cpus} cpu` : "…";
  const appVersion = appMeta?.version ?? "…";
  const appBuild = appMeta?.build ?? "…";
  const frameworkName = frameworkInfo ? `${frameworkInfo.name}@${frameworkInfo.version}` : "…";
  const frameworkBuild = frameworkInfo ? `${frameworkInfo.build} · ${frameworkInfo.updated}` : "…";
  const resolvedStackBadges = stackBadges.map((badge) => ({
    ...badge,
    value: badge.key === "nex"
      ? (frameworkInfo ? `${frameworkInfo.version}-${frameworkInfo.build}` : "…")
      : badge.key ? (frameworkStack[badge.key] ?? frameworkInfo?.stack?.[badge.key] ?? badge.value) : badge.value,
  }));

  return (
    <>
      <StatusBar
        info={info}
        sess={sess}
        beat={beat}
        theme={theme}
        themePreference={themePreference}
        onThemeChange={changeThemePreference}
        onDocsOpen={() => setDocsOpen(true)}
        onLicenseOpen={() => setLicenseOpen(true)}
      />
      <FileModal
        open={docsOpen}
        onClose={() => setDocsOpen(false)}
        onLog={log}
        file={docsFile}
        kicker="documentation"
        format="markdown"
      />
      <FileModal
        open={licenseOpen}
        onClose={() => setLicenseOpen(false)}
        onLog={log}
        file={licenseFile}
        kicker="license"
        format="text"
      />
      <main className="wrap">
        <NexLogo className="brand-intro" />

        {/* Framework intro */}
        <section className="section">
          <div className="framework-note">
            <div>
              <span className="framework-eyebrow">framework</span>
              <h2>nex turns React, Vite, and Go into one native desktop app.</h2>
            </div>
            <div className="framework-badges" aria-label="framework stack versions">
              {resolvedStackBadges.map((badge) => (
                <span className={`stack-badge ${badge.tone}`} key={`${badge.label}-${badge.value}`}>
                  <span className="stack-badge-label">{badge.label}</span>
                  <span className="stack-badge-value">{badge.value}</span>
                </span>
              ))}
            </div>
            <div className="framework-points">
              <span><FiFileText aria-hidden="true" className="ui-icon" />Embedded frontend assets</span>
              <span><FiZap aria-hidden="true" className="ui-icon" />Secure loopback RPC bridge</span>
              <span><FiCpu aria-hidden="true" className="ui-icon" />Native dialog, notify, fs, shell, window APIs</span>
            </div>
          </div>
        </section>

        {/* App info */}
        <section className="section">
          <span className="label section-title">
            <FiInfo aria-hidden="true" className="ui-icon" />
            App
          </span>
          <div className="panel">
            <div className="kv">
              <div className="k">name</div>
              <div>{appMeta?.name ?? "…"}</div>
              <div className="k">version</div>
              <div>{appVersion}</div>
              <div className="k">build</div>
              <div>{appBuild}</div>
              <div className="k">author</div>
              <div>{appMeta?.author ?? "…"}</div>
              <div className="k">platform</div>
              <div>{appPlatform}</div>
            </div>
          </div>
        </section>

        {/* Framework info */}
        <section className="section">
          <span className="label section-title">
            <FiLayers aria-hidden="true" className="ui-icon" />
            Framework
          </span>
          <div className="panel">
            <div className="kv">
              <div className="k">framework</div>
              <div>{frameworkName}</div>
              <div className="k">build · updated</div>
              <div>{frameworkBuild}</div>
              <div className="k">author</div>
              <div>{frameworkInfo?.author ?? "…"}</div>
            </div>
          </div>
        </section>

        {/* Public env vars */}
        <section className="section">
          <span className="label">Public env (VITE_*)</span>
          <div className="panel">
            <div className="env-grid">
              {Object.keys(pub).length === 0 ? (
                <div className="env-row">
                  <span className="env-val">No public variables loaded.</span>
                </div>
              ) : (
                Object.entries(pub).map(([k, v]) => (
                  <div className="env-row" key={k}>
                    <span className="env-key">{k}</span>
                    <span className="env-val">{v}</span>
                  </div>
                ))
              )}
            </div>
            <div className="note">
              Variables without the VITE_ prefix stay backend-only and never reach the frontend.
            </div>
          </div>
        </section>

        {/* API explorer */}
        <section className="section">
          <span className="label">Available APIs</span>
          <ApiExplorer demos={demos} onRun={onRun} />
        </section>

        {/* Console */}
        <section className="section console-dock">
          <Console lines={lines} onClear={() => setLines([])} />
        </section>

      </main>
    </>
  );
}
