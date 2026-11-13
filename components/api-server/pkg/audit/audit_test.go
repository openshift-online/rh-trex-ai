package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

func newTestService(t *testing.T, reads bool) (*Service, *MemoryRecorder) {
	t.Helper()
	cfg := config.NewAuditConfig()
	cfg.Reads = reads
	rec := NewMemoryRecorder()
	return New(cfg, rec), rec
}

func outcomes(rec *MemoryRecorder) []string {
	var out []string
	for _, e := range rec.Events() {
		out = append(out, e.Outcome)
	}
	return out
}

type fakeAuthorizer struct {
	decide func(authz.Request) (authz.Decision, error)
}

func (f fakeAuthorizer) Authorize(_ context.Context, req authz.Request) (authz.Decision, error) {
	return f.decide(req)
}

func allowAll(authz.Request) (authz.Decision, error) { return authz.Allow(), nil }

// fakeAuthn sets the username from X-User, or responds 401 without it.
func fakeAuthn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-User")
		if user == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.SetUsernameContext(r.Context(), user)))
	})
}

func newRouter(s *Service, a authz.Authorizer, handler http.HandlerFunc) *mux.Router {
	return newKindRouter(s, a, "dinosaurs", handler)
}

func newKindRouter(s *Service, a authz.Authorizer, kind string, handler http.HandlerFunc) *mux.Router {
	router := mux.NewRouter()
	v1 := router.PathPrefix("/api/x/v1").Subrouter()
	v1.Use(s.HTTPBegin)
	v1.Use(fakeAuthn)
	sub := v1.NewRoute().Subrouter()
	sub.Use(authz.HTTPMiddleware(a, authz.SurfaceV1, kind))
	sub.HandleFunc("/"+kind, handler)
	sub.HandleFunc("/"+kind+"/{id}", handler)
	return router
}

func do(router http.Handler, method, path, user string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if user != "" {
		req.Header.Set("X-User", user)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestHTTPSuccessfulMutation(t *testing.T) {
	s, rec := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	do(router, http.MethodDelete, "/api/x/v1/dinosaurs/d-1", "alice")
	events := rec.Events()
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %v", outcomes(rec))
	}
	e := events[0]
	if e.Outcome != OutcomeSuccess || e.Action != "delete" || e.Actor.Type != ActorUser || e.Actor.Username != "alice" ||
		e.Resource.Kind != "dinosaurs" || e.Resource.ID != "d-1" || e.Resource.Surface != "v1" || e.Transport != TransportHTTP ||
		e.HTTP.Method != http.MethodDelete || e.HTTP.Path != "/api/x/v1/dinosaurs/d-1" || e.HTTP.Status != http.StatusNoContent ||
		e.EventID == "" || e.OccurredAt.IsZero() {
		t.Fatalf("unexpected event: %+v", e)
	}
}

func TestHTTPHandlerFailure(t *testing.T) {
	s, rec := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})
	do(router, http.MethodPatch, "/api/x/v1/dinosaurs/d-1", "alice")
	events := rec.Events()
	if len(events) != 1 || events[0].Outcome != OutcomeFailure || events[0].HTTP.Status != http.StatusConflict || events[0].Actor.Username != "alice" {
		t.Fatalf("want one failure event, got %+v", events)
	}
}

func TestHTTPDenials(t *testing.T) {
	cases := []struct {
		name       string
		decision   authz.Decision
		err        error
		wantStatus int
		outcome    string
	}{
		{"denied", authz.Deny("no"), nil, http.StatusForbidden, OutcomeDenied},
		{"hidden", authz.Decision{HideExistence: true}, nil, http.StatusNotFound, OutcomeDenied},
		{"error", authz.Decision{}, errors.New("policy store down"), http.StatusServiceUnavailable, OutcomeError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec := newTestService(t, false)
			called := false
			a := fakeAuthorizer{func(authz.Request) (authz.Decision, error) { return tc.decision, tc.err }}
			router := newRouter(s, a, func(http.ResponseWriter, *http.Request) { called = true })
			resp := do(router, http.MethodGet, "/api/x/v1/dinosaurs/d-1", "bob")
			events := rec.Events()
			if resp.Code != tc.wantStatus || called {
				t.Fatalf("status %d called %v", resp.Code, called)
			}
			if len(events) != 1 || events[0].Outcome != tc.outcome || events[0].Action != "get" ||
				events[0].Actor.Username != "bob" || events[0].HTTP.Status != tc.wantStatus {
				t.Fatalf("unexpected events: %+v", events)
			}
		})
	}
}

