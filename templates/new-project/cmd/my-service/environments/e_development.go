package environments

import (
	"github.com/openshift-online/rh-trex-ai/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"github.com/openshift-online/rh-trex-ai/pkg/db/db_session"
	pkgenv "github.com/openshift-online/rh-trex-ai/pkg/environments"
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
