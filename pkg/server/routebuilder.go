package server

import (
	"fmt"
	"net/http"
	"strings"

	gorillahandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/server/logging"
	"github.com/openshift-online/rh-trex-ai/pkg/trex"
)

func BuildDefaultRoutes(env *environments.Env, specData []byte) *mux.Router {
	services := &env.Services

	metadataHandler := handlers.NewMetadataHandler()

	// Build authentication middleware based on configuration
	authConfig := env.Config.GetEffectiveAuthConfig()
	authBuilder := auth.NewAuthMiddlewareBuilder(authConfig)
	httpAuthMiddleware, err := authBuilder.BuildHTTPMiddleware()
	if err != nil {
		Check(err, "Unable to create HTTP auth middleware")
	}
	
	// For backward compatibility, also create JWT middleware for plugins that expect it
	var authMiddleware environments.JWTMiddleware
	if authConfig.EnableJWT {
		var err error
		middleware, err := auth.NewAuthMiddleware()
		if err != nil {
			Check(err, "Unable to create JWT middleware")
		}
		authMiddleware = middleware
	} else {
		authMiddleware = &auth.MiddlewareMock{}
	}
	if authMiddleware == nil {
		Check(fmt.Errorf("auth middleware is nil"), "Unable to create auth middleware: missing middleware")
	}

	authzMiddleware := auth.NewAuthzMiddlewareMock() //nolint:staticcheck // placeholder for real authz middleware

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
		Check(err, "Unable to create OpenAPI handler")
	}
	apiV1Router.HandleFunc("/openapi.html", openapiHandler.GetOpenAPIUI).Methods(http.MethodGet)
	apiV1Router.HandleFunc("/openapi", openapiHandler.GetOpenAPI).Methods(http.MethodGet)

	apiV1Router.Use(MetricsMiddleware)
	
	// Apply authentication middleware if configured
	if httpAuthMiddleware != nil {
		apiV1Router.Use(httpAuthMiddleware)
	}
	
	apiV1Router.Use(
		func(next http.Handler) http.Handler {
			return db.TransactionMiddleware(next, env.Database.SessionFactory)
		},
	)
	apiV1Router.Use(gorillahandlers.CompressHandler)

	LoadDiscoveredRoutes(apiV1Router, services, authMiddleware, authzMiddleware)

	return mainRouter
}
