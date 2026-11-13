package environments

import (
	"net/http"
	"sync"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
)

const (
	UnitTestingEnv        string = "unit_testing"
	IntegrationTestingEnv string = "integration_testing"
	DevelopmentEnv        string = "development"
	ProductionEnv         string = "production"

	EnvironmentStringKey string = "API_ENV"
	EnvironmentDefault          = DevelopmentEnv
)

type Env struct {
	Name     string
	Services Services
	Handlers Handlers
	Clients  Clients
	Database Database
	Config   *config.ApplicationConfig
	// Authorizer is the policy for every REST route and gRPC method.
	// It is set by OverrideAuthorizer and is never nil after Initialize.
	Authorizer authz.Authorizer
	// Audit captures audit events and hands them to the environment's
	// recorder (SEC-004). It is set by Initialize and is never nil
	// afterwards; it may be disabled.
	Audit *audit.Service
}

type ApplicationConfig struct {
	ApplicationConfig *config.ApplicationConfig
}

type Database struct {
	SessionFactory db.SessionFactory
}

// JWTMiddleware interface for JWT authentication - avoiding import cycle
type JWTMiddleware interface {
	AuthenticateAccountJWT(next http.Handler) http.Handler
}

type Handlers struct {
	AuthMiddleware JWTMiddleware
}

type Services struct {
	serviceRegistry map[string]interface{}
	mutex           sync.RWMutex
}

func (s *Services) GetService(name string) interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.serviceRegistry == nil {
		return nil
	}
	return s.serviceRegistry[name]
}

func (s *Services) SetService(name string, service interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.serviceRegistry == nil {
		s.serviceRegistry = make(map[string]interface{})
	}
	s.serviceRegistry[name] = service
}

func (s *Services) InitRegistry() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.serviceRegistry = make(map[string]interface{})
}

// Clients holds the external clients a service needs. TRex ships none; an
// environment fills in its own fields from OverrideClients.
type Clients struct{}

type ConfigDefaults struct {
	Server   map[string]interface{}
	Metrics  map[string]interface{}
	Database map[string]interface{}
	Options  map[string]interface{}
}

type EnvironmentImpl interface {
	Flags() map[string]string
	OverrideConfig(c *config.ApplicationConfig) error
	OverrideServices(s *Services) error
	OverrideDatabase(s *Database) error
	OverrideHandlers(c *Handlers) error
	OverrideClients(c *Clients) error
	// OverrideAuthorizer returns the authorization policy. It runs after
	// OverrideServices, so the policy can use registered services. It must
	// not return nil.
	OverrideAuthorizer(env *Env) (authz.Authorizer, error)
	// OverrideAudit returns the recorder audit events are handed to
	// (SEC-004). It runs after OverrideAuthorizer. Return
	// audit.DefaultRecorder(env.Config.Audit) for JSON lines on stdout or a
	// file. It must not return nil.
	OverrideAudit(env *Env) (audit.Recorder, error)
}
