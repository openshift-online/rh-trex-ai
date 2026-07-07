package fieldDefinitions

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

type ServiceLocator func() FieldDefinitionService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() FieldDefinitionService {
		return NewFieldDefinitionService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewFieldDefinitionDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
		)
	}
}

func Service(s *environments.Services) FieldDefinitionService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("FieldDefinitions"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("FieldDefinitions", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("fieldDefinitions", func(apiV1Router *mux.Router, services pkgserver.ServicesInterface, authMiddleware environments.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
		envServices := services.(*environments.Services)
		fieldDefinitionHandler := NewFieldDefinitionHandler(Service(envServices), generic.Service(envServices))

		fieldDefinitionsRouter := apiV1Router.PathPrefix("/field_definitions").Subrouter()
		fieldDefinitionsRouter.HandleFunc("", fieldDefinitionHandler.List).Methods(http.MethodGet)
		fieldDefinitionsRouter.HandleFunc("/{id}", fieldDefinitionHandler.Get).Methods(http.MethodGet)
		fieldDefinitionsRouter.HandleFunc("", fieldDefinitionHandler.Create).Methods(http.MethodPost)
		fieldDefinitionsRouter.HandleFunc("/{id}", fieldDefinitionHandler.Patch).Methods(http.MethodPatch)
		fieldDefinitionsRouter.HandleFunc("/{id}", fieldDefinitionHandler.Delete).Methods(http.MethodDelete)
		fieldDefinitionsRouter.Use(authMiddleware.AuthenticateAccountJWT)
		fieldDefinitionsRouter.Use(authzMiddleware.AuthorizeApi)
	})

	pkgserver.RegisterController("FieldDefinitions", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		fieldDefinitionServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "FieldDefinitions",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {fieldDefinitionServices.OnUpsert},
				api.UpdateEventType: {fieldDefinitionServices.OnUpsert},
				api.DeleteEventType: {fieldDefinitionServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("fieldDefinitions", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		fieldDefinitionService := Service(envServices)
		genericService := generic.Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterFieldDefinitionServiceServer(grpcServer, NewFieldDefinitionGRPCHandler(fieldDefinitionService, genericService, brokerFunc))
	})

	presenters.RegisterPath(FieldDefinition{}, "field_definitions")
	presenters.RegisterPath(&FieldDefinition{}, "field_definitions")
	presenters.RegisterKind(FieldDefinition{}, "FieldDefinition")
	presenters.RegisterKind(&FieldDefinition{}, "FieldDefinition")

	db.RegisterMigration(migration())
}
