package nex

import (
	"nex/config"
	"nex/internal/app"
	"nex/internal/core"
	"nex/internal/meta"
)

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

var Errorf = core.Errorf

func New(cfg Config) *App { return app.New(cfg) }

const Name = meta.Name

func FrameworkVersion() string { return meta.Version }
func FrameworkBuild() string   { return meta.Build }
func FrameworkUpdated() string { return meta.Updated }
func FrameworkAuthor() string  { return meta.Author }

var (
	Version = meta.Version

	Build = meta.Build

	Updated = meta.Updated

	Author = meta.Author

	Commit = meta.Build

	Date = meta.Updated
)

func FrameworkInfo() map[string]any { return meta.Info() }
