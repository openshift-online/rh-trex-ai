package environments

import (
	"os"

	"github.com/openshift-online/rh-trex/pkg/config"
	"github.com/openshift-online/rh-trex/pkg/db/db_session"
)

var _ EnvironmentImpl = &IntegrationTestingEnvImpl{}

type IntegrationTestingEnvImpl struct {
	Env *Env
}

func (e *IntegrationTestingEnvImpl) OverrideDatabase(c *Database) error {
	mode := os.Getenv("DB_FACTORY_MODE")
	if mode == "external" {
		c.SessionFactory = db_session.NewTestFactory(e.Env.Config.Database)
	} else {
		c.SessionFactory = db_session.NewTestcontainerFactory(e.Env.Config.Database)
	}
	return nil
}

func (e *IntegrationTestingEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	if os.Getenv("DB_DEBUG") == "true" {
		c.Database.Debug = true
	}
	return nil
}

func (e *IntegrationTestingEnvImpl) OverrideServices(s *Services) error {
	return nil
}

func (e *IntegrationTestingEnvImpl) OverrideHandlers(h *Handlers) error {
	return nil
}

func (e *IntegrationTestingEnvImpl) OverrideClients(c *Clients) error {
	return nil
}

func (e *IntegrationTestingEnvImpl) Flags() map[string]string {
	return map[string]string{
		"v":                    "0",
		"logtostderr":          "true",
		"ocm-base-url":         "https://api.integration.openshift.com",
		"enable-https":         "false",
		"enable-metrics-https": "false",
		"enable-authz":         "true",
		"ocm-debug":            "false",
		"enable-ocm-mock":      "true",
		"enable-sentry":        "false",
	}
}
