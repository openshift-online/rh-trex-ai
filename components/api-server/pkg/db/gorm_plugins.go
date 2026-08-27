package db

import (
	"github.com/golang/glog"
	"gorm.io/gorm"
)

// gormPlugins holds GORM plugins that cross-cutting concerns (such as
// OpenTelemetry instrumentation) register before a SessionFactory builds its
// base *gorm.DB. It mirrors the pre-auth middleware and interceptor registries
// in pkg/server: a plugin registers itself from an init() and the SessionFactory
// applies the whole set once, when it opens the base connection.
//
// Registration must happen on the base *gorm.DB, not on the per-request session
// returned by New(ctx): a GORM plugin installs callbacks, so applying it per
// request would append duplicate callbacks on every call. Applying the registry
// when the base connection is opened installs each plugin exactly once, and the
// per-request sessions cloned from that base inherit the callbacks together with
// the request context.
var gormPlugins []gorm.Plugin

// RegisterGormPlugin registers a GORM plugin to be applied to the base *gorm.DB
// of every SessionFactory when it is initialized. It is meant to be called from
// a package init(), before any SessionFactory is constructed; registering a
// plugin has no effect on a SessionFactory that has already been initialized.
func RegisterGormPlugin(plugin gorm.Plugin) {
	gormPlugins = append(gormPlugins, plugin)
}

// ApplyGormPlugins installs every registered plugin on the given base *gorm.DB.
// SessionFactory implementations call it once, on the base connection they build
// New(ctx) sessions from. A plugin failure is logged and skipped rather than
// fatal: instrumentation must never prevent the server from connecting to its
// database.
func ApplyGormPlugins(g2 *gorm.DB) {
	for _, plugin := range gormPlugins {
		if err := g2.Use(plugin); err != nil {
			glog.Errorf("failed to apply GORM plugin %q, continuing without it: %v", plugin.Name(), err)
		}
	}
}
