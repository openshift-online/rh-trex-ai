package environments

import (
	"sync"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/client/ocm"
	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

const (
	UnitTestingEnv        string = "unit_testing"
	IntegrationTestingEnv string = "integration_testing"
	DevelopmentEnv        string = "development"
	ProductionEnv         string = "production"

	EnvironmentStringKey string = "OCM_ENV"
	EnvironmentDefault          = DevelopmentEnv
)

type Env struct {
	Name     string
	Services Services
	Handlers Handlers
	Clients  Clients
	Database Database
	Config   *config.ApplicationConfig
}

type ApplicationConfig struct {
	ApplicationConfig *config.ApplicationConfig
}

type Database struct {
	SessionFactory db.SessionFactory
}

type Handlers struct {
	AuthMiddleware auth.JWTMiddleware
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

type Clients struct {
	OCM *ocm.Client
}

type ConfigDefaults struct {
	Server   map[string]interface{}
	Metrics  map[string]interface{}
	Database map[string]interface{}
	OCM      map[string]interface{}
	Options  map[string]interface{}
}

type EnvironmentImpl interface {
	Flags() map[string]string
	OverrideConfig(c *config.ApplicationConfig) error
	OverrideServices(s *Services) error
	OverrideDatabase(s *Database) error
	OverrideHandlers(c *Handlers) error
	OverrideClients(c *Clients) error
}