func TestHTTPUnauthenticated(t *testing.T) {
	s, rec := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(http.ResponseWriter, *http.Request) {})
	do(router, http.MethodGet, "/api/x/v1/dinosaurs", "")
	events := rec.Events()
	if len(events) != 1 || events[0].Outcome != OutcomeUnauthenticated || events[0].Actor.Type != ActorAnonymous ||
		events[0].HTTP.Status != http.StatusUnauthorized {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestHTTPAuthenticationOuterHandler(t *testing.T) {
	s, rec := newTestService(t, false)
	refuse := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	h := s.HTTPAuthentication("/api/x/")(refuse)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/x/v1/dinosaurs", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/elsewhere", nil))
	if got := outcomes(rec); len(got) != 1 || got[0] != OutcomeUnauthenticated {
		t.Fatalf("want one unauthenticated event under the API prefix, got %v", got)
	}

	// A request that reached HTTPBegin is recorded there, not twice.
	rec.Reset()
	router := newRouter(s, fakeAuthorizer{allowAll}, func(http.ResponseWriter, *http.Request) {})
	s.HTTPAuthentication("/api/x/")(router).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/x/v1/dinosaurs", nil))
	if got := outcomes(rec); len(got) != 1 {
		t.Fatalf("want exactly one event, got %v", got)
	}
}

func TestHTTPReads(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

	s, rec := newTestService(t, false)
	do(newRouter(s, fakeAuthorizer{allowAll}, handler), http.MethodGet, "/api/x/v1/dinosaurs", "alice")
	if len(rec.Events()) != 0 {
		t.Fatalf("allowed read must not be recorded by default: %v", outcomes(rec))
	}

	s, rec = newTestService(t, true)
	do(newRouter(s, fakeAuthorizer{allowAll}, handler), http.MethodGet, "/api/x/v1/dinosaurs", "alice")
	if events := rec.Events(); len(events) != 1 || events[0].Action != "list" || events[0].Outcome != OutcomeSuccess {
		t.Fatalf("read must be recorded with --audit-reads: %+v", events)
	}

	RecordReads("test_secrets")
	s, rec = newTestService(t, false)
	do(newKindRouter(s, fakeAuthorizer{allowAll}, "test_secrets", handler), http.MethodGet, "/api/x/v1/test_secrets/s-1", "alice")
	do(newRouter(s, fakeAuthorizer{allowAll}, handler), http.MethodGet, "/api/x/v1/dinosaurs/d-1", "alice")
	if events := rec.Events(); len(events) != 1 || events[0].Resource.Kind != "test_secrets" || events[0].Action != "get" {
		t.Fatalf("only reads of opted-in kinds must be recorded: %+v", events)
	}
}

type failingRecorder struct{}

func (failingRecorder) Record(context.Context, Event) error { return errors.New("sink down") }

func TestRecorderFailureDoesNotChangeResponse(t *testing.T) {
	s := New(config.NewAuditConfig(), failingRecorder{})
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	if resp := do(router, http.MethodPost, "/api/x/v1/dinosaurs", "alice"); resp.Code != http.StatusCreated {
		t.Fatalf("status %d; a recorder failure must not change the response", resp.Code)
	}
	if err := s.Record(context.Background(), Event{Action: "x", Outcome: OutcomeSuccess}); err == nil {
		t.Fatal("audit.Record must return the recorder error to the caller")
	}
}

func TestHTTPEventHasNoCredentials(t *testing.T) {
	var out bytes.Buffer
	s := New(config.NewAuditConfig(), NewJSONRecorder(&out))
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/x/v1/dinosaurs?token=query-secret",
		strings.NewReader(`{"password":"body-secret"}`))
	req.Header.Set("X-User", "alice")
	req.Header.Set("Authorization", "Bearer header-secret")
	req.Header.Set("Cookie", "session=cookie-secret")
	router.ServeHTTP(httptest.NewRecorder(), req)
	if out.Len() == 0 {
		t.Fatal("want one line")
	}
	for _, secret := range []string{"query-secret", "body-secret", "header-secret", "cookie-secret", "token="} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("event contains %q: %s", secret, out.String())
		}
	}
}

