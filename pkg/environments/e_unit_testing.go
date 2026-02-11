package environments

import (
	"os"

	"github.com/openshift-online/rh-trex-ai/pkg/config"
	dbmocks "github.com/openshift-online/rh-trex-ai/pkg/db/mocks"
)

var _ EnvironmentImpl = &UnitTestingEnvImpl{}

type UnitTestingEnvImpl struct {
	Env *Env
}

func (e *UnitTestingEnvImpl) OverrideDatabase(c *Database) error {
	c.SessionFactory = dbmocks.NewMockSessionFactory()
	return nil
}

func (e *UnitTestingEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	if os.Getenv("DB_DEBUG") == "true" {
		c.Database.Debug = true
	}
	return nil
}

func (e *UnitTestingEnvImpl) OverrideServices(s *Services) error {
	return nil
}

func (e *UnitTestingEnvImpl) OverrideHandlers(h *Handlers) error {
	return nil
}

func (e *UnitTestingEnvImpl) OverrideClients(c *Clients) error {
	return nil
}

func (e *UnitTestingEnvImpl) Flags() map[string]string {
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
