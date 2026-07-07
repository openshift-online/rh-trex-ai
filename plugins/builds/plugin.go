package builds

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
	"github.com/openshift-online/rh-trex-ai/plugins/entityDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/events"
	"github.com/openshift-online/rh-trex-ai/plugins/fieldDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/generic"
)

type ServiceLocator func() BuildService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() BuildService {
		return NewBuildService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewBuildDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
			entityDefinitions.NewEntityDefinitionDao(&env.Database.SessionFactory),
			fieldDefinitions.NewFieldDefinitionDao(&env.Database.SessionFactory),
		)
	}
}

func Service(s *environments.Services) BuildService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("Builds"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("Builds", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("builds", func(apiV1Router *mux.Router, services pkgserver.ServicesInterface, authMiddleware environments.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
		envServices := services.(*environments.Services)
		buildHandler := NewBuildHandler(Service(envServices), generic.Service(envServices))

		buildsRouter := apiV1Router.PathPrefix("/builds").Subrouter()
		buildsRouter.HandleFunc("", buildHandler.List).Methods(http.MethodGet)
		buildsRouter.HandleFunc("/{id}", buildHandler.Get).Methods(http.MethodGet)
		buildsRouter.HandleFunc("", buildHandler.Create).Methods(http.MethodPost)
		buildsRouter.HandleFunc("/{id}", buildHandler.Patch).Methods(http.MethodPatch)
		buildsRouter.HandleFunc("/{id}", buildHandler.Delete).Methods(http.MethodDelete)
		buildsRouter.Use(authMiddleware.AuthenticateAccountJWT)
		buildsRouter.Use(authzMiddleware.AuthorizeApi)
	})

	pkgserver.RegisterController("Builds", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		buildServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "Builds",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {buildServices.OnUpsert},
				api.UpdateEventType: {buildServices.OnUpsert},
				api.DeleteEventType: {buildServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("builds", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		buildService := Service(envServices)
		genericService := generic.Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterBuildServiceServer(grpcServer, NewBuildGRPCHandler(buildService, genericService, brokerFunc))
	})

	presenters.RegisterPath(Build{}, "builds")
	presenters.RegisterPath(&Build{}, "builds")
	presenters.RegisterKind(Build{}, "Build")
	presenters.RegisterKind(&Build{}, "Build")

	db.RegisterMigration(migration())
}