func TestHTTPUndeclaredRouteKeepsActor(t *testing.T) {
	s, rec := newTestService(t, false)
	router := mux.NewRouter()
	v1 := router.PathPrefix("/api/x/v1").Subrouter()
	v1.Use(s.HTTPBegin)
	v1.Use(fakeAuthn)
	v1.Handle("/stray", authz.UndeclaredHandler("/stray"))
	do(router, http.MethodDelete, "/api/x/v1/stray", "alice")
	events := rec.Events()
	if len(events) != 1 || events[0].Outcome != OutcomeDenied || events[0].HTTP.Status != http.StatusForbidden ||
		events[0].Actor.Username != "alice" || events[0].Action != "delete" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestPublicRouteSurface(t *testing.T) {
	s, rec := newTestService(t, false)
	router := mux.NewRouter()
	ext := router.PathPrefix("/api/x/ext").Subrouter()
	ext.Use(s.HTTPBegin)
	ext.Use(fakeAuthn)
	pub := ext.NewRoute().Subrouter()
	pub.Use(authz.HTTPPublicSurfaceMiddleware(fakeAuthorizer{allowAll}, "ext"))
	pub.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	do(router, http.MethodPost, "/api/x/ext/ping", "alice")
	if events := rec.Events(); len(events) != 1 || events[0].Resource.Surface != "ext" || events[0].Action != "create" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestDisabledServiceRecordsNothing(t *testing.T) {
	cfg := config.NewAuditConfig()
	cfg.Enabled = false
	rec := NewMemoryRecorder()
	s := New(cfg, rec)
	if err := s.Record(context.Background(), Event{Action: "x", Outcome: OutcomeSuccess}); err != nil || len(rec.Events()) != 0 {
		t.Fatalf("disabled Record must return nil and record nothing, got %v", err)
	}
	var nilService *Service
	if err := nilService.Record(context.Background(), Event{}); err != nil {
		t.Fatal(err)
	}
	h := nilService.HTTPBegin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/", nil))
	if resp.Code != http.StatusTeapot {
		t.Fatal("a nil service must pass requests through")
	}
}

func TestApplicationEvents(t *testing.T) {
	s, rec := newTestService(t, false)
	if err := s.Record(context.Background(), Event{Action: "reconcile", Outcome: OutcomeSuccess, Resource: Resource{Kind: "dinosaurs"}}); err != nil {
		t.Fatal(err)
	}
	ctx := auth.SetUsernameContext(context.Background(), "carol")
	if err := s.Record(ctx, Event{Action: "approve", Outcome: OutcomeSuccess}); err != nil {
		t.Fatal(err)
	}
	events := rec.Events()
	if events[0].Actor.Type != ActorSystem || events[0].Transport != TransportInternal || events[0].EventID == "" {
		t.Fatalf("defaults not filled: %+v", events[0])
	}
	if events[1].Actor.Type != ActorUser || events[1].Actor.Username != "carol" {
		t.Fatalf("actor not taken from context: %+v", events[1])
	}
	for _, bad := range []Event{
		{Outcome: OutcomeSuccess},
		{Action: "x", Outcome: "maybe"},
		{Action: "x", Outcome: OutcomeSuccess, Actor: Actor{Type: "robot"}},
	} {
		if err := s.Record(context.Background(), bad); err == nil {
			t.Fatalf("event %+v must be rejected", bad)
		}
	}
	if len(rec.Events()) != 2 {
		t.Fatal("invalid events must not be recorded")
	}
}

// grpcChain mirrors the order TRex uses: begin, authentication, authorization.
func grpcChain(s *Service, authn grpc.UnaryServerInterceptor, a authz.Authorizer, action string) grpc.UnaryServerInterceptor {
	lookup := func(string) (authz.GRPCResource, bool) {
		return authz.GRPCResource{Kind: "dinosaurs", Action: action}, true
	}
	chain := []grpc.UnaryServerInterceptor{s.UnaryBegin(nil), authn, authz.UnaryInterceptor(a, lookup, nil)}
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var call func(i int, ctx context.Context, req interface{}) (interface{}, error)
		call = func(i int, ctx context.Context, req interface{}) (interface{}, error) {
			if i == len(chain) {
				return handler(ctx, req)
			}
			return chain[i](ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
				return call(i+1, ctx, req)
			})
		}
		return call(0, ctx, req)
	}
}

func grpcAuthn(user string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if user == "" {
			return nil, status.Error(codes.Unauthenticated, "no token")
		}
		return handler(auth.SetUsernameContext(ctx, user), req)
	}
}

