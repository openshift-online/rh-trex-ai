package environments

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_session"
	pkgenv "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
)

type DevEnvImpl struct {
	Env *pkgenv.Env
}

var _ pkgenv.EnvironmentImpl = &DevEnvImpl{}

func (e *DevEnvImpl) OverrideDatabase(c *pkgenv.Database) error {
	c.SessionFactory = db_session.NewProdFactory(e.Env.Config.Database)
	return nil
}

func (e *DevEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	c.Auth.EnableJWT = false
	c.Server.EnableHTTPS = false
	return nil
}

func (e *DevEnvImpl) OverrideServices(s *pkgenv.Services) error {
	return nil
}

func (e *DevEnvImpl) OverrideHandlers(h *pkgenv.Handlers) error {
	return nil
}

func (e *DevEnvImpl) OverrideClients(c *pkgenv.Clients) error {
	return nil
}

func (e *DevEnvImpl) OverrideAuthorizer(env *pkgenv.Env) (authz.Authorizer, error) {
	return authz.FromFlag(env.Config.Auth.EnableAuthz), nil
}

// OverrideAudit is optional; this spells out the default (JSON lines to --audit-sink).
func (e *DevEnvImpl) OverrideAudit(env *pkgenv.Env) (audit.Recorder, error) {
	return audit.DefaultRecorder(env.Config.Audit)
}

func (e *DevEnvImpl) Flags() map[string]string {
	return map[string]string{
		"log-level":              "debug",
		"enable-authz":           "false",
		"enable-https":           "false",
		"enable-metrics-https":   "false",
		"api-server-hostname":    "localhost",
		"api-server-bindaddress": "localhost:8000",
	}
}
