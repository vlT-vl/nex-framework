// Package app wires together webview, local HTTP server, session security,
// RPC dispatch, and backend-to-frontend events.
package app

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ncruces/zenity"
	webview "github.com/webview/webview_go"

	"nex/config"
	"nex/internal/core"
	"nex/internal/meta"
	"nex/internal/session"
	"nex/internal/system"
)

// Config configures the application.
type Config struct {
	Title        string
	Width        int
	Height       int
	Debug        bool
	AppVersion   string
	AppBuild     string
	AppUpdated   string
	AppAuthor    string
	Icon         string        // app icon source used by dev/build scripts
	Dist         embed.FS      // React build, embedded in binary
	DistDir      string        // subdirectory inside Dist (default "frontend/dist")
	DevServerURL string        // e.g. http://127.0.0.1:5179 for Vite HMR
	IdleTimeout  time.Duration // session idle timeout (default 12h)
	EnvFile      string        // default ".env"
	PublicPrefix string        // default "VITE_"

	SingleInstanceID string
	OnStartup        func(*App)
	OnDomReady       func(*App)
	OnShutdown       func(*App)
	OnBeforeClose    func(*App) bool // return true to prevent app.quit
	OnSecondInstance func(*App, []string)

	// SecurityPolicy and focused hooks are optional. Nil means allow, preserving
	// the framework's trusted-frontend default.
	SecurityPolicy     core.SecurityPolicy
	OnSecurityDecision func(*core.Context, core.SecurityDecision) error
	OnShellCommand     func(*core.Context, core.SecurityDecision) error
	OnFileAccess       func(*core.Context, core.SecurityDecision) error
	OnWindowEval       func(*core.Context, core.SecurityDecision) error
	OnAppExit          func(*core.Context, core.SecurityDecision) error
	OnOpenURL          func(*core.Context, core.SecurityDecision) error
}

// App is a running nex application.
type App struct {
	cfg       Config
	wv        webview.WebView
	sessions  *session.Manager
	session   *session.Session
	handlers  map[string]core.HandlerFunc
	env       *config.Env
	single    *singleInstance
	winMu     sync.RWMutex
	winTitle  string
	winWidth  int
	winHeight int
	winHint   string
}

// New creates the application, loads .env, and registers all sys.* handlers.
func New(cfg Config) *App {
	if cfg.Width == 0 {
		cfg.Width = 1024
	}
	if cfg.Height == 0 {
		cfg.Height = 768
	}
	if cfg.Title == "" {
		cfg.Title = "nex"
	}
	if cfg.AppVersion == "" {
		cfg.AppVersion = "dev"
	}
	if cfg.AppBuild == "" {
		cfg.AppBuild = "dev"
	}
	if cfg.AppUpdated == "" {
		cfg.AppUpdated = "unknown"
	}
	if cfg.DistDir == "" {
		cfg.DistDir = "frontend/dist"
	}
	if cfg.EnvFile == "" {
		cfg.EnvFile = ".env"
	}
	if cfg.PublicPrefix == "" {
		cfg.PublicPrefix = config.DefaultPublicPrefix
	}
	a := &App{
		cfg:       cfg,
		sessions:  session.NewManager(cfg.IdleTimeout),
		handlers:  map[string]core.HandlerFunc{},
		env:       config.Load(cfg.EnvFile),
		winTitle:  cfg.Title,
		winWidth:  cfg.Width,
		winHeight: cfg.Height,
		winHint:   "none",
	}
	system.Register(a)
	a.Register("sys.app.info", a.sysAppInfo)
	a.Register("sys.framework.info", a.sysFrameworkInfo)
	a.Register("sys.framework.stack", a.sysFrameworkStack)
	return a
}

// Register implements core.Registrar (internal use, no namespace guard).
func (a *App) Register(method string, fn core.HandlerFunc) {
	a.handlers[method] = fn
}

// Handle registers an application handler. The "sys." namespace is reserved.
func (a *App) Handle(method string, fn core.HandlerFunc) {
	if strings.HasPrefix(method, "sys.") {
		panic("nex: 'sys.' namespace is reserved")
	}
	a.handlers[method] = fn
}

// Env returns the loaded environment (for reading backend-only variables).
func (a *App) Env() *config.Env { return a.env }

