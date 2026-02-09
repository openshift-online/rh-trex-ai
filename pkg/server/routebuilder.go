package server

import (
	"fmt"
	"net/http"
	"strings"

	gorillahandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex/pkg/api"
	"github.com/openshift-online/rh-trex/pkg/auth"
	"github.com/openshift-online/rh-trex/pkg/db"
	"github.com/openshift-online/rh-trex/pkg/environments"
	"github.com/openshift-online/rh-trex/pkg/handlers"
	"github.com/openshift-online/rh-trex/pkg/logger"
	"github.com/openshift-online/rh-trex/pkg/server/logging"
	"github.com/openshift-online/rh-trex/pkg/trex"
)

func BuildDefaultRoutes(env *environments.Env, specData []byte) *mux.Router {
	services := &env.Services

	metadataHandler := handlers.NewMetadataHandler()

	var authMiddleware auth.JWTMiddleware
	authMiddleware = &auth.MiddlewareMock{}
	if env.Config.Server.EnableJWT {
		var err error
		authMiddleware, err = auth.NewAuthMiddleware()
		if err != nil {
			Check(err, "Unable to create auth middleware", env.Config.Sentry.Timeout)
		}
	}
	if authMiddleware == nil {
		Check(fmt.Errorf("auth middleware is nil"), "Unable to create auth middleware: missing middleware", env.Config.Sentry.Timeout)
	}

	authzMiddleware := auth.NewAuthzMiddlewareMock()
	if env.Config.Server.EnableAuthz {
	}

	mainRouter := mux.NewRouter()
	mainRouter.NotFoundHandler = http.HandlerFunc(api.SendNotFound)
	mainRouter.Use(logger.OperationIDMiddleware)
	mainRouter.Use(logging.RequestLoggingMiddleware)

	apiPrefix := strings.TrimSuffix(trex.GetConfig().BasePath, "/v1")
	apiRouter := mainRouter.PathPrefix(apiPrefix).Subrouter()
	apiRouter.HandleFunc("", metadataHandler.Get).Methods(http.MethodGet)

	apiV1Router := apiRouter.PathPrefix("/v1").Subrouter()

	openapiHandler, err := handlers.NewOpenAPIHandler(specData)
	if err != nil {
		Check(err, "Unable to create OpenAPI handler", env.Config.Sentry.Timeout)
	}
	apiV1Router.HandleFunc("/openapi.html", openapiHandler.GetOpenAPIUI).Methods(http.MethodGet)
	apiV1Router.HandleFunc("/openapi", openapiHandler.GetOpenAPI).Methods(http.MethodGet)

	apiV1Router.Use(MetricsMiddleware)
	apiV1Router.Use(
		func(next http.Handler) http.Handler {
			return db.TransactionMiddleware(next, env.Database.SessionFactory)
		},
	)
	apiV1Router.Use(gorillahandlers.CompressHandler)

	LoadDiscoveredRoutes(apiV1Router, services, authMiddleware, authzMiddleware)

	return mainRouter
}
