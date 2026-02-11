package environments

import (
	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"github.com/openshift-online/rh-trex-ai/pkg/db/db_session"
)

var _ EnvironmentImpl = &ProductionEnvImpl{}

type ProductionEnvImpl struct {
	Env *Env
}

func (e *ProductionEnvImpl) OverrideDatabase(c *Database) error {
	c.SessionFactory = db_session.NewProdFactory(e.Env.Config.Database)
	return nil
}

func (e *ProductionEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideServices(s *Services) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideHandlers(h *Handlers) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideClients(c *Clients) error {
	return nil
}

func (e *ProductionEnvImpl) Flags() map[string]string {
	return map[string]string{
		"v":               "1",
		"ocm-debug":       "false",
		"enable-ocm-mock": "false",
		"enable-sentry":   "true",
	}
}