func TestGRPCOutcomes(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/rh_trex.v1.DinosaurService/DeleteDinosaur"}
	ok := func(context.Context, interface{}) (interface{}, error) { return "ok", nil }
	fail := func(context.Context, interface{}) (interface{}, error) {
		return nil, status.Error(codes.NotFound, "gone")
	}
	deny := func(authz.Request) (authz.Decision, error) { return authz.Deny("no"), nil }

	cases := []struct {
		name    string
		user    string
		decide  func(authz.Request) (authz.Decision, error)
		handler grpc.UnaryHandler
		outcome string
		code    string
	}{
		{"success", "alice", allowAll, ok, OutcomeSuccess, "OK"},
		{"failure", "alice", allowAll, fail, OutcomeFailure, "NotFound"},
		{"denied", "alice", deny, ok, OutcomeDenied, "PermissionDenied"},
		{"unauthenticated", "", allowAll, ok, OutcomeUnauthenticated, "Unauthenticated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, rec := newTestService(t, false)
			_, _ = grpcChain(s, grpcAuthn(tc.user), fakeAuthorizer{tc.decide}, "delete")(context.Background(), "req", info, tc.handler)
			events := rec.Events()
			if len(events) != 1 || events[0].Outcome != tc.outcome || events[0].Transport != TransportGRPC ||
				events[0].GRPC.Method != info.FullMethod || events[0].GRPC.Code != tc.code {
				t.Fatalf("unexpected events: %+v", events)
			}
		})
	}
}

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeStream) Context() context.Context { return f.ctx }

func TestStreamClosedByClientIsSuccess(t *testing.T) {
	s, rec := newTestService(t, true)
	lookup := func(string) (authz.GRPCResource, bool) {
		return authz.GRPCResource{Kind: "dinosaurs", Action: "watch"}, true
	}
	authzStream := authz.StreamInterceptor(fakeAuthorizer{allowAll}, lookup, nil)
	info := &grpc.StreamServerInfo{FullMethod: "/rh_trex.v1.DinosaurService/WatchDinosaurs"}
	ctx, cancel := context.WithCancel(auth.SetUsernameContext(context.Background(), "alice"))
	cancel()
	err := s.StreamBegin(nil)(nil, fakeStream{ctx: ctx}, info, func(srv interface{}, ss grpc.ServerStream) error {
		return authzStream(srv, ss, info, func(interface{}, grpc.ServerStream) error { return ss.Context().Err() })
	})
	if err == nil {
		t.Fatal("expected the context error")
	}
	if events := rec.Events(); len(events) != 1 || events[0].Outcome != OutcomeSuccess || events[0].Action != "watch" {
		t.Fatalf("a client-closed stream must be a success: %+v", events)
	}
}

func TestJSONLine(t *testing.T) {
	e := Event{
		EventID: "e1", OccurredAt: time.Date(2026, 11, 12, 10, 0, 0, 123456789, time.UTC),
		Actor: Actor{Type: ActorUser, Username: "alice"}, Action: "delete", Outcome: OutcomeSuccess,
		Transport: TransportHTTP, Resource: Resource{Surface: "v1", Kind: "dinosaurs", ID: "d-1", Parent: map[string]string{"z": "1", "a": "<2>"}},
		HTTP: HTTPInfo{Method: "DELETE", Path: "/x", Status: 204},
	}
	b, err := MarshalLine(e)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, `{"schema_version":1,"event_id":"e1","occurred_at":"2026-11-12T10:00:00.123456Z"`) || !strings.HasSuffix(s, "}\n") {
		t.Fatalf("unexpected line: %s", s)
	}
	if !strings.Contains(s, `"parent":{"a":"<2>","z":"1"}`) || !strings.Contains(s, `"details":{}`) {
		t.Fatalf("maps must be sorted, unescaped, and never null: %s", s)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
}

func TestHostileBytesAreSanitized(t *testing.T) {
	b, err := MarshalLine(Event{
		Action: "x", Outcome: OutcomeSuccess, Actor: Actor{Type: ActorSystem},
		Source:  Source{UserAgent: "bad\xff\x00agent" + strings.Repeat("a", 5000)},
		Details: map[string]string{"k\x00": strings.Repeat("\xff", 3000)},
	})
	if err != nil {
		t.Fatal(err)
	}
	var l Line
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	ua := l.Source.UserAgent
	if !utf8.ValidString(ua) || strings.ContainsRune(ua, 0) || len(ua) > maxFieldBytes {
		t.Fatalf("user agent not sanitized: %q", ua[:40])
	}
	for k, v := range l.Details {
		if strings.ContainsRune(k, 0) || len(v) > maxFieldBytes {
			t.Fatalf("details not sanitized: %q", k)
		}
	}
}

func TestTraceIDFromHeader(t *testing.T) {
	if got := traceIDFromHeader("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("got %q", got)
	}
	for _, bad := range []string{"", "garbage", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", "00-XYZ92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"} {
		if traceIDFromHeader(bad) != "" {
			t.Fatalf("%q must not yield a trace ID", bad)
		}
	}
}
