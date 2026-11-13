package environments

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_session"
	pkgenv "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
)

var _ pkgenv.EnvironmentImpl = &ProductionEnvImpl{}

type ProductionEnvImpl struct {
	Env *pkgenv.Env
}

func (e *ProductionEnvImpl) OverrideDatabase(c *pkgenv.Database) error {
	c.SessionFactory = db_session.NewProdFactory(e.Env.Config.Database)
	return nil
}

func (e *ProductionEnvImpl) OverrideConfig(c *config.ApplicationConfig) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideServices(s *pkgenv.Services) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideHandlers(h *pkgenv.Handlers) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideClients(c *pkgenv.Clients) error {
	return nil
}

func (e *ProductionEnvImpl) OverrideAuthorizer(env *pkgenv.Env) (authz.Authorizer, error) {
	return authz.FromFlag(env.Config.Auth.EnableAuthz), nil
}

// OverrideAudit is optional; this spells out the default (JSON lines to --audit-sink).
func (e *ProductionEnvImpl) OverrideAudit(env *pkgenv.Env) (audit.Recorder, error) {
	return audit.DefaultRecorder(env.Config.Audit)
}

func (e *ProductionEnvImpl) Flags() map[string]string {
	return map[string]string{
		"log-level": "info",
	}
}
