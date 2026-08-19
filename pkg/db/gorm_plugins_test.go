package db

import (
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// fakeGormPlugin records whether Initialize was invoked so a test can assert the
// registry applied it to a *gorm.DB.
type fakeGormPlugin struct {
	name        string
	initialized bool
	err         error
}

func (p *fakeGormPlugin) Name() string { return p.name }

func (p *fakeGormPlugin) Initialize(*gorm.DB) error {
	p.initialized = true
	return p.err
}

// withCleanRegistry saves and restores the package-level registry so a test can
// register plugins without leaking into other tests.
func withCleanRegistry(t *testing.T) {
	t.Helper()
	saved := gormPlugins
	gormPlugins = nil
	t.Cleanup(func() { gormPlugins = saved })
}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	g2, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{})
	if err != nil {
		t.Fatalf("opening dummy gorm.DB: %v", err)
	}
	return g2
}

func TestApplyGormPluginsInstallsRegisteredPlugins(t *testing.T) {
	withCleanRegistry(t)

	first := &fakeGormPlugin{name: "first"}
	second := &fakeGormPlugin{name: "second"}
	RegisterGormPlugin(first)
	RegisterGormPlugin(second)

	g2 := newTestDB(t)
	ApplyGormPlugins(g2)

	if !first.initialized || !second.initialized {
		t.Fatalf("expected both plugins initialized, got first=%v second=%v", first.initialized, second.initialized)
	}
	if _, ok := g2.Plugins["first"]; !ok {
		t.Errorf("plugin %q not installed on the *gorm.DB", "first")
	}
	if _, ok := g2.Plugins["second"]; !ok {
		t.Errorf("plugin %q not installed on the *gorm.DB", "second")
	}
}

func TestApplyGormPluginsNoRegistrationsIsNoOp(t *testing.T) {
	withCleanRegistry(t)

	g2 := newTestDB(t)
	ApplyGormPlugins(g2) // must not panic

	if len(g2.Plugins) != 0 {
		t.Errorf("expected no plugins installed, got %d", len(g2.Plugins))
	}
}

// A plugin whose Initialize fails is logged and skipped; ApplyGormPlugins must
// not panic and must continue installing the remaining plugins.
func TestApplyGormPluginsSkipsFailingPlugin(t *testing.T) {
	withCleanRegistry(t)

	bad := &fakeGormPlugin{name: "bad", err: gorm.ErrInvalidValue}
	good := &fakeGormPlugin{name: "good"}
	RegisterGormPlugin(bad)
	RegisterGormPlugin(good)

	g2 := newTestDB(t)
	ApplyGormPlugins(g2)

	if _, ok := g2.Plugins["bad"]; ok {
		t.Errorf("failing plugin %q should not be recorded as installed", "bad")
	}
	if _, ok := g2.Plugins["good"]; !ok {
		t.Errorf("plugin %q after a failing one should still be installed", "good")
	}
}