// Emit sends an event to the frontend.
func (a *App) Emit(event string, payload any) {
	if a.wv == nil {
		return
	}
	js := fmt.Sprintf("window.__nexEmit&&window.__nexEmit(%q,%s);",
		event, mustJSON(payload))
	a.wv.Dispatch(func() { a.wv.Eval(js) })
}

// Eval executes JavaScript in the webview on the main thread.
func (a *App) Eval(js string) {
	if a.wv == nil {
		return
	}
	a.wv.Dispatch(func() { a.wv.Eval(js) })
}

// --- core.Host ---------------------------------------------------------------

func (a *App) Authorize(c *core.Context, d core.SecurityDecision) error {
	if d.Method == "" && c != nil {
		d.Method = c.Method
	}
	if a.cfg.SecurityPolicy != nil {
		if err := a.cfg.SecurityPolicy.Allow(c, d); err != nil {
			return err
		}
	}
	if a.cfg.OnSecurityDecision != nil {
		if err := a.cfg.OnSecurityDecision(c, d); err != nil {
			return err
		}
	}
	switch d.Category {
	case "shell":
		if d.Operation == "openURL" && a.cfg.OnOpenURL != nil {
			return a.cfg.OnOpenURL(c, d)
		}
		if a.cfg.OnShellCommand != nil {
			return a.cfg.OnShellCommand(c, d)
		}
	case "filesystem":
		if a.cfg.OnFileAccess != nil {
			return a.cfg.OnFileAccess(c, d)
		}
	case "window":
		if d.Operation == "eval" && a.cfg.OnWindowEval != nil {
			return a.cfg.OnWindowEval(c, d)
		}
	case "app":
		if (d.Operation == "exit" || d.Operation == "restart" || d.Operation == "quit") && a.cfg.OnAppExit != nil {
			return a.cfg.OnAppExit(c, d)
		}
	}
	// Backend-owned safety net: if the app hasn't set a SecurityPolicy nor a
	// focused hook for this exact category/operation, a privileged decision
	// (arbitrary shell exec, filesystem access, process lifecycle,
	// window.eval, password prompt, native clipboard) still requires an
	// explicit yes/no from the user — shown here via a native zenity dialog
	// dispatched on the webview's main thread, never delegated to frontend
	// JS (which a compromised/malicious page content could just skip by
	// calling call() directly).
	if a.cfg.SecurityPolicy == nil && isPrivilegedDecision(d) {
		return a.confirmPrivileged(d)
	}
	return nil
}

// privilegedDecisions mirrors frontend/src/lib/nex.js's privilegedApiIds
// classification, category/operation pairs that always require confirmation
// unless the app has taken over via SecurityPolicy or a focused hook.
var privilegedDecisions = map[string]map[string]bool{
	"app":        {"restart": true, "exit": true, "quit": true},
	"shell":      {"exec": true, "run": true, "start": true},
	"filesystem": {"read": true, "write": true, "list": true, "exists": true, "stat": true, "mkdir": true, "remove": true, "rename": true, "trashEmpty": true},
	"window":     {"eval": true},
	"dialog":     {"password": true},
	"clipboard":  {"readText": true, "writeText": true, "clear": true},
}

func isPrivilegedDecision(d core.SecurityDecision) bool {
	ops, ok := privilegedDecisions[d.Category]
	return ok && ops[d.Operation]
}

// confirmPrivileged shows a native (zenity) yes/no dialog on the main thread
// and blocks the RPC call until the user answers. Denying returns a
// "forbidden" RPC error to the caller.
func (a *App) confirmPrivileged(d core.SecurityDecision) error {
	method := d.Method
	if method == "" {
		method = d.Category + "." + d.Operation
	}
	message := fmt.Sprintf("%s is a privileged native API.\n\nAllow this app to run it?", method)

	var dialogErr error
	confirmed := false
	a.OnMain(func() {
		err := zenity.Question(message,
			zenity.Title("nex — privileged API"),
			zenity.OKLabel("Allow"),
			zenity.CancelLabel("Deny"),
		)
		switch err {
		case nil:
			confirmed = true
		case zenity.ErrCanceled:
			confirmed = false
		default:
			dialogErr = err
		}
	})
	if dialogErr != nil {
		return core.Errorf("dialog", "%v", dialogErr)
	}
	if !confirmed {
		return core.Errorf("forbidden", "denied by user: %s", method)
	}
	return nil
}

