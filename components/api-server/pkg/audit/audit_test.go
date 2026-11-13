package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// capture is an in-memory replacement for the audit table.
type capture struct {
	mu   sync.Mutex
	rows []*row
	err  error
}

func (c *capture) insert(_ context.Context, r *row) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.rows = append(c.rows, r)
	return nil
}

func (c *capture) outcomes() []string {
	var out []string
	for _, r := range c.rows {
		out = append(out, r.Outcome)
	}
	return out
}

func newTestService(t *testing.T, reads bool) (*Service, *capture) {
	t.Helper()
	cfg := config.NewAuditConfig()
	cfg.Sink = config.AuditSinkNone
	cfg.Reads = reads
	s, err := New(nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	c := &capture{}
	s.insert = c.insert
	return s, c
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
	sub.Use(s.HTTPRecord)
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
	s, c := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := do(router, http.MethodDelete, "/api/x/v1/dinosaurs/d-1", "alice")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if len(c.rows) != 1 {
		t.Fatalf("want 1 event, got %v", c.outcomes())
	}
	r := c.rows[0]
	if r.Outcome != OutcomeSuccess || r.Action != "delete" || r.ActorType != ActorUser || r.ActorUsername != "alice" ||
		r.ResourceKind != "dinosaurs" || r.ResourceID != "d-1" || r.Surface != "v1" || r.Transport != TransportHTTP ||
		r.HTTPMethod != http.MethodDelete || r.HTTPPath != "/api/x/v1/dinosaurs/d-1" || r.HTTPStatus != 0 || r.EventID == "" {
		t.Fatalf("unexpected event: %+v", r)
	}
}

func TestHTTPHandlerFailureRecordsFailure(t *testing.T) {
	s, c := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})
	do(router, http.MethodPatch, "/api/x/v1/dinosaurs/d-1", "alice")
	// The success event is written in the request transaction (rolled back in
	// production); the failure event is written outside it.
	if got := c.outcomes(); len(got) != 2 || got[0] != OutcomeSuccess || got[1] != OutcomeFailure {
		t.Fatalf("want [success failure], got %v", got)
	}
	if c.rows[1].HTTPStatus != http.StatusConflict || c.rows[1].ActorUsername != "alice" {
		t.Fatalf("unexpected failure event: %+v", c.rows[1])
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
			s, c := newTestService(t, false)
			called := false
			a := fakeAuthorizer{func(authz.Request) (authz.Decision, error) { return tc.decision, tc.err }}
			router := newRouter(s, a, func(http.ResponseWriter, *http.Request) { called = true })
			rec := do(router, http.MethodGet, "/api/x/v1/dinosaurs/d-1", "bob")
			if rec.Code != tc.wantStatus || called {
				t.Fatalf("status %d called %v", rec.Code, called)
			}
			if len(c.rows) != 1 || c.rows[0].Outcome != tc.outcome || c.rows[0].Action != "get" ||
				c.rows[0].ActorUsername != "bob" || c.rows[0].HTTPStatus != tc.wantStatus {
				t.Fatalf("unexpected events: %+v", c.rows)
			}
		})
	}
}

func TestHTTPUnauthenticatedIsStreamOnly(t *testing.T) {
	s, c := newTestService(t, false)
	var sink bytes.Buffer
	s.sink = NewWriterSink(&sink)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(http.ResponseWriter, *http.Request) {})
	rec := do(router, http.MethodGet, "/api/x/v1/dinosaurs", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	if len(c.rows) != 0 {
		t.Fatalf("unauthenticated requests must not be stored: %+v", c.rows)
	}
	var l map[string]any
	if err := json.Unmarshal(sink.Bytes(), &l); err != nil {
		t.Fatalf("want one JSON line on the sink, got %q: %v", sink.String(), err)
	}
	if l["outcome"] != OutcomeUnauthenticated || l["chain_seq"].(float64) != 0 || l["hash"] != nil ||
		l["actor"].(map[string]any)["type"] != ActorAnonymous || l["http"].(map[string]any)["status"].(float64) != 401 {
		t.Fatalf("unexpected stream-only line: %s", sink.String())
	}
}

