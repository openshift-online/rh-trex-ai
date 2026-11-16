package environments

import (
	"testing"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

// plainEnv is an environment written before audit existed.
type plainEnv struct{}

func (plainEnv) Flags() map[string]string                          { return nil }
func (plainEnv) OverrideConfig(*config.ApplicationConfig) error    { return nil }
func (plainEnv) OverrideServices(*Services) error                  { return nil }
func (plainEnv) OverrideDatabase(*Database) error                  { return nil }
func (plainEnv) OverrideHandlers(*Handlers) error                  { return nil }
func (plainEnv) OverrideClients(*Clients) error                    { return nil }
func (plainEnv) OverrideAuthorizer(*Env) (authz.Authorizer, error) { return authz.AllowAll(), nil }

type auditEnv struct {
	plainEnv
	recorder audit.Recorder
}

func (a auditEnv) OverrideAudit(*Env) (audit.Recorder, error) { return a.recorder, nil }

func TestAuditRecorderIsOptional(t *testing.T) {
	env := &Env{Name: "test", Config: config.NewApplicationConfig()}

	rec, err := auditRecorder(plainEnv{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.(*audit.JSONRecorder); !ok {
		t.Fatalf("an environment without OverrideAudit must get the default recorder, got %T", rec)
	}

	mem := audit.NewMemoryRecorder()
	rec, err = auditRecorder(auditEnv{recorder: mem}, env)
	if err != nil || rec != mem {
		t.Fatalf("OverrideAudit must choose the recorder, got %T, %v", rec, err)
	}

	if _, err := auditRecorder(auditEnv{recorder: nil}, env); err == nil {
		t.Fatal("a nil recorder from OverrideAudit must be an error")
	}
}
