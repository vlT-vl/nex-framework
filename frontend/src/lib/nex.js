/**
 * nex client bridge — pure JavaScript, no TypeScript, no external deps.
 *
 * The Go backend injects window.__NEX__ = { token, base } before any page
 * script runs (via webview.Init). This module reads it and provides:
 *
 *   call(method, params?)  — RPC call to the Go backend
 *   on(event, handler)     — subscribe to backend events; returns unsubscribe fn
 *   session()              — fetch current session info
 *
 *   os.info()
 *   os.host()
 *   os.user()
 *   os.runtime()
 *   os.process()
 *   os.network()
 *   os.disks()
 *   os.memory()
 *   os.time()
 *   screen.info()
 *   env.paths()
 *   dialog.open(opts?)
 *   dialog.save(opts?)
 *   dialog.message(opts)
 *   notify.toast(opts)
 *   shell.openURL(url)
 *   shell.openPath(path)
 *   shell.showInFolder(path)
 *   shell.exec(command, opts?)
 *   shell.run(command, opts?)
 *   shell.start(command, opts?)
 *   win.setTitle(title)
 *   win.setSize(w, h, hint?)
 *   win.info()
 *   win.fullscreen()
 *   win.unfullscreen()
 *   win.reload()
 *   win.print()
 *   win.eval(js)
 *   fs.read(path, encoding?)
 *   fs.write(path, data, encoding?)
 *   fs.list(path)
 *   fs.exists(path)
 *   fs.stat(path)
 *   fs.mkdir(path)
 *   fs.remove(path, recursive?)
 *   fs.rename(from, to)
 *   clipboard.writeText(text)   — DOM navigator.clipboard (no backend round-trip)
 *   clipboard.readText()        — DOM navigator.clipboard
 *   app.info()
 *   framework.info()
 *   app.paths()
 *   app.runtime()
 *   app.restart()
 *   app.quit()
 *   native clipboard, menu, appearance, power, shortcuts, protocol, updater,
 *   secure storage availability, file associations and recent documents helpers
 *   log.info(message)
 *   dragdrop.onFileDrop(callback, target?)
 */

const NEX = (typeof window !== "undefined" && window.__NEX__) || { token: "", base: "" };
const API_BASE = NEX.base
  ? `${NEX.base.replace(/\/$/, "")}/api`
  : import.meta.env.VITE_API_BASE ?? "/api";

// ── Core RPC ─────────────────────────────────────────────────────────────────

export class nexError extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
    this.name = "nexError";
  }
}

export function isReady() {
  return Boolean(NEX.token && NEX.base);
}

function requireBridge() {
  if (!isReady()) {
    throw new nexError(
      "bridge_unavailable",
      "nex bridge is not injected; open the app through the native webview"
    );
  }
}

/**
 * Call a backend RPC method.
 * @param {string} method
 * @param {unknown} [params]
 * @returns {Promise<unknown>}
 */
export async function call(method, params = {}) {
  requireBridge();
  const res = await fetch(`${API_BASE}/rpc`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-nex-Token": NEX.token,
    },
    body: JSON.stringify({ method, params }),
  });
  if (!res.ok) throw new nexError(`http_${res.status}`, `HTTP ${res.status}`);
  const data = await res.json();
  if (data.error) throw new nexError(data.error.code, data.error.message);
  return data.result;
}

/**
 * Subscribe to a backend event.
 * @param {string} event
 * @param {(payload: unknown) => void} handler
 * @returns {() => void} unsubscribe
 */
export function on(event, handler) {
  const listener = (e) => handler(e.detail);
  window.addEventListener(`nex:${event}`, listener);
  return () => window.removeEventListener(`nex:${event}`, listener);
}

/** Fetch current session metadata. */
export async function session() {
  requireBridge();
  const res = await fetch(`${API_BASE}/session`, {
    headers: { "X-nex-Token": NEX.token },
  });
  if (!res.ok) throw new nexError(`http_${res.status}`, `HTTP ${res.status}`);
  return res.json();
}

// ── Webview runtime integrations ─────────────────────────────────────────────

const shortcutHandlers = new Map();
let shortcutListenerInstalled = false;

