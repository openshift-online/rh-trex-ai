package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
)

// resetPrefixedRegistry clears global state between tests.
func resetPrefixedRegistry() func() {
	orig := prefixedRouteRegistry
	prefixedRouteRegistry = make(map[string]map[string]RouteRegistrationFunc)
	return func() { prefixedRouteRegistry = orig }
}

func noopMiddleware(r *mux.Router) {}

func nopRegistration(_ *APIRouter, _ ServicesInterface) {}

func TestRegisterPrefixedRoutes_RouteReachable(t *testing.T) {
	defer resetPrefixedRegistry()()

	hit := false
	RegisterPrefixedRoutes("things", "ext", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("things").HandleFunc("/things", func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})

	apiRouter := mux.NewRouter().PathPrefix("/api/rh-trex-ai").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, authz.AllowAll(), noopMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/rh-trex-ai/ext/things", nil)
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

	RegisterPrefixedRoutes("things", "ext", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("things").HandleFunc("/things", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})

	apiRouter := mux.NewRouter().PathPrefix("/api/rh-trex-ai").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, authz.AllowAll(), noopMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/api/rh-trex-ai/v1/things", nil)
	rr := httptest.NewRecorder()
	apiRouter.ServeHTTP(rr, req)

	if rr.Code == http.StatusOK {
		t.Fatal("ext route should not be reachable under /v1")
	}
}

func TestRegisterPrefixedRoutes_MultiplePluginsSamePrefix(t *testing.T) {
	defer resetPrefixedRegistry()()

	hitA, hitB := false, false
	RegisterPrefixedRoutes("a", "ext", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("things").HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
			hitA = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})
	RegisterPrefixedRoutes("b", "ext", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("things").HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
			hitB = true
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
	})

	apiRouter := mux.NewRouter().PathPrefix("/api/rh-trex-ai").Subrouter()
	middlewareCalls := 0
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, authz.AllowAll(), func(r *mux.Router) {
		middlewareCalls++
	})

	// Both routes reachable
	for _, path := range []string{"/api/rh-trex-ai/ext/a", "/api/rh-trex-ai/ext/b"} {
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
	apiRouter := mux.NewRouter().PathPrefix("/api/rh-trex-ai").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, authz.AllowAll(), func(r *mux.Router) {
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

// recordingAuthorizer records each request and returns a fixed decision.
type recordingAuthorizer struct {
	decision authz.Decision
	err      error
	calls    []authz.Request
}

func (a *recordingAuthorizer) Authorize(_ context.Context, req authz.Request) (authz.Decision, error) {
	a.calls = append(a.calls, req)
	return a.decision, a.err
}

func resetV1Registry() func() {
	orig := routeRegistry
	routeRegistry = make(map[string]RouteRegistrationFunc)
	return func() { routeRegistry = orig }
}

func serve(router *mux.Router, method, path string) int {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(method, path, nil))
	return rr.Code
}

func okHandler(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func TestResource_AuthorizerReceivesKindIDParentAndAction(t *testing.T) {
	defer resetV1Registry()()
	RegisterRoutes("fossils", func(r *APIRouter, _ ServicesInterface) {
		rt := r.Resource("fossils")
		rt.HandleFunc("/dinosaurs/{dinosaur_id}/fossils", okHandler).Methods(http.MethodPost)
		rt.HandleFunc("/fossils/{id}", okHandler).Methods(http.MethodGet)
	})
	a := &recordingAuthorizer{decision: authz.Allow()}
	v1 := mux.NewRouter().PathPrefix("/api/rh-trex-ai/v1").Subrouter()
	LoadDiscoveredRoutes(v1, nil, a)

	if code := serve(v1, http.MethodGet, "/api/rh-trex-ai/v1/fossils/f-1"); code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", code)
	}
	if code := serve(v1, http.MethodPost, "/api/rh-trex-ai/v1/dinosaurs/d-1/fossils"); code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d", code)
	}
	if len(a.calls) != 2 {
		t.Fatalf("expected 2 authorize calls, got %d", len(a.calls))
	}
	get, create := a.calls[0], a.calls[1]
	if get.Action != authz.ActionGet || get.Resource.Kind != "fossils" || get.Resource.ID != "f-1" || get.Resource.Surface != authz.SurfaceV1 {
		t.Fatalf("unexpected get request: %+v", get)
	}
	if create.Action != authz.ActionCreate || create.Resource.Parent["dinosaur_id"] != "d-1" || create.Resource.ID != "" {
		t.Fatalf("unexpected create request: %+v", create)
	}
}

