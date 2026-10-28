package app

import (
	"testing"

	"nex/internal/core"
)

func TestSetupUpdaterSourceURL(t *testing.T) {
	const envURL = "https://github.com/owner/env-repo"
	const cfgURL = "https://updates.example.com/feed.json"

	cases := []struct {
		name           string
		env            string
		cfg            string
		wantConfigured bool
		wantSource     string
	}{
		{"env only", envURL, "", true, "github"},
		{"cfg only", "", cfgURL, true, "feed"},
		{"env wins over cfg", envURL, cfgURL, true, "github"},
		{"neither", "", "", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NEX_UPDATE_URL", tc.env)
			t.Setenv("NEX_UPDATE_ASSET", "")
			a := &App{
				cfg:      Config{SingleInstanceID: "dev.vlt.nex-updater-test", UpdateURL: tc.cfg},
				handlers: map[string]core.HandlerFunc{},
			}
			a.setupUpdater()
			state := a.upd.State()
			if state.Configured != tc.wantConfigured {
				t.Fatalf("Configured = %v, want %v", state.Configured, tc.wantConfigured)
			}
			if state.Source != tc.wantSource {
				t.Fatalf("Source = %q, want %q", state.Source, tc.wantSource)
			}
		})
	}
}
