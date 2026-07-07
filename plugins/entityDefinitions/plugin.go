package entityDefinitions

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
	"github.com/openshift-online/rh-trex-ai/plugins/fieldDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/generic"
	"github.com/openshift-online/rh-trex-ai/plugins/relationships"
)

type ServiceLocator func() EntityDefinitionService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() EntityDefinitionService {
		return NewEntityDefinitionService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewEntityDefinitionDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
			fieldDefinitions.NewFieldDefinitionDao(&env.Database.SessionFactory),
			relationships.NewRelationshipDao(&env.Database.SessionFactory),
		)
	}
}

func Service(s *environments.Services) EntityDefinitionService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("EntityDefinitions"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("EntityDefinitions", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("entityDefinitions", func(apiV1Router *mux.Router, services pkgserver.ServicesInterface, authMiddleware environments.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
		envServices := services.(*environments.Services)
		entityDefinitionHandler := NewEntityDefinitionHandler(Service(envServices), generic.Service(envServices))

		entityDefinitionsRouter := apiV1Router.PathPrefix("/entity_definitions").Subrouter()
		entityDefinitionsRouter.HandleFunc("", entityDefinitionHandler.List).Methods(http.MethodGet)
		entityDefinitionsRouter.HandleFunc("/{id}", entityDefinitionHandler.Get).Methods(http.MethodGet)
		entityDefinitionsRouter.HandleFunc("", entityDefinitionHandler.Create).Methods(http.MethodPost)
		entityDefinitionsRouter.HandleFunc("/{id}", entityDefinitionHandler.Patch).Methods(http.MethodPatch)
		entityDefinitionsRouter.HandleFunc("/{id}", entityDefinitionHandler.Delete).Methods(http.MethodDelete)
		entityDefinitionsRouter.Use(authMiddleware.AuthenticateAccountJWT)
		entityDefinitionsRouter.Use(authzMiddleware.AuthorizeApi)
	})

	pkgserver.RegisterController("EntityDefinitions", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		entityDefinitionServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "EntityDefinitions",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {entityDefinitionServices.OnUpsert},
				api.UpdateEventType: {entityDefinitionServices.OnUpsert},
				api.DeleteEventType: {entityDefinitionServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("entityDefinitions", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		entityDefinitionService := Service(envServices)
		genericService := generic.Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterEntityDefinitionServiceServer(grpcServer, NewEntityDefinitionGRPCHandler(entityDefinitionService, genericService, brokerFunc))
	})

	presenters.RegisterPath(EntityDefinition{}, "entity_definitions")
	presenters.RegisterPath(&EntityDefinition{}, "entity_definitions")
	presenters.RegisterKind(EntityDefinition{}, "EntityDefinition")
	presenters.RegisterKind(&EntityDefinition{}, "EntityDefinition")

	db.RegisterMigration(migration())
}
