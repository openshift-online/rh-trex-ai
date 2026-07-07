package relationships

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

type ServiceLocator func() RelationshipService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() RelationshipService {
		return NewRelationshipService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewRelationshipDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
		)
	}
}

func Service(s *environments.Services) RelationshipService {
	if s == nil {
		return nil
	}
	if obj := s.GetService("Relationships"); obj != nil {
		locator := obj.(ServiceLocator)
		return locator()
	}
	return nil
}

func init() {
	registry.RegisterService("Relationships", func(env interface{}) interface{} {
		return NewServiceLocator(env.(*environments.Env))
	})

	pkgserver.RegisterRoutes("relationships", func(apiV1Router *mux.Router, services pkgserver.ServicesInterface, authMiddleware environments.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
		envServices := services.(*environments.Services)
		relationshipHandler := NewRelationshipHandler(Service(envServices), generic.Service(envServices))

		relationshipsRouter := apiV1Router.PathPrefix("/relationships").Subrouter()
		relationshipsRouter.HandleFunc("", relationshipHandler.List).Methods(http.MethodGet)
		relationshipsRouter.HandleFunc("/{id}", relationshipHandler.Get).Methods(http.MethodGet)
		relationshipsRouter.HandleFunc("", relationshipHandler.Create).Methods(http.MethodPost)
		relationshipsRouter.HandleFunc("/{id}", relationshipHandler.Patch).Methods(http.MethodPatch)
		relationshipsRouter.HandleFunc("/{id}", relationshipHandler.Delete).Methods(http.MethodDelete)
		relationshipsRouter.Use(authMiddleware.AuthenticateAccountJWT)
		relationshipsRouter.Use(authzMiddleware.AuthorizeApi)
	})

	pkgserver.RegisterController("Relationships", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		relationshipServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "Relationships",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {relationshipServices.OnUpsert},
				api.UpdateEventType: {relationshipServices.OnUpsert},
				api.DeleteEventType: {relationshipServices.OnDelete},
			},
		})
	})

	pkgserver.RegisterGRPCService("relationships", func(grpcServer *grpc.Server, services pkgserver.ServicesInterface) {
		envServices := services.(*environments.Services)
		relationshipService := Service(envServices)
		genericService := generic.Service(envServices)
		brokerFunc := func() *pkgserver.EventBroker {
			if obj := envServices.GetService("EventBroker"); obj != nil {
				return obj.(*pkgserver.EventBroker)
			}
			return nil
		}
		pb.RegisterRelationshipServiceServer(grpcServer, NewRelationshipGRPCHandler(relationshipService, genericService, brokerFunc))
	})

	presenters.RegisterPath(Relationship{}, "relationships")
	presenters.RegisterPath(&Relationship{}, "relationships")
	presenters.RegisterKind(Relationship{}, "Relationship")
	presenters.RegisterKind(&Relationship{}, "Relationship")

	db.RegisterMigration(migration())
}
