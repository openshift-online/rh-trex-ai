package fossils

import (
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/controllers"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/registry"
	pkgserver "github.com/openshift-online/rh-trex-ai/pkg/server"
	"github.com/openshift-online/rh-trex-ai/plugins/events"
	"github.com/openshift-online/rh-trex-ai/plugins/generic"
)

type ServiceLocator func() FossilService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() FossilService {
		return NewFossilService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewFossilDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
		)
	}
}

func Service(s *environments.Services) FossilService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("Fossils"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("Fossils", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("fossils", func(apiV1Router *mux.Router, services pkgserver.ServicesInterface, authMiddleware auth.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
		envServices := services.(*environments.Services)
		fossilHandler := NewFossilHandler(Service(envServices), generic.Service(envServices))

		fossilsRouter := apiV1Router.PathPrefix("/fossils").Subrouter()
		fossilsRouter.HandleFunc("", fossilHandler.List).Methods(http.MethodGet)
		fossilsRouter.HandleFunc("/{id}", fossilHandler.Get).Methods(http.MethodGet)
		fossilsRouter.HandleFunc("", fossilHandler.Create).Methods(http.MethodPost)
		fossilsRouter.HandleFunc("/{id}", fossilHandler.Patch).Methods(http.MethodPatch)
		fossilsRouter.HandleFunc("/{id}", fossilHandler.Delete).Methods(http.MethodDelete)
		fossilsRouter.Use(authMiddleware.AuthenticateAccountJWT)
		fossilsRouter.Use(authzMiddleware.AuthorizeApi)
	})

	pkgserver.RegisterController("Fossils", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		fossilServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "Fossils",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {fossilServices.OnUpsert},
				api.UpdateEventType: {fossilServices.OnUpsert},
				api.DeleteEventType: {fossilServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("fossils", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		fossilService := Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterFossilServiceServer(grpcServer, NewFossilGRPCHandler(fossilService, brokerFunc))
	})

	presenters.RegisterPath(Fossil{}, "fossils")
	presenters.RegisterPath(&Fossil{}, "fossils")
	presenters.RegisterKind(Fossil{}, "Fossil")
	presenters.RegisterKind(&Fossil{}, "Fossil")

	db.RegisterMigration(migration())
}