func TestHTTPReads(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

	s, c := newTestService(t, false)
	do(newRouter(s, fakeAuthorizer{allowAll}, handler), http.MethodGet, "/api/x/v1/dinosaurs", "alice")
	if len(c.rows) != 0 {
		t.Fatalf("allowed read must not be recorded by default: %v", c.outcomes())
	}

	s, c = newTestService(t, true)
	do(newRouter(s, fakeAuthorizer{allowAll}, handler), http.MethodGet, "/api/x/v1/dinosaurs", "alice")
	if len(c.rows) != 1 || c.rows[0].Action != "list" || c.rows[0].Outcome != OutcomeSuccess {
		t.Fatalf("read must be recorded with --audit-reads: %+v", c.rows)
	}

	RecordReads("test_secrets")
	s, c = newTestService(t, false)
	do(newKindRouter(s, fakeAuthorizer{allowAll}, "test_secrets", handler), http.MethodGet, "/api/x/v1/test_secrets/s-1", "alice")
	do(newRouter(s, fakeAuthorizer{allowAll}, handler), http.MethodGet, "/api/x/v1/dinosaurs/d-1", "alice")
	if len(c.rows) != 1 || c.rows[0].ResourceKind != "test_secrets" || c.rows[0].Action != "get" {
		t.Fatalf("only reads of opted-in kinds must be recorded: %+v", c.rows)
	}
}

func TestHTTPAuditWriteFailureRefusesRequest(t *testing.T) {
	s, c := newTestService(t, false)
	c.err = errors.New("database down")
	called := false
	router := newRouter(s, fakeAuthorizer{allowAll}, func(http.ResponseWriter, *http.Request) { called = true })
	rec := do(router, http.MethodPost, "/api/x/v1/dinosaurs", "alice")
	if rec.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("status %d called %v; the handler must not run without an audit event", rec.Code, called)
	}
}

func TestHTTPEventHasNoCredentials(t *testing.T) {
	s, c := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/x/v1/dinosaurs?token=query-secret",
		strings.NewReader(`{"password":"body-secret"}`))
	req.Header.Set("X-User", "alice")
	req.Header.Set("Authorization", "Bearer header-secret")
	req.Header.Set("Cookie", "session=cookie-secret")
	router.ServeHTTP(httptest.NewRecorder(), req)
	if len(c.rows) != 1 {
		t.Fatalf("want 1 event, got %v", c.outcomes())
	}
	b, _ := json.Marshal(c.rows[0])
	for _, secret := range []string{"query-secret", "body-secret", "header-secret", "cookie-secret", "token="} {
		if bytes.Contains(b, []byte(secret)) {
			t.Fatalf("event contains %q: %s", secret, b)
		}
	}
}

func TestHTTPUndeclaredRoute(t *testing.T) {
	s, c := newTestService(t, false)
	router := mux.NewRouter()
	v1 := router.PathPrefix("/api/x/v1").Subrouter()
	v1.Use(s.HTTPBegin)
	v1.Use(fakeAuthn)
	v1.Handle("/stray", authz.UndeclaredHandler("/stray"))
	do(router, http.MethodPost, "/api/x/v1/stray", "alice")
	if len(c.rows) != 1 || c.rows[0].Outcome != OutcomeDenied || c.rows[0].HTTPStatus != http.StatusForbidden {
		t.Fatalf("unexpected events: %+v", c.rows)
	}
}

func TestDisabledServiceRecordsNothing(t *testing.T) {
	cfg := config.NewAuditConfig()
	cfg.Enabled = false
	s, err := New(nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Record(context.Background(), Event{Action: "x"}); err != nil {
		t.Fatalf("disabled Record must return nil, got %v", err)
	}
	var nilService *Service
	if err := nilService.Record(context.Background(), Event{}); err != nil {
		t.Fatal(err)
	}
	h := nilService.HTTPBegin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatal("a nil service must pass requests through")
	}
}