function ensureShortcutListener() {
  if (shortcutListenerInstalled || typeof window === "undefined") return;
  shortcutListenerInstalled = true;
  window.addEventListener("keydown", (event) => {
    for (const [id, item] of shortcutHandlers) {
      if (!item.enabled || !acceleratorMatches(event, item.accelerator)) continue;
      if (!item.allowInput && isEditableTarget(event.target)) return;
      event.preventDefault();
      const detail = { id, accelerator: item.accelerator, event };
      if (typeof item.handler === "function") item.handler(detail);
      window.dispatchEvent(new CustomEvent("nex:shortcut", { detail }));
      break;
    }
  });
}

function isEditableTarget(target) {
  const tag = target?.tagName?.toLowerCase();
  return target?.isContentEditable || tag === "input" || tag === "textarea" || tag === "select";
}

function acceleratorMatches(event, accelerator) {
  const parts = String(accelerator).toLowerCase().split("+").map((part) => part.trim());
  const key = parts.at(-1);
  const wants = new Set(parts.slice(0, -1));
  const meta = event.metaKey || event.ctrlKey;
  if ((wants.has("cmd") || wants.has("command") || wants.has("ctrl") || wants.has("control") || wants.has("mod")) && !meta) return false;
  if (wants.has("shift") && !event.shiftKey) return false;
  if ((wants.has("alt") || wants.has("option")) && !event.altKey) return false;
  const eventKey = normalizeKey(event.key);
  return eventKey === normalizeKey(key);
}

function normalizeKey(key) {
  const value = String(key || "").toLowerCase();
  if (value === " ") return "space";
  if (value === "esc") return "escape";
  if (value === "arrowup") return "up";
  if (value === "arrowdown") return "down";
  if (value === "arrowleft") return "left";
  if (value === "arrowright") return "right";
  return value;
}

function injectMenuStyles() {
  if (document.getElementById("nex-runtime-menu-style")) return;
  const style = document.createElement("style");
  style.id = "nex-runtime-menu-style";
  style.textContent = `
    #nex-runtime-menu {
      position: fixed;
      z-index: 2147483646;
      top: 0;
      left: 0;
      right: 0;
      min-height: 30px;
      display: flex;
      align-items: stretch;
      gap: 2px;
      padding: 0 8px;
      border-bottom: 1px solid rgba(128, 128, 128, 0.25);
      background: color-mix(in srgb, Canvas 92%, transparent);
      color: CanvasText;
      font: 13px system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      backdrop-filter: blur(12px);
    }
    #nex-runtime-menu button {
      min-width: 54px;
      border: 0;
      border-radius: 0;
      padding: 0 10px;
      background: transparent;
      color: inherit;
      font: inherit;
    }
    #nex-runtime-menu button:hover,
    #nex-runtime-menu button:focus-visible {
      background: color-mix(in srgb, Highlight 18%, transparent);
      outline: none;
    }
    #nex-runtime-menu [data-menu-group] {
      position: relative;
      display: flex;
    }
    #nex-runtime-menu [data-menu-items] {
      position: absolute;
      top: 100%;
      left: 0;
      min-width: 190px;
      display: none;
      flex-direction: column;
      padding: 5px;
      border: 1px solid rgba(128, 128, 128, 0.28);
      background: Canvas;
      color: CanvasText;
      box-shadow: 0 12px 32px rgba(0, 0, 0, 0.18);
    }
    #nex-runtime-menu [data-menu-group]:hover [data-menu-items],
    #nex-runtime-menu [data-menu-group]:focus-within [data-menu-items] {
      display: flex;
    }
    #nex-runtime-menu [data-menu-items] button {
      width: 100%;
      min-height: 28px;
      display: flex;
      justify-content: space-between;
      gap: 16px;
      text-align: left;
      white-space: nowrap;
    }
    body:has(#nex-runtime-menu) {
      padding-top: max(30px, env(safe-area-inset-top));
    }
  `;
  document.head.appendChild(style);
}

function normalizeMenuDefinition(definition) {
  if (Array.isArray(definition)) return { items: definition };
  if (definition?.items) return definition;
  return { items: [] };
}

