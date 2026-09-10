package events

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/registry"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

func NewServiceLocator(env *environments.Env) services.EventServiceLocator {
	return func() services.EventService {
		return services.NewEventService(dao.NewEventDao(&env.Database.SessionFactory))
	}
}

// Service helper function to get the event service from the registry
func Service(s *environments.Services) services.EventService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("Events"); obj != nil {
		locator := obj.(services.EventServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("Events", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	db.RegisterMigration(migration())
}
