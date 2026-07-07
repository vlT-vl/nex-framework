package main

import (
	"embed"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"nex/config"
	nexpkg "nex/nex"
)

//go:embed frontend/dist
var distFS embed.FS

func main() {
	envFile := envDefault("NEX_ENV_FILE", ".env")
	envValues := config.Read(envFile)
	config.Load(envFile)

	appName := envValue(envValues, "NEX_APP_NAME", "nex-app-template")
	appVersion := envValue(envValues, "NEX_APP_VERSION", AppVersion)
	appBuild := envValue(envValues, "NEX_APP_BUILD", AppBuild)
	appUpdated := envValue(envValues, "NEX_APP_UPDATED", AppUpdated)
	appAuthor := envValue(envValues, "NEX_APP_AUTHOR", AppAuthor)
	appIcon := envValue(envValues, "NEX_APP_ICON", "res/nexicon.svg")
	appID := envValue(envValues, "NEX_APP_ID", "dev.vlt.nex-app-template")

	log.Printf("%s %s (%s) starting on nex %s (%s); NEX_ENV=%s",
		appName, appVersion, appBuild, nexpkg.FrameworkVersion(), nexpkg.FrameworkBuild(), os.Getenv("NEX_ENV"))

	a := nexpkg.New(nexpkg.Config{
		Title:            appName,
		Width:            1100,
		Height:           760,
		Debug:            os.Getenv("APP_DEV") == "1",
		AppVersion:       appVersion,
		AppBuild:         appBuild,
		AppUpdated:       appUpdated,
		AppAuthor:        appAuthor,
		Icon:             appIcon,
		Dist:             distFS,
		DistDir:          "frontend/dist",
		DevServerURL:     envDefault("NEX_DEV_SERVER_URL", "http://127.0.0.1:5179"),
		IdleTimeout:      12 * time.Hour,
		EnvFile:          envFile,
		SingleInstanceID: appID,
	})

	a.Handle("greet", func(c *nexpkg.Context, params json.RawMessage) (any, error) {
		var p struct {
			Name string `json:"name"`
		}
		if err := c.Bind(params, &p); err != nil {
			return nil, nexpkg.Errorf("bad_request", "%v", err)
		}
		if p.Name == "" {
			p.Name = "world"
		}
		return map[string]any{"message": "Hello, " + p.Name + "!"}, nil
	})

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		n := 0
		for range t.C {
			n++
			a.Emit("tick", map[string]any{"n": n, "at": time.Now().Format(time.RFC3339)})
		}
	}()

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envValue(values map[string]string, key, fallback string) string {
	if v := strings.TrimSpace(values[key]); v != "" {
		return v
	}
	return envDefault(key, fallback)
}