func (a *App) OnMain(fn func()) {
	if a.wv == nil {
		fn()
		return
	}
	done := make(chan struct{})
	a.wv.Dispatch(func() { defer close(done); fn() })
	<-done
}

func (a *App) SetTitle(title string) {
	a.winMu.Lock()
	a.winTitle = title
	a.winMu.Unlock()
	if a.wv != nil {
		a.OnMain(func() { a.wv.SetTitle(title) })
	}
}

func (a *App) SetSize(w, h int, hint string) {
	a.winMu.Lock()
	a.winWidth = w
	a.winHeight = h
	a.winHint = hint
	a.winMu.Unlock()
	if a.wv != nil {
		a.OnMain(func() { a.wv.SetSize(w, h, hintFrom(hint)) })
	}
}

func (a *App) WindowInfo() map[string]any {
	a.winMu.RLock()
	defer a.winMu.RUnlock()
	return map[string]any{
		"title":  a.winTitle,
		"width":  a.winWidth,
		"height": a.winHeight,
		"hint":   a.winHint,
	}
}

func (a *App) Quit() {
	if a.cfg.OnBeforeClose != nil && a.cfg.OnBeforeClose(a) {
		return
	}
	if a.wv != nil {
		a.wv.Dispatch(func() { a.wv.Terminate() })
	}
}

// ReleaseSingleInstance is safe to call more than once: stopSingleInstance
// already guards against a nil a.single and ignores close/remove errors.
func (a *App) ReleaseSingleInstance() {
	a.stopSingleInstance()
}

// --- sys.app.info / sys.framework.info ---------------------------------------

func (a *App) sysFrameworkInfo(_ *core.Context, _ json.RawMessage) (any, error) {
	return meta.Info(), nil
}

func (a *App) sysFrameworkStack(_ *core.Context, _ json.RawMessage) (any, error) {
	return meta.Stack(), nil
}

func (a *App) sysAppInfo(_ *core.Context, _ json.RawMessage) (any, error) {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	cacheDir, _ := os.UserCacheDir()
	dev := os.Getenv("APP_DEV") == "1"
	appInfo := map[string]any{
		"name":    a.cfg.Title,
		"title":   a.cfg.Title,
		"version": a.cfg.AppVersion,
		"build":   a.cfg.AppBuild,
		"updated": a.cfg.AppUpdated,
		"author":  a.cfg.AppAuthor,
		"icon":    a.cfg.Icon,
	}
	return map[string]any{
		"name":    a.cfg.Title,
		"title":   a.cfg.Title,
		"version": a.cfg.AppVersion,
		"build":   a.cfg.AppBuild,
		"updated": a.cfg.AppUpdated,
		"author":  a.cfg.AppAuthor,
		"icon":    a.cfg.Icon,
		"commit":  a.cfg.AppBuild,
		"date":    a.cfg.AppUpdated,
		"app":     appInfo,
		"public":  a.env.Public(a.cfg.PublicPrefix),
		"runtime": map[string]any{
			"dev":        dev,
			"packaged":   !dev,
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"pid":        os.Getpid(),
			"executable": exe,
			"cwd":        cwd,
		},
		"paths": map[string]any{
			"home":       home,
			"config":     configDir,
			"cache":      cacheDir,
			"temp":       os.TempDir(),
			"executable": exe,
			"cwd":        cwd,
		},
	}, nil
}

// --- Run ---------------------------------------------------------------------

