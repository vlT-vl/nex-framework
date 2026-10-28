package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"nex/internal/core"
	"nex/internal/updater"
)

type updaterHost struct {
	app *App
}

func (h *updaterHost) AppID() string {
	return h.app.updaterAppID()
}

func (h *updaterHost) Version() (string, string) {
	return h.app.cfg.AppVersion, h.app.cfg.AppBuild
}

func (h *updaterHost) StateDir() string {
	return updaterStateDir(h.app.updaterAppID())
}

func (h *updaterHost) Authorize(op string) error {
	c := &core.Context{Ctx: context.Background(), Host: h.app, Session: h.app.session, Method: "sys.updater." + op}
	return c.Authorize(core.SecurityDecision{Category: "app", Operation: "update." + op})
}

func (h *updaterHost) Emit(event string, payload any) {
	h.app.Emit(event, payload)
}

func (h *updaterHost) Quiesce() error {
	h.app.ReleaseSingleInstance()
	if !h.app.tryQuit() {
		if err := h.app.startSingleInstance(); err != nil && err != errSecondInstance {
			log.Printf("nex: updater: single-instance lock not restored: %v", err)
		}
		return errors.New("application close was vetoed")
	}
	return nil
}

func (h *updaterHost) RelaunchEnv() []string {
	return os.Environ()
}

func (h *updaterHost) HealthMode() updater.HealthMode {
	return updater.HealthFrontend
}

func (h *updaterHost) InstallMode() updater.InstallMode {
	if os.Getenv("APP_DEV") == "1" {
		return updater.InstallNotify
	}
	return updater.InstallSelf
}

func (a *App) updaterAppID() string {
	if a.cfg.SingleInstanceID != "" {
		return a.cfg.SingleInstanceID
	}
	if id := os.Getenv("NEX_APP_ID"); id != "" {
		return id
	}
	return "nex-app"
}

func updaterStateDir(appID string) string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, appID)
}

func (a *App) setupUpdater() {
	sourceURL := os.Getenv("NEX_UPDATE_URL")
	if sourceURL == "" {
		sourceURL = a.cfg.UpdateURL
	}
	a.upd = updater.New(updater.Config{
		SourceURL:    sourceURL,
		AssetPattern: os.Getenv("NEX_UPDATE_ASSET"),
		AllowedHosts: a.cfg.UpdateAllowedHosts,
		PublicKeys:   a.cfg.UpdatePublicKeys,
	}, &updaterHost{app: a})

	a.Register("sys.updater.getState", a.updaterGetState)
	a.Register("sys.updater.check", a.updaterCheck)
	a.Register("sys.updater.download", a.updaterDownload)
	a.Register("sys.updater.install", a.updaterInstall)
	a.Register("sys.updater.confirm", a.updaterConfirm)
	a.Register("sys.updater.getLastResult", a.updaterLastResult)
	a.Register("sys.updater.setSettings", a.updaterSetSettings)
}

func runUpdaterBootstrap(appID string) {
	if updater.RunHelperIfRequested() {
		os.Exit(0)
	}
	if err := updater.Recover(updaterStateDir(appID)); err != nil {
		log.Printf("nex: updater recover: %v", err)
	}
}

func (a *App) startAutoUpdate() {
	a.updOnce.Do(func() {
		go func() {
			state := a.upd.State()
			if !state.Configured || !state.Settings.AutoCheck {
				return
			}
			ctx := context.Background()
			state, err := a.upd.Check(ctx, false)
			if err != nil {
				log.Printf("nex: updater check: %v", err)
				return
			}
			if state.Status == "available" && state.Settings.AutoDownload && state.Update != nil && state.Update.Installable && !state.Update.RolledBack {
				if err := a.upd.Download(ctx); err != nil {
					log.Printf("nex: updater download: %v", err)
				}
			}
		}()
	})
}

func updaterError(err error) error {
	if err == nil {
		return nil
	}
	if code := updater.ErrorCode(err); code != "" {
		return core.Errorf(code, "%v", err)
	}
	return core.Errorf("updater", "%v", err)
}

func (a *App) updaterGetState(_ *core.Context, _ json.RawMessage) (any, error) {
	return a.upd.State(), nil
}

func (a *App) updaterCheck(c *core.Context, params json.RawMessage) (any, error) {
	var p struct {
		Manual bool   `json:"manual"`
		URL    string `json:"url"`
	}
	if len(params) > 0 {
		if err := c.Bind(params, &p); err != nil {
			return nil, core.Errorf("bad_request", "%v", err)
		}
	}
	a.upd.SetSourceIfUnset(p.URL)
	state := a.upd.State()
	if !state.Configured {
		return updaterLegacyPayload(state), nil
	}
	state, err := a.upd.Check(c.Ctx, p.Manual)
	if err != nil {
		return nil, updaterError(err)
	}
	return updaterLegacyPayload(state), nil
}

func updaterLegacyPayload(state updater.State) map[string]any {
	out := map[string]any{
		"configured":      state.Configured,
		"source":          state.Source,
		"currentVersion":  state.CurrentVersion,
		"currentBuild":    state.CurrentBuild,
		"status":          state.Status,
		"update":          state.Update,
		"progress":        state.Progress,
		"settings":        state.Settings,
		"lastCheck":       state.LastCheck,
		"error":           state.Error,
		"current":         state.CurrentVersion,
		"updateAvailable": state.Update != nil,
	}
	if state.Update != nil {
		out["latest"] = state.Update.Version
		out["notes"] = state.Update.Notes
		out["mandatory"] = state.Update.Mandatory
	}
	return out
}

func (a *App) updaterDownload(c *core.Context, _ json.RawMessage) (any, error) {
	if err := a.upd.Download(c.Ctx); err != nil {
		return nil, updaterError(err)
	}
	return a.upd.State(), nil
}

func (a *App) updaterInstall(c *core.Context, _ json.RawMessage) (any, error) {
	if err := a.upd.Install(c.Ctx); err != nil {
		return nil, updaterError(err)
	}
	return map[string]any{"status": "installing"}, nil
}

func (a *App) updaterConfirm(_ *core.Context, _ json.RawMessage) (any, error) {
	confirmed, err := a.upd.Confirm()
	if err != nil {
		return nil, updaterError(err)
	}
	return map[string]any{"confirmed": confirmed}, nil
}

func (a *App) updaterLastResult(_ *core.Context, _ json.RawMessage) (any, error) {
	result, err := a.upd.LastResult()
	if err != nil {
		return nil, updaterError(err)
	}
	return result, nil
}

func (a *App) updaterSetSettings(c *core.Context, params json.RawMessage) (any, error) {
	var settings updater.Settings
	if err := c.Bind(params, &settings); err != nil {
		return nil, core.Errorf("bad_request", "%v", err)
	}
	if err := a.upd.SetSettings(settings); err != nil {
		return nil, updaterError(err)
	}
	return a.upd.State(), nil
}
