package scientists

import (
	"net/http"

	"google.golang.org/grpc"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/controllers"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/registry"
	pkgserver "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
	"github.com/openshift-online/rh-trex-ai/components/api-server/plugins/events"
	"github.com/openshift-online/rh-trex-ai/components/api-server/plugins/generic"
)

type ServiceLocator func() ScientistService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() ScientistService {
		return NewScientistService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewScientistDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
			generic.Service(&env.Services),
		)
	}
}

func Service(s *environments.Services) ScientistService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("Scientists"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("Scientists", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("scientists", func(apiRouter *pkgserver.APIRouter, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		scientistHandler := NewScientistHandler(Service(envServices))

		scientistsRouter := apiRouter.Resource("scientists").PathPrefix("/scientists").Subrouter()
		scientistsRouter.HandleFunc("", scientistHandler.List).Methods(http.MethodGet)
		scientistsRouter.HandleFunc("/{id}", scientistHandler.Get).Methods(http.MethodGet)
		scientistsRouter.HandleFunc("", scientistHandler.Create).Methods(http.MethodPost)
		scientistsRouter.HandleFunc("/{id}", scientistHandler.Patch).Methods(http.MethodPatch)
		scientistsRouter.HandleFunc("/{id}", scientistHandler.Delete).Methods(http.MethodDelete)
	})

	pkgserver.RegisterController("Scientists", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		scientistServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "Scientists",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {scientistServices.OnUpsert},
				api.UpdateEventType: {scientistServices.OnUpsert},
				api.DeleteEventType: {scientistServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("scientists", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		scientistService := Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterScientistServiceServer(grpcServer, NewScientistGRPCHandler(scientistService, brokerFunc))
	}, authz.CRUDResources(pb.ScientistService_ServiceDesc.ServiceName, "scientists", "Scientist", "Scientists"))

	presenters.RegisterPath(Scientist{}, "scientists")
	presenters.RegisterPath(&Scientist{}, "scientists")
	presenters.RegisterKind(Scientist{}, "Scientist")
	presenters.RegisterKind(&Scientist{}, "Scientist")

	db.RegisterMigration(migration())
}
