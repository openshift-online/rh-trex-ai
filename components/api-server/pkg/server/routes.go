package server

import (
	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
)

type ServicesInterface interface {
	GetService(name string) interface{}
}

// RouteRegistrationFunc registers a plugin's routes. Plugins register routes
// only on subrouters from r.Resource(kind) or r.Public(); TRex applies
// authentication and authorization itself.
type RouteRegistrationFunc func(r *APIRouter, services ServicesInterface)

// APIRouter is the router handed to plugins for one API surface (/v1 or a
// prefix such as /ext). Every route must be declared with Resource or Public.
type APIRouter struct {
	router     *mux.Router
	surface    string
	authorizer authz.Authorizer
	audit      *audit.Service
	declared   map[*mux.Route]bool
}

// RouteOption configures an APIRouter.
type RouteOption func(*APIRouter)

// WithAudit records audit events for every declared route (SEC-004). TRex
// passes it for the main and prefixed surfaces.
func WithAudit(s *audit.Service) RouteOption {
	return func(r *APIRouter) { r.audit = s }
}

// NewAPIRouter wraps router for one surface.
func NewAPIRouter(router *mux.Router, surface string, authorizer authz.Authorizer, opts ...RouteOption) *APIRouter {
	r := &APIRouter{
		router:     router,
		surface:    surface,
		authorizer: authorizer,
		declared:   make(map[*mux.Route]bool),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resource returns a subrouter for one kind. Every request on it is
// authorized with Resource.Kind set to kind.
func (r *APIRouter) Resource(kind string) *mux.Router {
	route := r.router.NewRoute()
	sub := route.Subrouter()
	sub.Use(authz.HTTPMiddleware(r.authorizer, r.surface, kind))
	sub.Use(r.audit.HTTPRecord)
	r.declared[route] = true
	return sub
}

// Public returns a subrouter whose requests skip authorization. They are
// still authenticated. Use sparingly.
func (r *APIRouter) Public() *mux.Router {
	route := r.router.NewRoute()
	sub := route.Subrouter()
	sub.Use(authz.HTTPPublicSurfaceMiddleware(r.authorizer, r.surface))
	sub.Use(r.audit.HTTPRecord)
	r.declared[route] = true
	return sub
}

// denyUndeclared replaces the handler of every route registered outside
// Resource and Public with one that denies the request (fail closed).
func (r *APIRouter) denyUndeclared() {
	_ = r.router.Walk(func(route *mux.Route, _ *mux.Router, ancestors []*mux.Route) error {
		if route.GetHandler() == nil || r.declared[route] {
			return nil
		}
		for _, a := range ancestors {
			if r.declared[a] {
				return nil
			}
		}
		path, _ := route.GetPathTemplate()
		route.Handler(authz.UndeclaredHandler(path))
		return nil
	})
}

var routeRegistry = make(map[string]RouteRegistrationFunc)

func RegisterRoutes(name string, registrationFunc RouteRegistrationFunc) {
	routeRegistry[name] = registrationFunc
}

// LoadDiscoveredRoutes registers every plugin's routes on the /v1 router.
func LoadDiscoveredRoutes(apiV1Router *mux.Router, services ServicesInterface, authorizer authz.Authorizer, opts ...RouteOption) {
	r := NewAPIRouter(apiV1Router, authz.SurfaceV1, authorizer, opts...)
	for _, registrationFunc := range routeRegistry {
		registrationFunc(r, services)
	}
	r.denyUndeclared()
}

// prefixedRouteRegistry maps a URL prefix segment (e.g. "ext", "beta") to its
// named registration funcs. Each unique prefix gets its own subrouter with the
// full standard middleware stack, built automatically by LoadDiscoveredPrefixedRoutes.
var prefixedRouteRegistry = make(map[string]map[string]RouteRegistrationFunc)

// RegisterPrefixedRoutes registers a plugin's routes under /api/<base>/<prefix>/.
// The prefix is a single path segment without slashes (e.g. "ext", "beta", "v2").
// All plugins sharing the same prefix are mounted on one subrouter, and the
// prefix is the Surface their requests are authorized with.
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
	authorizer authz.Authorizer,
	applyMiddleware func(*mux.Router),
	opts ...RouteOption,
) {
	for prefix, registry := range prefixedRouteRegistry {
		prefixRouter := apiRouter.PathPrefix("/" + prefix).Subrouter()
		applyMiddleware(prefixRouter)
		r := NewAPIRouter(prefixRouter, prefix, authorizer, opts...)
		for _, registrationFunc := range registry {
			registrationFunc(r, services)
		}
		r.denyUndeclared()
	}
}
