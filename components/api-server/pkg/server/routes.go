package server

import (
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
)

type ServicesInterface interface {
	GetService(name string) interface{}
}

type RouteRegistrationFunc func(apiV1Router *mux.Router, services ServicesInterface, authMiddleware environments.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware)

var routeRegistry = make(map[string]RouteRegistrationFunc)

func RegisterRoutes(name string, registrationFunc RouteRegistrationFunc) {
	routeRegistry[name] = registrationFunc
}

func LoadDiscoveredRoutes(apiV1Router *mux.Router, services ServicesInterface, authMiddleware environments.JWTMiddleware, authzMiddleware auth.AuthorizationMiddleware) {
	for _, registrationFunc := range routeRegistry {
		registrationFunc(apiV1Router, services, authMiddleware, authzMiddleware)
	}
}

// prefixedRouteRegistry maps a URL prefix segment (e.g. "ext", "beta") to its
// named registration funcs. Each unique prefix gets its own subrouter with the
// full standard middleware stack, built automatically by LoadDiscoveredPrefixedRoutes.
var prefixedRouteRegistry = make(map[string]map[string]RouteRegistrationFunc)

// RegisterPrefixedRoutes registers a plugin's routes under /api/<base>/<prefix>/.
// The prefix is a single path segment without slashes (e.g. "ext", "beta", "v2").
// All plugins sharing the same prefix are mounted on one subrouter.
func RegisterPrefixedRoutes(name, prefix string, registrationFunc RouteRegistrationFunc) {
	if prefixedRouteRegistry[prefix] == nil {
		prefixedRouteRegistry[prefix] = make(map[string]RouteRegistrationFunc)
	}
	prefixedRouteRegistry[prefix][name] = registrationFunc
}

// LoadDiscoveredPrefixedRoutes creates one subrouter per registered prefix under
// apiRouter and calls each plugin's registration func with that subrouter.
// The applyMiddleware callback receives each fresh subrouter so the caller can
// attach the standard middleware stack (metrics, auth, transaction, compress).
func LoadDiscoveredPrefixedRoutes(
	apiRouter *mux.Router,
	services ServicesInterface,
	authMiddleware environments.JWTMiddleware,
	authzMiddleware auth.AuthorizationMiddleware,
	applyMiddleware func(*mux.Router),
) {
	for prefix, registry := range prefixedRouteRegistry {
		prefixRouter := apiRouter.PathPrefix("/" + prefix).Subrouter()
		applyMiddleware(prefixRouter)
		for _, registrationFunc := range registry {
			registrationFunc(prefixRouter, services, authMiddleware, authzMiddleware)
		}
	}
}