// Run starts the local HTTP server and opens the native webview.
// Blocks until the window is closed. Must be called from main goroutine.
func (a *App) Run() error {
	runtime.LockOSThread()

	if err := a.startSingleInstance(); err != nil {
		if err == errSecondInstance {
			return nil
		}
		return err
	}
	defer a.stopSingleInstance()

	sess, err := a.sessions.Create()
	if err != nil {
		return err
	}
	a.session = sess

	dist, err := fs.Sub(a.cfg.Dist, a.cfg.DistDir)
	if err != nil {
		return fmt.Errorf("invalid dist dir %q: %w", a.cfg.DistDir, err)
	}

	dev := os.Getenv("APP_DEV") == "1" && a.cfg.DevServerURL != ""

	addr := "127.0.0.1:0"
	if dev {
		addr = "127.0.0.1:34115"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	baseURL := "http://" + ln.Addr().String()

	mux := http.NewServeMux()
	mux.Handle("/api/rpc", a.secure(baseURL, http.HandlerFunc(a.handleRPC)))
	mux.Handle("/api/session", a.secure(baseURL, http.HandlerFunc(a.handleSession)))
	mux.Handle("/api/lifecycle/dom-ready", a.secure(baseURL, http.HandlerFunc(a.handleDomReady)))
	mux.Handle("/", spaHandler(dist))

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("nex: server: %v", err)
		}
	}()

	a.wv = webview.New(a.cfg.Debug)
	defer a.wv.Destroy()
	ensureEditMenu()
	ensureJSDialogSupport()
	a.wv.SetTitle(a.cfg.Title)
	a.wv.SetSize(a.cfg.Width, a.cfg.Height, webview.HintNone)
	a.wv.Init(bootstrapJS(sess.Token, baseURL))
	if a.cfg.OnStartup != nil {
		a.cfg.OnStartup(a)
	}

	if dev {
		a.wv.Navigate(a.cfg.DevServerURL)
	} else {
		a.wv.Navigate(baseURL)
	}
	a.wv.Run()

	if a.cfg.OnShutdown != nil {
		a.cfg.OnShutdown(a)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	return nil
}

// --- Security middleware -----------------------------------------------------

func (a *App) secure(baseURL string, next http.Handler) http.Handler {
	allowed := map[string]bool{baseURL: true}
	if a.cfg.DevServerURL != "" {
		allowed[strings.TrimRight(a.cfg.DevServerURL, "/")] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if !allowed[o] {
				http.Error(w, "bad origin", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-nex-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if _, ok := a.sessions.Validate(r.Header.Get("X-nex-Token")); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Handlers ----------------------------------------------------------------

func (a *App) handleSession(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        a.session.ID,
		"expiresAt": a.session.ExpiresAt.Format(time.RFC3339),
	})
}

func (a *App) handleDomReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "use POST"})
		return
	}
	if a.cfg.OnDomReady != nil {
		go a.cfg.OnDomReady(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRPCErr(w, "method_not_allowed", "use POST")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		writeRPCErr(w, "bad_request", "Content-Type must be application/json")
		return
	}
	var req struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCErr(w, "bad_request", "invalid JSON")
		return
	}
	fn, ok := a.handlers[req.Method]
	if !ok {
		writeRPCErr(w, "not_found", "unknown method: "+req.Method)
		return
	}
	sess, _ := a.sessions.Validate(r.Header.Get("X-nex-Token"))
	c := &core.Context{Ctx: r.Context(), Host: a, Session: sess, Request: r, Method: req.Method}
	result, err := fn(c, req.Params)
	if err != nil {
		if rpcErr, ok2 := err.(*core.RPCError); ok2 {
			writeJSON(w, http.StatusOK, map[string]any{"error": rpcErr})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"error": &core.RPCError{Code: "internal", Message: err.Error()},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

// --- Helpers -----------------------------------------------------------------

func spaHandler(dist fs.FS) http.Handler {
	fs_ := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(dist, p); err != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			http.ServeFileFS(w, r2, dist, "index.html")
			return
		}
		fs_.ServeHTTP(w, r)
	})
}

func bootstrapJS(token, base string) string {
	return fmt.Sprintf(`(function(){
  window.__NEX__={token:%q,base:%q};
  window.__nexEmit=function(n,p){
    window.dispatchEvent(new CustomEvent("nex:"+n,{detail:p}));
  };
  window.__nexDomReady=function(){
    fetch(window.__NEX__.base+"/api/lifecycle/dom-ready",{
      method:"POST",
      headers:{"X-nex-Token":window.__NEX__.token}
    }).catch(function(){});
  };
  if(document.readyState==="loading"){
    document.addEventListener("DOMContentLoaded",window.__nexDomReady,{once:true});
  }else{
    setTimeout(window.__nexDomReady,0);
  }
})();`, token, base)
}

func hintFrom(s string) webview.Hint {
	switch s {
	case "min":
		return webview.HintMin
	case "max":
		return webview.HintMax
	case "fixed":
		return webview.HintFixed
	default:
		return webview.HintNone
	}
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRPCErr(w http.ResponseWriter, code, msg string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"error": &core.RPCError{Code: code, Message: msg},
	})
}