function renderRuntimeMenu(definition) {
  if (typeof document === "undefined") return;
  const normalized = normalizeMenuDefinition(definition);
  const existing = document.getElementById("nex-runtime-menu");
  if (!normalized.items.length) {
    existing?.remove();
    return;
  }
  injectMenuStyles();
  const root = existing || document.createElement("nav");
  root.id = "nex-runtime-menu";
  root.setAttribute("aria-label", "application menu");
  root.replaceChildren();
  for (const group of normalized.items) {
    const wrapper = document.createElement("div");
    wrapper.dataset.menuGroup = "";
    const trigger = document.createElement("button");
    trigger.type = "button";
    trigger.textContent = group.label || group.title || "Menu";
    wrapper.appendChild(trigger);
    const panel = document.createElement("div");
    panel.dataset.menuItems = "";
    panel.setAttribute("role", "menu");
    for (const item of group.items || []) {
      const button = document.createElement("button");
      button.type = "button";
      button.disabled = item.disabled === true;
      button.setAttribute("role", "menuitem");
      button.innerHTML = `<span>${escapeHTML(item.label || item.role || item.action || "Item")}</span><span>${escapeHTML(item.accelerator || "")}</span>`;
      button.addEventListener("click", () => runMenuItem(item));
      panel.appendChild(button);
    }
    wrapper.appendChild(panel);
    root.appendChild(wrapper);
  }
  if (!existing) document.body.prepend(root);
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[char]);
}

async function runMenuItem(item) {
  const detail = { item };
  window.dispatchEvent(new CustomEvent("nex:menu.action", { detail }));
  switch (item.role) {
    case "quit":
      return app.quit();
    case "reload":
      return win.reload();
    case "print":
      return win.print();
    case "fullscreen":
      return win.fullscreen();
    case "unfullscreen":
      return win.unfullscreen();
    case "about":
      return dialog.message({ title: "About", text: item.text || document.title || "nex" });
    default:
      if (item.url) return shell.openURL(item.url);
      if (item.path) return shell.openPath(item.path);
      return detail;
  }
}

on("menu.changed", (definition) => renderRuntimeMenu(definition));

// ── Namespaced helpers ────────────────────────────────────────────────────────

export const os = {
  info: () => call("sys.os.info"),
  host: () => call("sys.os.host"),
  user: () => call("sys.os.user"),
  runtime: () => call("sys.os.runtime"),
  process: () => call("sys.os.process"),
  network: () => call("sys.os.network"),
  disks: () => call("sys.os.disks"),
  memory: () => call("sys.os.memory"),
  time: () => call("sys.os.time"),
  paths: () => call("sys.env.paths"),
};

export const screen = {
  async info() {
    const browser = {
      width: window.screen?.width ?? 0,
      height: window.screen?.height ?? 0,
      availWidth: window.screen?.availWidth ?? 0,
      availHeight: window.screen?.availHeight ?? 0,
      colorDepth: window.screen?.colorDepth ?? 0,
      pixelDepth: window.screen?.pixelDepth ?? 0,
      devicePixelRatio: window.devicePixelRatio ?? 1,
      innerWidth: window.innerWidth,
      innerHeight: window.innerHeight,
      outerWidth: window.outerWidth,
      outerHeight: window.outerHeight,
    };
    try {
      return { browser, native: await call("sys.screen.info") };
    } catch (error) {
      return { browser, native: null, error: error.message };
    }
  },
  native: () => call("sys.screen.info"),
};

export const env = {
  paths: () => call("sys.env.paths"),
};

export const dialog = {
  /** @param {{ title?:string, multiple?:boolean, directory?:boolean, startDir?:string, filters?:Array<{name:string,patterns:string[]}>}} [opts] */
  open: (opts = {}) => call("sys.dialog.open", opts),
  /** @param {{ title?:string, defaultName?:string, confirmOverwrite?:boolean, filters?:Array<{name:string,patterns:string[]}>}} [opts] */
  save: (opts = {}) => call("sys.dialog.save", opts),
  /** @param {{ level?:"info"|"warning"|"error"|"question", title?:string, text:string }} opts */
  message: (opts) => call("sys.dialog.message", opts),
  entry: (opts = {}) => call("sys.dialog.entry", opts),
  password: (opts = {}) => call("sys.dialog.password", opts),
  color: (opts = {}) => call("sys.dialog.color", opts),
  /** Check if native file dialogs are available on this platform (always true on macOS/Windows; heuristic on Linux). */
  isSupported: () => call("sys.dialog.isSupported"),
};

