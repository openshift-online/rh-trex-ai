package server

import (
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
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

type RootRouteRegistrationFunc func(mainRouter *mux.Router)

var rootRouteRegistry = make(map[string]RootRouteRegistrationFunc)

func RegisterRootRoutes(name string, registrationFunc RootRouteRegistrationFunc) {
	rootRouteRegistry[name] = registrationFunc
}

func LoadDiscoveredRootRoutes(mainRouter *mux.Router) {
	for _, registrationFunc := range rootRouteRegistry {
		registrationFunc(mainRouter)
	}
}
