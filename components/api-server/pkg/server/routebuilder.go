package server

import (
	"net/http"
	"strings"

	gorillahandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server/logging"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/trex"
)

func BuildDefaultRoutes(env *environments.Env, specData []byte) *mux.Router {
	services := &env.Services

	metadataHandler := handlers.NewMetadataHandler()

	// Build authentication middleware based on configuration
	authConfig := env.Config.GetAuthConfig()
	authBuilder := auth.NewAuthMiddlewareBuilder(authConfig)
	httpAuthMiddleware, err := authBuilder.BuildHTTPMiddleware()
	if err != nil {
		Check(err, "Unable to create HTTP auth middleware")
	}

	// Without JWT there is no verified identity. Use the local development
	// identity so policies such as RequireIdentity still see a subject.
	if !authConfig.EnableJWT {
		devIdentity := (&auth.MiddlewareMock{}).AuthenticateAccountJWT
		authn := httpAuthMiddleware
		httpAuthMiddleware = func(next http.Handler) http.Handler { return authn(devIdentity(next)) }
	}

	mainRouter := mux.NewRouter()
	mainRouter.NotFoundHandler = http.HandlerFunc(api.SendNotFound)
	mainRouter.Use(logger.OperationIDMiddleware)
	mainRouter.Use(logging.RequestLoggingMiddleware)

	apiPrefix := strings.TrimSuffix(trex.GetConfig().BasePath, "/v1")
	apiRouter := mainRouter.PathPrefix(apiPrefix).Subrouter()
	apiRouter.HandleFunc("", metadataHandler.Get).Methods(http.MethodGet)

	// OpenAPI routes are public. Register them on apiRouter before the
	// apiV1Router prefix subrouter so gorilla/mux matches them first and they
	// are not wrapped by the auth middleware applied to apiV1Router.
	openapiHandler, err := handlers.NewOpenAPIHandler(specData)
	if err != nil {
		Check(err, "Unable to create OpenAPI handler")
	}
	apiRouter.HandleFunc("/v1/openapi.html", openapiHandler.GetOpenAPIUI).Methods(http.MethodGet)
	apiRouter.HandleFunc("/v1/openapi", openapiHandler.GetOpenAPI).Methods(http.MethodGet)

	apiRouter.HandleFunc("/v1/auth/config", loginConfigHandler(authConfig)).Methods(http.MethodGet)

	apiV1Router := apiRouter.PathPrefix("/v1").Subrouter()

	apiV1Router.Use(MetricsMiddleware)

	// The outer audit hook runs before authentication so it also records
	// authentication failures (SEC-004).
	apiV1Router.Use(env.Audit.HTTPBegin)

	apiV1Router.Use(httpAuthMiddleware)

	apiV1Router.Use(
		func(next http.Handler) http.Handler {
			return db.TransactionMiddleware(next, env.Database.SessionFactory)
		},
	)
	apiV1Router.Use(gorillahandlers.CompressHandler)

	// Authorization is attached per declared resource by APIRouter, after
	// the authentication above.
	LoadDiscoveredRoutes(apiV1Router, services, env.Authorizer)

	LoadDiscoveredPrefixedRoutes(apiRouter, services, env.Authorizer,
		func(r *mux.Router) {
			r.Use(MetricsMiddleware)
			r.Use(env.Audit.HTTPBegin)
			r.Use(httpAuthMiddleware)
			r.Use(func(next http.Handler) http.Handler {
				return db.TransactionMiddleware(next, env.Database.SessionFactory)
			})
			r.Use(gorillahandlers.CompressHandler)
		},
	)

	return mainRouter
}