export const notify = {
  /** @param {{ title?:string, text:string, icon?:string }} opts */
  toast: (opts) => call("sys.notify.toast", opts),
  capabilities: () => call("sys.notify.capabilities"),
};

export const shell = {
  openURL: (url) => call("sys.shell.openURL", { url }),
  openPath: (path) => call("sys.shell.openPath", { path }),
  showInFolder: (path) => call("sys.shell.showInFolder", { path }),
  /** @param {{cwd?:string, env?:Record<string,string>, timeoutMs?:number}} [opts] */
  exec: (command, opts = {}) => call("sys.shell.exec", { command, ...opts }),
  /** @param {{cwd?:string, env?:Record<string,string>, timeoutMs?:number}} [opts] */
  run: (command, opts = {}) => call("sys.shell.run", { command, ...opts }),
  /** @param {{cwd?:string, env?:Record<string,string>}} [opts] */
  start: (command, opts = {}) => call("sys.shell.start", { command, ...opts }),
};

export const win = {
  info: () => call("sys.window.info"),
  setTitle: (title) => call("sys.window.setTitle", { title }),
  /** @param {"none"|"min"|"max"|"fixed"} [hint] */
  setSize: (width, height, hint = "none") =>
    call("sys.window.setSize", { width, height, hint }),
  getSize: () => call("sys.window.getSize"),
  fullscreen: async () => {
    await document.documentElement.requestFullscreen?.().catch(() => {});
    return call("sys.window.fullscreen");
  },
  unfullscreen: async () => {
    if (document.fullscreenElement) await document.exitFullscreen?.().catch(() => {});
    return call("sys.window.unfullscreen");
  },
  reload: () => call("sys.window.reload"),
  print: async () => {
    window.print();
    return { ok: true };
  },
  eval: (js) => call("sys.window.eval", { js }),
};

export const fs = {
  /** @param {"utf8"|"base64"} [encoding] */
  read: (path, encoding = "utf8") => call("sys.fs.read", { path, encoding }),
  /** @param {"utf8"|"base64"} [encoding] */
  write: (path, data, encoding = "utf8") =>
    call("sys.fs.write", { path, data, encoding }),
  list: (path) => call("sys.fs.list", { path }),
  exists: (path) => call("sys.fs.exists", { path }),
  stat: (path) => call("sys.fs.stat", { path }),
  mkdir: (path) => call("sys.fs.mkdir", { path }),
  remove: (path, recursive = false) => call("sys.fs.remove", { path, recursive }),
  rename: (from, to) => call("sys.fs.rename", { from, to }),
};

/** Clipboard via DOM (no backend round-trip needed — webview is a secure context). */
export const clipboard = {
  writeText: (text) => navigator.clipboard.writeText(text),
  readText: () => navigator.clipboard.readText(),
  /**
   * Returns a `{ onKeyDown, onPaste }` handler pair for password/secret inputs.
   * Uses native OS clipboard on Cmd/Ctrl+V (reliable in webviews where
   * navigator.clipboard.readText() may be blocked), with onPaste as primary path.
   * @param {(text: string) => void} onChange
   */
  pasteHandler: (onChange) => ({
    onPaste: (e) => {
      const text = e.clipboardData?.getData("text");
      if (text == null) return;
      e.preventDefault();
      onChange(text);
    },
    onKeyDown: async (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "v") {
        e.preventDefault();
        try {
          const { text } = await call("sys.clipboard.readText");
          onChange(text ?? "");
        } catch {
          try { onChange(await navigator.clipboard.readText()); } catch {}
        }
      }
    },
  }),
  native: {
    readText: () => call("sys.clipboard.readText"),
    writeText: (text) => call("sys.clipboard.writeText", { text }),
    clear: () => call("sys.clipboard.clear"),
    formats: () => call("sys.clipboard.formats"),
    /** Check native clipboard availability — always true on macOS/Windows/Linux (native NSPasteboard/user32/GtkClipboard, no external tool dependency). */
    isAvailable: () => call("sys.clipboard.isAvailable"),
  },
};

