package environments

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"os"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	dbmocks "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/mocks"
	pkgenv "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
)

var _ pkgenv.EnvironmentImpl = &UnitTestingEnvImpl{}

type UnitTestingEnvImpl struct {
	Env *pkgenv.Env
}

func (e *UnitTestingEnvImpl) OverrideDatabase(c *pkgenv.Database) error {
	c.SessionFactory = dbmocks.NewMockSessionFactory()
	return nil
}

func (e *UnitTestingEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	if os.Getenv("DB_DEBUG") == "true" {
		c.Database.Debug = true
	}
	return nil
}

func (e *UnitTestingEnvImpl) OverrideServices(s *pkgenv.Services) error {
	return nil
}

func (e *UnitTestingEnvImpl) OverrideHandlers(h *pkgenv.Handlers) error {
	return nil
}

func (e *UnitTestingEnvImpl) OverrideClients(c *pkgenv.Clients) error {
	return nil
}

func (e *UnitTestingEnvImpl) OverrideAuthorizer(env *pkgenv.Env) (authz.Authorizer, error) {
	return authz.FromFlag(env.Config.Auth.EnableAuthz), nil
}

// OverrideAudit keeps audit events in memory so tests can assert on them.
func (e *UnitTestingEnvImpl) OverrideAudit(env *pkgenv.Env) (audit.Recorder, error) {
	return audit.NewMemoryRecorder(), nil
}

func (e *UnitTestingEnvImpl) Flags() map[string]string {
	return map[string]string{
		"log-level":            "info",
		"enable-https":         "false",
		"enable-metrics-https": "false",
		"enable-authz":         "true",
	}
}
