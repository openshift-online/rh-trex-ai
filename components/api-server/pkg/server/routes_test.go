package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
)

// resetPrefixedRegistry clears global state between tests.
func resetPrefixedRegistry() func() {
	orig := prefixedRouteRegistry
	prefixedRouteRegistry = make(map[string]map[string]RouteRegistrationFunc)
	return func() { prefixedRouteRegistry = orig }
}

func noopMiddleware(r *mux.Router) {}

func nopRegistration(_ *mux.Router, _ ServicesInterface, _ environments.JWTMiddleware, _ auth.AuthorizationMiddleware) {
}

func TestRegisterPrefixedRoutes_RouteReachable(t *testing.T) {
	defer resetPrefixedRegistry()()

	hit := false
	RegisterPrefixedRoutes("things", "ext", func(router *mux.Router, _ ServicesInterface, _ environments.JWTMiddleware, _ auth.AuthorizationMiddleware) {
		router.HandleFunc("/things", func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})

	apiRouter := mux.NewRouter().PathPrefix("/api/hypershell").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, nil, nil, noopMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/hypershell/ext/things", nil)
	rr := httptest.NewRecorder()
	apiRouter.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !hit {
		t.Fatal("handler was not called")
	}
}

func TestRegisterPrefixedRoutes_NotUnderV1(t *testing.T) {
	defer resetPrefixedRegistry()()

	RegisterPrefixedRoutes("things", "ext", func(router *mux.Router, _ ServicesInterface, _ environments.JWTMiddleware, _ auth.AuthorizationMiddleware) {
		router.HandleFunc("/things", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})

	apiRouter := mux.NewRouter().PathPrefix("/api/hypershell").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, nil, nil, noopMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/hypershell/v1/things", nil)
	rr := httptest.NewRecorder()
	apiRouter.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("ext route should not be reachable under /v1")
	}
}

func TestRegisterPrefixedRoutes_MultiplePluginsSamePrefix(t *testing.T) {
	defer resetPrefixedRegistry()()

	hitA, hitB := false, false
	RegisterPrefixedRoutes("a", "ext", func(router *mux.Router, _ ServicesInterface, _ environments.JWTMiddleware, _ auth.AuthorizationMiddleware) {
		router.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
			hitA = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})
	RegisterPrefixedRoutes("b", "ext", func(router *mux.Router, _ ServicesInterface, _ environments.JWTMiddleware, _ auth.AuthorizationMiddleware) {
		router.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
			hitB = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})

	apiRouter := mux.NewRouter().PathPrefix("/api/hypershell").Subrouter()
	middlewareCalls := 0
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, nil, nil, func(r *mux.Router) {
		middlewareCalls++
	})

	// Both routes reachable
	for _, path := range []string{"/api/hypershell/ext/a", "/api/hypershell/ext/b"} {
		rr := httptest.NewRecorder()
		apiRouter.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rr.Code)
		}
	}
	if !hitA || !hitB {
		t.Fatal("both handlers should have been called")
	}
	// One shared subrouter means middleware applied exactly once for the "ext" prefix
	if middlewareCalls != 1 {
		t.Fatalf("expected middleware applied once for shared prefix, got %d", middlewareCalls)
	}
}

func TestRegisterPrefixedRoutes_DistinctPrefixes(t *testing.T) {
	defer resetPrefixedRegistry()()

	RegisterPrefixedRoutes("foo", "ext", nopRegistration)
	RegisterPrefixedRoutes("bar", "beta", nopRegistration)

	middlewareCalls := 0
	apiRouter := mux.NewRouter().PathPrefix("/api/hypershell").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, nil, nil, func(r *mux.Router) {
		middlewareCalls++
	})

	// Two distinct prefixes => two subrouters => middleware called twice
	if middlewareCalls != 2 {
		t.Fatalf("expected 2 middleware calls for 2 prefixes, got %d", middlewareCalls)
	}
}

func TestRegisterPrefixedRoutes_DoesNotAffectV1Registry(t *testing.T) {
	defer resetPrefixedRegistry()()
	origV1 := routeRegistry
	defer func() { routeRegistry = origV1 }()
	routeRegistry = make(map[string]RouteRegistrationFunc)

	RegisterPrefixedRoutes("things", "ext", nopRegistration)

	if len(routeRegistry) != 0 {
		t.Fatal("RegisterPrefixedRoutes must not modify the v1 routeRegistry")
	}
}