func TestResource_DecisionMapping(t *testing.T) {
	cases := []struct {
		name     string
		decision authz.Decision
		err      error
		want     int
	}{
		{"deny", authz.Deny("no"), nil, http.StatusForbidden},
		{"hide existence", authz.Decision{HideExistence: true}, nil, http.StatusNotFound},
		{"error", authz.Allow(), errors.New("lookup failed"), http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer resetV1Registry()()
			hit := false
			RegisterRoutes("things", func(r *APIRouter, _ ServicesInterface) {
				r.Resource("things").HandleFunc("/things", func(w http.ResponseWriter, _ *http.Request) { hit = true }).Methods(http.MethodGet)
			})
			v1 := mux.NewRouter()
			LoadDiscoveredRoutes(v1, nil, &recordingAuthorizer{decision: tc.decision, err: tc.err})
			if code := serve(v1, http.MethodGet, "/things"); code != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, code)
			}
			if hit {
				t.Fatal("handler must not run when the request is not allowed")
			}
		})
	}
}

func TestUndeclaredRoute_FailsClosed(t *testing.T) {
	defer resetV1Registry()()
	hit := false
	RegisterRoutes("sneaky", func(r *APIRouter, _ ServicesInterface) {
		// Registered without Resource or Public.
		r.router.HandleFunc("/sneaky", func(w http.ResponseWriter, _ *http.Request) { hit = true })
	})
	a := &recordingAuthorizer{decision: authz.Allow()}
	v1 := mux.NewRouter()
	LoadDiscoveredRoutes(v1, nil, a)

	if code := serve(v1, http.MethodGet, "/sneaky"); code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", code)
	}
	if hit || len(a.calls) != 0 {
		t.Fatal("undeclared route must not reach the handler or the authorizer")
	}
}

func TestPublicRoute_SkipsAuthorize(t *testing.T) {
	defer resetV1Registry()()
	RegisterRoutes("open", func(r *APIRouter, _ ServicesInterface) {
		r.Public().HandleFunc("/open", okHandler)
	})
	a := &recordingAuthorizer{decision: authz.Deny("no")}
	v1 := mux.NewRouter()
	LoadDiscoveredRoutes(v1, nil, a)

	if code := serve(v1, http.MethodGet, "/open"); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(a.calls) != 0 {
		t.Fatal("public route must not call Authorize")
	}
}

func TestPrefixedRoute_SurfaceIsPrefix(t *testing.T) {
	defer resetPrefixedRegistry()()
	RegisterPrefixedRoutes("things", "ext", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("things").HandleFunc("/things", okHandler).Methods(http.MethodGet)
	})
	a := &recordingAuthorizer{decision: authz.Allow()}
	apiRouter := mux.NewRouter().PathPrefix("/api/rh-trex-ai").Subrouter()
	LoadDiscoveredPrefixedRoutes(apiRouter, nil, a, noopMiddleware)

	if code := serve(apiRouter, http.MethodGet, "/api/rh-trex-ai/ext/things"); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(a.calls) != 1 || a.calls[0].Resource.Surface != "ext" || a.calls[0].Action != authz.ActionList {
		t.Fatalf("unexpected requests: %+v", a.calls)
	}
}

func TestTwoResources_MethodRoutingStillWorks(t *testing.T) {
	defer resetV1Registry()()
	RegisterRoutes("a", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("as").PathPrefix("/as").Subrouter().HandleFunc("/{id}", okHandler).Methods(http.MethodGet)
	})
	RegisterRoutes("b", func(r *APIRouter, _ ServicesInterface) {
		r.Resource("bs").PathPrefix("/bs").Subrouter().HandleFunc("/{id}", okHandler).Methods(http.MethodDelete)
	})
	a := &recordingAuthorizer{decision: authz.Allow()}
	v1 := mux.NewRouter()
	LoadDiscoveredRoutes(v1, nil, a)

	if code := serve(v1, http.MethodGet, "/as/1"); code != http.StatusOK {
		t.Fatalf("GET /as/1: expected 200, got %d", code)
	}
	if code := serve(v1, http.MethodDelete, "/bs/1"); code != http.StatusOK {
		t.Fatalf("DELETE /bs/1: expected 200, got %d", code)
	}
	if code := serve(v1, http.MethodGet, "/nothing"); code == http.StatusOK {
		t.Fatal("unknown path must not succeed")
	}
	if a.calls[0].Resource.Kind != "as" || a.calls[1].Resource.Kind != "bs" {
		t.Fatalf("kinds not routed to their resource: %+v", a.calls)
	}
}
