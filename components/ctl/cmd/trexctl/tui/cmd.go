package tui

import (
	pkgcmd "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tuicmd"

	"github.com/openshift-online/rh-trex-ai/components/ctl/data/generated/tui"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/connection"
)

// Cmd browses and operates the service through its OpenAPI-derived terminal
// UI. It reuses the TRex library's Bubble Tea runtime with this project's
// embedded descriptor, and the session saved by 'login'.
var Cmd = pkgcmd.NewTUICommand(tui.GetDescriptor, pkgcmd.WithSession(loadSession))

// loadSession reuses the saved login: its URL, its --insecure setting and its
// token, which is refreshed during the session like in the other commands.
// Without a saved login it returns an empty session, so --server and
// --token-file still work.
func loadSession() (pkgcmd.Session, error) {
	cfg, err := config.Load()
	if err != nil {
		return pkgcmd.Session{}, err
	}
	session := pkgcmd.Session{Server: cfg.URL, Insecure: cfg.Insecure}
	if cfg.AccessToken != "" {
		session.TokenProvider = connection.NewConfigTokenProvider(cfg)
	}
	return session, nil
}