export const app = {
  info: () => call("sys.app.info"),
  paths: () => call("sys.app.paths"),
  runtime: () => call("sys.app.runtime"),
  restart: () => call("sys.app.restart"),
  exit: (code = 0) => call("sys.app.exit", { code }),
  quit: () => call("sys.app.quit"),
};

export const framework = {
  info: () => call("sys.framework.info"),
  stack: () => call("sys.framework.stack"),
};

export const log = {
  print: (message) => call("sys.log.print", { message }),
  trace: (message) => call("sys.log.trace", { message }),
  debug: (message) => call("sys.log.debug", { message }),
  info: (message) => call("sys.log.info", { message }),
  warning: (message) => call("sys.log.warning", { message }),
  error: (message) => call("sys.log.error", { message }),
};

export const menu = {
  async set(definition) {
    const result = await call("sys.menu.set", definition);
    renderRuntimeMenu(result.menu ?? definition);
    return result;
  },
  async update() {
    const result = await call("sys.menu.update");
    renderRuntimeMenu(result.menu);
    return result;
  },
  onAction: (handler) => on("menu.action", handler),
};

export const appearance = {
  info: () => call("sys.appearance.info"),
};

export const power = {
  info: () => call("sys.power.info"),
};

export const shortcuts = {
  async register(accelerator, handler, opts = {}) {
    if (typeof accelerator === "object") {
      opts = accelerator;
      accelerator = opts.accelerator;
      handler = opts.handler;
    }
    const result = await call("sys.shortcuts.register", { accelerator, ...opts, handler: undefined });
    const item = result.shortcut;
    shortcutHandlers.set(item.id, {
      accelerator: item.accelerator,
      enabled: item.enabled !== false,
      allowInput: opts.allowInput === true,
      handler,
    });
    ensureShortcutListener();
    return {
      ...result,
      dispose: () => shortcuts.unregister(item.id),
    };
  },
  async unregister(idOrAccelerator) {
    const params = String(idOrAccelerator).includes("+")
      ? { accelerator: idOrAccelerator }
      : { id: idOrAccelerator };
    const result = await call("sys.shortcuts.unregister", params);
    shortcutHandlers.delete(result.id);
    return result;
  },
  async clear() {
    const result = await call("sys.shortcuts.clear");
    shortcutHandlers.clear();
    return result;
  },
  list: () => call("sys.shortcuts.list"),
  onShortcut: (handler) => on("shortcut", handler),
};

export const protocol = {
  status: (scheme) => call("sys.protocol.status", { scheme }),
};

export const updater = {
  check: (opts = {}) => call("sys.updater.check", opts),
};

export const secureStorage = {
  isAvailable: () => call("sys.secureStorage.isAvailable"),
};

export const fileAssociations = {
  status: (extension) => call("sys.fileAssociations.status", { extension }),
};

export const recentDocs = {
  add: (path) => call("sys.recentDocs.add", { path }),
  clear: () => call("sys.recentDocs.clear"),
  list: () => call("sys.recentDocs.list"),
};

/**
 * Native OS trash/recycle bin. `empty()` permanently deletes its current
 * contents (no undo) — the backend itself asks for native confirmation
 * before running it unless the app has installed its own SecurityPolicy/hook.
 */
export const trash = {
  /** @returns {Promise<{ ok: boolean, platform: string, removed?: number }>} */
  empty: () => call("sys.trash.empty"),
};

export const dragdrop = {
  onFileDrop(callback, target = window) {
    const over = (event) => {
      event.preventDefault();
      event.dataTransfer.dropEffect = "copy";
    };
    const drop = (event) => {
      event.preventDefault();
      const files = Array.from(event.dataTransfer?.files ?? []).map((file) => ({
        name: file.name,
        size: file.size,
        type: file.type,
        lastModified: file.lastModified,
        path: file.path || file.webkitRelativePath || "",
      }));
      callback({
        x: event.clientX,
        y: event.clientY,
        files,
        paths: files.map((file) => file.path).filter(Boolean),
      });
    };
    target.addEventListener("dragover", over);
    target.addEventListener("drop", drop);
    return () => {
      target.removeEventListener("dragover", over);
      target.removeEventListener("drop", drop);
    };
  },
};

