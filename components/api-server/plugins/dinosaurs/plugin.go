package dinosaurs

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

type ServiceLocator func() DinosaurService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() DinosaurService {
		return NewDinosaurService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewDinosaurDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
			generic.Service(&env.Services),
		)
	}
}

func Service(s *environments.Services) DinosaurService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("Dinosaurs"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("Dinosaurs", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("dinosaurs", func(apiRouter *pkgserver.APIRouter, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		dinosaurHandler := NewDinosaurHandler(Service(envServices))

		dinosaursRouter := apiRouter.Resource("dinosaurs").PathPrefix("/dinosaurs").Subrouter()
		dinosaursRouter.HandleFunc("", dinosaurHandler.List).Methods(http.MethodGet)
		dinosaursRouter.HandleFunc("/{id}", dinosaurHandler.Get).Methods(http.MethodGet)
		dinosaursRouter.HandleFunc("", dinosaurHandler.Create).Methods(http.MethodPost)
		dinosaursRouter.HandleFunc("/{id}", dinosaurHandler.Patch).Methods(http.MethodPatch)
		dinosaursRouter.HandleFunc("/{id}", dinosaurHandler.Delete).Methods(http.MethodDelete)
	})

	pkgserver.RegisterController("Dinosaurs", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		dinosaurServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "Dinosaurs",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {dinosaurServices.OnUpsert},
				api.UpdateEventType: {dinosaurServices.OnUpsert},
				api.DeleteEventType: {dinosaurServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("dinosaurs", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		dinosaurService := Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterDinosaurServiceServer(grpcServer, NewDinosaurGRPCHandler(dinosaurService, brokerFunc))
	}, authz.CRUDResources(pb.DinosaurService_ServiceDesc.ServiceName, "dinosaurs", "Dinosaur", "Dinosaurs"))

	presenters.RegisterPath(Dinosaur{}, "dinosaurs")
	presenters.RegisterPath(&Dinosaur{}, "dinosaurs")
	presenters.RegisterKind(Dinosaur{}, "Dinosaur")
	presenters.RegisterKind(&Dinosaur{}, "Dinosaur")

	db.RegisterMigration(migration())
}