func TestApplicationEventValidation(t *testing.T) {
	s, c := newTestService(t, false)
	for _, e := range []Event{
		{Outcome: OutcomeSuccess},
		{Action: "x", Outcome: "maybe"},
		{Action: "x", Outcome: OutcomeSuccess, Actor: Actor{Type: "robot"}},
	} {
		if err := s.Record(context.Background(), e); err == nil {
			t.Fatalf("event %+v must be rejected", e)
		}
	}
	if len(c.rows) != 0 {
		t.Fatal("invalid events must not be stored")
	}
}

func TestApplicationEventDefaults(t *testing.T) {
	s, c := newTestService(t, false)
	if err := s.Record(context.Background(), Event{Action: "reconcile", Outcome: OutcomeSuccess, Resource: Resource{Kind: "dinosaurs"}}); err != nil {
		t.Fatal(err)
	}
	r := c.rows[0]
	if r.ActorType != ActorSystem || r.Transport != TransportInternal || r.EventID == "" || r.OccurredAt.IsZero() {
		t.Fatalf("defaults not filled: %+v", r)
	}
	ctx := auth.SetUsernameContext(context.Background(), "carol")
	if err := s.Record(ctx, Event{Action: "approve", Outcome: OutcomeSuccess}); err != nil {
		t.Fatal(err)
	}
	if c.rows[1].ActorType != ActorUser || c.rows[1].ActorUsername != "carol" {
		t.Fatalf("actor not taken from context: %+v", c.rows[1])
	}
}

// grpcChain mirrors the order TRex uses: begin, authentication, authorization, record.
func grpcChain(s *Service, authn grpc.UnaryServerInterceptor, a authz.Authorizer) grpc.UnaryServerInterceptor {
	lookup := func(string) (authz.GRPCResource, bool) {
		return authz.GRPCResource{Kind: "dinosaurs", Action: "delete"}, true
	}
	chain := []grpc.UnaryServerInterceptor{s.UnaryBegin(nil), authn, authz.UnaryInterceptor(a, lookup, nil), s.UnaryRecord()}
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

	cases := []struct {
		name     string
		user     string
		decide   func(authz.Request) (authz.Decision, error)
		handler  grpc.UnaryHandler
		outcomes []string
		code     string
	}{
		{"success", "alice", allowAll, ok, []string{OutcomeSuccess}, ""},
		{"failure", "alice", allowAll, fail, []string{OutcomeSuccess, OutcomeFailure}, "NotFound"},
		{"denied", "alice", func(authz.Request) (authz.Decision, error) { return authz.Deny("no"), nil }, ok, []string{OutcomeDenied}, "PermissionDenied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, c := newTestService(t, false)
			_, _ = grpcChain(s, grpcAuthn(tc.user), fakeAuthorizer{tc.decide})(context.Background(), "req", info, tc.handler)
			got := c.outcomes()
			if strings.Join(got, ",") != strings.Join(tc.outcomes, ",") {
				t.Fatalf("outcomes %v, want %v", got, tc.outcomes)
			}
			last := c.rows[len(c.rows)-1]
			if last.Transport != TransportGRPC || last.GRPCMethod != info.FullMethod || last.GRPCCode != tc.code {
				t.Fatalf("unexpected event: %+v", last)
			}
		})
	}
}