const privilegedApiIds = new Set([
  "sys.app.restart",
  "sys.app.exit",
  "sys.app.quit",
  "sys.dialog.password",
  "sys.shell.exec",
  "sys.shell.run",
  "sys.shell.start",
  "sys.window.eval",
  "sys.fs.read",
  "sys.fs.write",
  "sys.fs.list",
  "sys.fs.exists",
  "sys.fs.stat",
  "sys.fs.mkdir",
  "sys.fs.remove",
  "sys.fs.rename",
  "clipboard.write",
  "clipboard.read",
  "sys.clipboard.readText",
  "sys.clipboard.writeText",
  "sys.clipboard.clear",
  "sys.trash.empty",
]);

const userMediatedApiIds = new Set([
  "sys.dialog.open",
  "sys.dialog.save",
  "sys.dialog.message",
  "sys.dialog.entry",
  "sys.dialog.color",
  "sys.notify.toast",
  "sys.shell.openURL",
  "sys.shell.openPath",
  "sys.shell.showInFolder",
  "sys.window.print",
]);

export function apiRiskLevel(id) {
  if (privilegedApiIds.has(id)) return "privileged";
  if (userMediatedApiIds.has(id)) return "user";
  return "safe";
}

// ── API catalog (for dashboard/docs) ─────────────────────────────────────────

