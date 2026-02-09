package environments

import (
	"github.com/openshift-online/rh-trex/pkg/config"
	"github.com/openshift-online/rh-trex/pkg/db/db_session"
)

type DevEnvImpl struct {
	Env *Env
}

var _ EnvironmentImpl = &DevEnvImpl{}

func (e *DevEnvImpl) OverrideDatabase(c *Database) error {
	c.SessionFactory = db_session.NewProdFactory(e.Env.Config.Database)
	return nil
}

func (e *DevEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	c.Server.EnableJWT = false
	c.Server.EnableHTTPS = false
	return nil
}

func (e *DevEnvImpl) OverrideServices(s *Services) error {
	return nil
}

func (e *DevEnvImpl) OverrideHandlers(h *Handlers) error {
	return nil
}

func (e *DevEnvImpl) OverrideClients(c *Clients) error {
	return nil
}

func (e *DevEnvImpl) Flags() map[string]string {
	return map[string]string{
		"v":                      "10",
		"enable-authz":           "false",
		"ocm-debug":              "false",
		"enable-ocm-mock":        "true",
		"enable-https":           "false",
		"enable-metrics-https":   "false",
		"api-server-hostname":    "localhost",
		"api-server-bindaddress": "localhost:8000",
		"enable-sentry":          "false",
	}
}