func TestGRPCUnauthenticatedIsStreamOnly(t *testing.T) {
	s, c := newTestService(t, false)
	var sink bytes.Buffer
	s.sink = NewWriterSink(&sink)
	info := &grpc.UnaryServerInfo{FullMethod: "/rh_trex.v1.DinosaurService/DeleteDinosaur"}
	_, _ = grpcChain(s, grpcAuthn(""), fakeAuthorizer{allowAll})(context.Background(), "req", info,
		func(context.Context, interface{}) (interface{}, error) { return nil, nil })
	if len(c.rows) != 0 || !strings.Contains(sink.String(), `"outcome":"unauthenticated"`) ||
		!strings.Contains(sink.String(), `"code":"Unauthenticated"`) {
		t.Fatalf("rows %+v sink %q", c.rows, sink.String())
	}
}

func TestGRPCAuditWriteFailureRefusesCall(t *testing.T) {
	s, c := newTestService(t, false)
	c.err = errors.New("database down")
	called := false
	info := &grpc.UnaryServerInfo{FullMethod: "/rh_trex.v1.DinosaurService/DeleteDinosaur"}
	_, err := grpcChain(s, grpcAuthn("alice"), fakeAuthorizer{allowAll})(context.Background(), "req", info,
		func(context.Context, interface{}) (interface{}, error) { called = true; return nil, nil })
	if status.Code(err) != codes.Unavailable || called {
		t.Fatalf("code %v called %v", status.Code(err), called)
	}
}

func sampleRow() *row {
	return &row{
		EventID: "e1", OccurredAt: time.Date(2026, 11, 12, 10, 0, 0, 123456789, time.UTC),
		ActorType: ActorUser, ActorUsername: "alice", Action: "delete", Outcome: OutcomeSuccess,
		Transport: TransportHTTP, Surface: "v1", ResourceKind: "dinosaurs", ResourceID: "d-1",
		ResourceParent: `{"z":"1","a":"<2>"}`, HTTPMethod: "DELETE", HTTPPath: "/x",
	}
}

func TestCanonicalEncoding(t *testing.T) {
	l, err := lineFor(sampleRow(), 7, "none")
	if err != nil {
		t.Fatal(err)
	}
	b, err := l.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, `{"schema_version":1,"chain_seq":7,"event_id":"e1","occurred_at":"2026-11-12T10:00:00.123456Z"`) {
		t.Fatalf("unexpected canonical prefix: %s", s)
	}
	if !strings.Contains(s, `"parent":{"a":"<2>","z":"1"}`) || !strings.Contains(s, `"details":{}`) {
		t.Fatalf("maps must be sorted, unescaped, and never null: %s", s)
	}
	if strings.Contains(s, "prev_hash") || strings.Contains(s, `"hash"`) {
		t.Fatalf("canonical form must omit the seal hashes: %s", s)
	}
	again, _ := l.Canonical()
	if !bytes.Equal(b, again) {
		t.Fatal("canonical encoding must be deterministic")
	}
}

func TestComputeHash(t *testing.T) {
	l, _ := lineFor(sampleRow(), 1, "none")
	plain, _ := ComputeHash(nil, GenesisHash, l)
	key := []byte(strings.Repeat("k", 32))
	keyed, _ := ComputeHash(key, GenesisHash, l)
	otherPrev, _ := ComputeHash(key, strings.Repeat("1", 64), l)
	if len(plain) != 64 || plain == keyed || keyed == otherPrev {
		t.Fatalf("hashes must differ by key and prev: %s %s %s", plain, keyed, otherPrev)
	}
	r := sampleRow()
	r.ActorUsername = "mallory"
	changed, _ := lineFor(r, 1, "none")
	if h, _ := ComputeHash(nil, GenesisHash, changed); h == plain {
		t.Fatal("changing a field must change the hash")
	}
	if KeyID(nil) != KeyIDNone || len(KeyID(key)) != 16 || KeyID(key) == KeyID([]byte(strings.Repeat("j", 32))) {
		t.Fatal("unexpected key IDs")
	}
}

