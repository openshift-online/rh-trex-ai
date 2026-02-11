package dinosaurs

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openshift-online/rh-trex/pkg/api"
	"github.com/openshift-online/rh-trex/pkg/api/presenters"
	"github.com/openshift-online/rh-trex/pkg/auth"
	"github.com/openshift-online/rh-trex/pkg/controllers"
	"github.com/openshift-online/rh-trex/pkg/db"
	"github.com/openshift-online/rh-trex/pkg/environments"
	"github.com/openshift-online/rh-trex/pkg/registry"
	pkgserver "github.com/openshift-online/rh-trex/pkg/server"
	"github.com/openshift-online/rh-trex/plugins/events"
	"github.com/openshift-online/rh-trex/plugins/generic"
)

type ServiceLocator func() DinosaurService

func NewServiceLocator(env *environments.Env) ServiceLocator {
	return func() DinosaurService {
		return NewDinosaurService(
			db.NewAdvisoryLockFactory(env.Database.SessionFactory),
			NewDinosaurDao(&env.Database.SessionFactory),
			events.Service(&env.Services),
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

	pkgserver.RegisterRoutes("dinosaurs", func(apiV1Router *mux.Router, services pkgserver.ServicesInterface, authMiddleware auth.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
		envServices := services.(*environments.Services)
		dinosaurHandler := NewDinosaurHandler(Service(envServices), generic.Service(envServices))

		dinosaursRouter := apiV1Router.PathPrefix("/dinosaurs").Subrouter()
		dinosaursRouter.HandleFunc("", dinosaurHandler.List).Methods(http.MethodGet)
		dinosaursRouter.HandleFunc("/{id}", dinosaurHandler.Get).Methods(http.MethodGet)
		dinosaursRouter.HandleFunc("", dinosaurHandler.Create).Methods(http.MethodPost)
		dinosaursRouter.HandleFunc("/{id}", dinosaurHandler.Patch).Methods(http.MethodPatch)
		dinosaursRouter.HandleFunc("/{id}", dinosaurHandler.Delete).Methods(http.MethodDelete)
		dinosaursRouter.Use(authMiddleware.AuthenticateAccountJWT)
		dinosaursRouter.Use(authzMiddleware.AuthorizeApi)
	})

	pkgserver.RegisterController("Dinosaurs", func(manager *controllers.KindControllerManager, services pkgserver.ServicesInterface) {
		dinoServices := Service(services.(*environments.Services))

		manager.Add(&controllers.ControllerConfig{
			Source: "Dinosaurs",
			Handlers: map[api.EventType][]controllers.ControllerHandlerFunc{
				api.CreateEventType: {dinoServices.OnUpsert},
				api.UpdateEventType: {dinoServices.OnUpsert},
				api.DeleteEventType: {dinoServices.OnDelete},
			},
		})
	})

	presenters.RegisterPath(Dinosaur{}, "dinosaurs")
	presenters.RegisterPath(&Dinosaur{}, "dinosaurs")
	presenters.RegisterKind(Dinosaur{}, "Dinosaur")
	presenters.RegisterKind(&Dinosaur{}, "Dinosaur")

	db.RegisterMigration(migration())
}
