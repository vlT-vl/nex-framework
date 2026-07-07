// Package nex is the public facade of the nex desktop framework.
// Applications import only this package.
package nex

import (
	"nex/config"
	"nex/internal/app"
	"nex/internal/core"
	"nex/internal/meta"
)

// Public type aliases.
type (
	App         = app.App
	Config      = app.Config
	Context     = core.Context
	HandlerFunc = core.HandlerFunc
	RPCError    = core.RPCError
	Env         = config.Env

	SecurityDecision = core.SecurityDecision
	SecurityPolicy   = core.SecurityPolicy
)

// Errorf creates a typed RPC error returned to the frontend.
var Errorf = core.Errorf

// New creates a new application.
func New(cfg Config) *App { return app.New(cfg) }

// Framework identity — single source of truth is internal/meta.
// Override at build time: -ldflags "-X nex/internal/meta.Version=x.y.z -X nex/internal/meta.Build=R..."
const Name = meta.Name

func FrameworkVersion() string { return meta.Version }
func FrameworkBuild() string   { return meta.Build }
func FrameworkUpdated() string { return meta.Updated }
func FrameworkAuthor() string  { return meta.Author }

var (
	// Deprecated: use FrameworkVersion.
	Version = meta.Version
	// Deprecated: use FrameworkBuild.
	Build = meta.Build
	// Deprecated: use FrameworkUpdated.
	Updated = meta.Updated
	// Deprecated: use FrameworkAuthor.
	Author = meta.Author
	// Deprecated: use FrameworkBuild.
	Commit = meta.Build
	// Deprecated: use FrameworkUpdated.
	Date = meta.Updated
)

func FrameworkInfo() map[string]any { return meta.Info() }