func TestExportLine(t *testing.T) {
	r := sampleRow()
	seq, prev, hash, key := int64(3), GenesisHash, strings.Repeat("a", 64), KeyIDNone
	r.ChainSeq, r.PrevHash, r.Hash, r.KeyID = &seq, &prev, &hash, &key
	b, err := exportLine(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(b, []byte("\n")) || bytes.Count(b, []byte("\n")) != 1 {
		t.Fatalf("want exactly one line: %q", b)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"].(float64) != 1 || got["chain_seq"].(float64) != 3 || got["hash"] != hash || got["prev_hash"] != prev || got["event_id"] != "e1" {
		t.Fatalf("unexpected line: %s", b)
	}
	// A SIEM can recompute the hash from the line itself.
	var l Line
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	want, _ := lineFor(r, seq, key)
	wantCanon, _ := want.Canonical()
	gotCanon, _ := l.Canonical()
	if !bytes.Equal(wantCanon, gotCanon) {
		t.Fatalf("line does not round-trip to the canonical form:\n%s\n%s", wantCanon, gotCanon)
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

func TestHostileBytesAreStorable(t *testing.T) {
	s, c := newTestService(t, false)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(http.ResponseWriter, *http.Request) {})
	req := httptest.NewRequest(http.MethodPost, "/api/x/v1/dinosaurs", nil)
	req.Header.Set("X-User", "alice")
	req.Header.Set("User-Agent", "bad\xff\x00agent"+strings.Repeat("a", 5000))
	router.ServeHTTP(httptest.NewRecorder(), req)
	if len(c.rows) != 1 {
		t.Fatalf("want 1 event, got %v", c.outcomes())
	}
	ua := c.rows[0].UserAgent
	if !utf8.ValidString(ua) || strings.ContainsRune(ua, 0) || len(ua) > maxFieldBytes {
		t.Fatalf("user agent not sanitized: %q", ua[:40])
	}
	r, err := toRow(Event{Action: "x", Outcome: OutcomeSuccess, Actor: Actor{Type: ActorSystem},
		Details: map[string]string{"k\x00": strings.Repeat("\xff", 3000)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeMap(r.Details); err != nil || !utf8.ValidString(r.Details) {
		t.Fatalf("details must stay valid JSON and UTF-8: %v", err)
	}
}

func TestUndeclaredRouteKeepsActor(t *testing.T) {
	s, c := newTestService(t, false)
	router := mux.NewRouter()
	v1 := router.PathPrefix("/api/x/v1").Subrouter()
	v1.Use(s.HTTPBegin)
	v1.Use(fakeAuthn)
	v1.Handle("/stray", authz.UndeclaredHandler("/stray"))
	do(router, http.MethodDelete, "/api/x/v1/stray", "alice")
	if len(c.rows) != 1 || c.rows[0].ActorUsername != "alice" || c.rows[0].Action != "delete" {
		t.Fatalf("unexpected events: %+v", c.rows)
	}
}

func TestPublicRouteSurface(t *testing.T) {
	s, c := newTestService(t, false)
	router := mux.NewRouter()
	ext := router.PathPrefix("/api/x/ext").Subrouter()
	ext.Use(s.HTTPBegin)
	ext.Use(fakeAuthn)
	pub := ext.NewRoute().Subrouter()
	pub.Use(authz.HTTPPublicSurfaceMiddleware(fakeAuthorizer{allowAll}, "ext"))
	pub.Use(s.HTTPRecord)
	pub.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	do(router, http.MethodPost, "/api/x/ext/ping", "alice")
	if len(c.rows) != 1 || c.rows[0].Surface != "ext" || c.rows[0].Action != "create" {
		t.Fatalf("unexpected events: %+v", c.rows)
	}
}

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeStream) Context() context.Context { return f.ctx }

func TestStreamClosedByClientIsSuccess(t *testing.T) {
	s, c := newTestService(t, true)
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
	if len(c.rows) != 1 || c.rows[0].Outcome != OutcomeSuccess || c.rows[0].Action != "watch" {
		t.Fatalf("a client-closed stream must be a success: %+v", c.rows)
	}
}