export const apiCatalog = [
  {
    namespace: "framework",
    methods: [
      { id: "sys.framework.info", sig: "framework.info()", desc: "nex framework name, version, build and author." },
      { id: "sys.framework.stack", sig: "framework.stack()", desc: "Resolved nex runtime dependency versions." },
    ],
  },
  {
    namespace: "app",
    methods: [
      { id: "sys.app.info", sig: "app.info()", desc: "Application metadata, public env vars, paths and runtime." },
      { id: "sys.app.paths", sig: "app.paths()", desc: "Home, config, cache, temp, executable and cwd paths." },
      { id: "sys.app.runtime", sig: "app.runtime()", desc: "Runtime mode, pid, executable, OS and architecture." },
      { id: "sys.app.restart", sig: "app.restart()", desc: "Start a new process for the current executable and exit this one." },
      { id: "sys.app.exit", sig: "app.exit(code?)", desc: "Exit the current process with an explicit code." },
      { id: "sys.app.quit", sig: "app.quit()", desc: "Close the application." },
    ],
  },
  {
    namespace: "os",
    methods: [
      { id: "sys.os.info", sig: "os.info()", desc: "Full system snapshot: host, user, runtime, process, network, disks, memory, time." },
      { id: "sys.os.host", sig: "os.host()", desc: "OS, kernel, hostname, CPU, uptime and platform details." },
      { id: "sys.os.user", sig: "os.user()", desc: "Current OS user, ids and home directory." },
      { id: "sys.os.runtime", sig: "os.runtime()", desc: "Go runtime, build metadata and runtime memory stats." },
      { id: "sys.os.process", sig: "os.process()", desc: "Current process pid, executable, cwd, args and environment summary." },
      { id: "sys.os.network", sig: "os.network()", desc: "Network interfaces, flags, MAC addresses and assigned addresses." },
      { id: "sys.os.disks", sig: "os.disks()", desc: "Mounted volumes or logical disks with size and free space when available." },
      { id: "sys.os.memory", sig: "os.memory()", desc: "System memory plus Go runtime allocator stats." },
      { id: "sys.os.time", sig: "os.time()", desc: "Local/UTC time, Unix timestamps and timezone offset." },
      { id: "sys.env.paths", sig: "env.paths()", desc: "Standard paths: home, config, cache, temp." },
    ],
  },
  {
    namespace: "screen",
    methods: [
      { id: "sys.screen.info", sig: "screen.info()", desc: "Browser screen metrics plus native display details when available." },
    ],
  },
  {
    namespace: "dialog",
    methods: [
      { id: "sys.dialog.open", sig: "dialog.open(opts?)", desc: "Native file/folder picker." },
      { id: "sys.dialog.save", sig: "dialog.save(opts?)", desc: "Native save dialog." },
      { id: "sys.dialog.message", sig: "dialog.message(opts)", desc: "Native message box (info/warning/error/question)." },
      { id: "sys.dialog.entry", sig: "dialog.entry(opts?)", desc: "Native text entry dialog." },
      { id: "sys.dialog.password", sig: "dialog.password(opts?)", desc: "Native password/username dialog." },
      { id: "sys.dialog.color", sig: "dialog.color(opts?)", desc: "Native color picker." },
      { id: "sys.dialog.isSupported", sig: "dialog.isSupported()", desc: "Check if native file dialogs are available (always true on macOS/Windows; heuristic on Linux)." },
    ],
  },
  {
    namespace: "notify",
    methods: [
      { id: "sys.notify.toast", sig: "notify.toast(opts)", desc: "Native OS toast notification with title, text and icon." },
      { id: "sys.notify.capabilities", sig: "notify.capabilities()", desc: "Notification backend feature matrix." },
    ],
  },
  {
    namespace: "shell",
    methods: [
      { id: "sys.shell.openURL", sig: "shell.openURL(url)", desc: "Open URL in system browser." },
      { id: "sys.shell.openPath", sig: "shell.openPath(path)", desc: "Open file with default app." },
      { id: "sys.shell.showInFolder", sig: "shell.showInFolder(path)", desc: "Reveal file in file manager." },
      { id: "sys.shell.exec", sig: "shell.exec(command, opts?)", desc: "Run a native shell command and capture output." },
      { id: "sys.shell.run", sig: "shell.run(command, opts?)", desc: "Alias for shell.exec." },
      { id: "sys.shell.start", sig: "shell.start(command, opts?)", desc: "Start a native shell command and return its pid." },
    ],
  },
  {
    namespace: "window",
    methods: [
      { id: "sys.window.info", sig: "win.info()", desc: "Known window title, size and hint tracked by nex." },
      { id: "sys.window.setTitle", sig: "win.setTitle(title)", desc: "Set window title." },
      { id: "sys.window.setSize", sig: "win.setSize(w, h, hint?)", desc: "Resize window." },
      { id: "sys.window.getSize", sig: "win.getSize()", desc: "Read the last known window size." },
      { id: "sys.window.fullscreen", sig: "win.fullscreen()", desc: "Enter DOM fullscreen mode." },
      { id: "sys.window.unfullscreen", sig: "win.unfullscreen()", desc: "Exit DOM fullscreen mode." },
      { id: "sys.window.reload", sig: "win.reload()", desc: "Reload the frontend document." },
      { id: "sys.window.print", sig: "win.print()", desc: "Open the native/browser print dialog." },
      { id: "sys.window.eval", sig: "win.eval(js)", desc: "Evaluate JavaScript inside the webview." },
    ],
  },
  {
    namespace: "log",
    methods: [
      { id: "sys.log.print", sig: "log.print(message)", desc: "Write a raw message to the Go logger." },
      { id: "sys.log.trace", sig: "log.trace(message)", desc: "Write a trace message to the Go logger." },
      { id: "sys.log.debug", sig: "log.debug(message)", desc: "Write a debug message to the Go logger." },
      { id: "sys.log.info", sig: "log.info(message)", desc: "Write an info message to the Go logger." },
      { id: "sys.log.warning", sig: "log.warning(message)", desc: "Write a warning message to the Go logger." },
      { id: "sys.log.error", sig: "log.error(message)", desc: "Write an error message to the Go logger." },
    ],
  },
  {
    namespace: "menu",
    methods: [
      { id: "sys.menu.set", sig: "menu.set(definition)", desc: "Set and render the runtime application menu in the webview." },
      { id: "sys.menu.update", sig: "menu.update()", desc: "Replay the current runtime menu definition." },
    ],
  },
  {
    namespace: "dragdrop",
    methods: [
      { id: "dragdrop.onFileDrop", sig: "dragdrop.onFileDrop(cb, target?)", desc: "Handle file/folder drops in the webview via DOM events." },
    ],
  },
  {
    namespace: "fs",
    methods: [
      { id: "sys.fs.read", sig: "fs.read(path, enc?)", desc: "Read file (utf8 | base64)." },
      { id: "sys.fs.write", sig: "fs.write(path, data, enc?)", desc: "Write file." },
      { id: "sys.fs.list", sig: "fs.list(path)", desc: "List directory." },
      { id: "sys.fs.exists", sig: "fs.exists(path)", desc: "Check existence." },
      { id: "sys.fs.stat", sig: "fs.stat(path)", desc: "File metadata." },
      { id: "sys.fs.mkdir", sig: "fs.mkdir(path)", desc: "Create directory (recursive)." },
      { id: "sys.fs.remove", sig: "fs.remove(path, rec?)", desc: "Remove file/directory." },
      { id: "sys.fs.rename", sig: "fs.rename(from, to)", desc: "Rename/move." },
    ],
  },
  {
    namespace: "clipboard",
    methods: [
      { id: "clipboard.write", sig: "clipboard.writeText(s)", desc: "Write to clipboard (DOM)." },
      { id: "clipboard.read", sig: "clipboard.readText()", desc: "Read from clipboard (DOM)." },
      { id: "sys.clipboard.readText", sig: "clipboard.native.readText()", desc: "Read text via the native OS clipboard (NSPasteboard/user32/GtkClipboard)." },
      { id: "sys.clipboard.writeText", sig: "clipboard.native.writeText(s)", desc: "Write text via the native OS clipboard (NSPasteboard/user32/GtkClipboard)." },
      { id: "sys.clipboard.clear", sig: "clipboard.native.clear()", desc: "Clear native clipboard text." },
      { id: "sys.clipboard.formats", sig: "clipboard.native.formats()", desc: "Native clipboard format support matrix." },
      { id: "sys.clipboard.isAvailable", sig: "clipboard.native.isAvailable()", desc: "Check native clipboard availability — always true on macOS/Windows/Linux (no external tool dependency)." },
    ],
  },
  {
    namespace: "desktop",
    methods: [
      { id: "sys.appearance.info", sig: "appearance.info()", desc: "System appearance and dark-mode data exposed by the host OS." },
      { id: "sys.power.info", sig: "power.info()", desc: "Battery and power information exposed by the host OS." },
      { id: "sys.shortcuts.register", sig: "shortcuts.register(accelerator, handler?, opts?)", desc: "Register a webview-window keyboard shortcut." },
      { id: "sys.shortcuts.unregister", sig: "shortcuts.unregister(idOrAccelerator)", desc: "Unregister a runtime keyboard shortcut." },
      { id: "sys.shortcuts.clear", sig: "shortcuts.clear()", desc: "Clear runtime keyboard shortcuts." },
      { id: "sys.shortcuts.list", sig: "shortcuts.list()", desc: "List runtime keyboard shortcuts." },
      { id: "sys.protocol.status", sig: "protocol.status(scheme)", desc: "Inspect OS protocol/deep-link registration for a scheme." },
      { id: "sys.updater.check", sig: "updater.check(opts?)", desc: "Check a JSON update provider." },
      { id: "sys.secureStorage.isAvailable", sig: "secureStorage.isAvailable()", desc: "Detect OS secure storage backend availability." },
      { id: "sys.fileAssociations.status", sig: "fileAssociations.status(ext)", desc: "Inspect OS file association for an extension." },
      { id: "sys.recentDocs.add", sig: "recentDocs.add(path)", desc: "Persist a recent document path." },
      { id: "sys.recentDocs.list", sig: "recentDocs.list()", desc: "List persisted recent documents." },
      { id: "sys.recentDocs.clear", sig: "recentDocs.clear()", desc: "Clear persisted recent documents." },
    ],
  },
  {
    namespace: "trash",
    methods: [
      { id: "sys.trash.empty", sig: "trash.empty()", desc: "Permanently empty the native OS trash/recycle bin (no undo)." },
    ],
  },
];
