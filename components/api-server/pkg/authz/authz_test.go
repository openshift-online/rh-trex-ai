package authz

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
)

type fakeAuthorizer struct {
	decide func(Request) (Decision, error)
	calls  []Request
}

func (f *fakeAuthorizer) Authorize(_ context.Context, req Request) (Decision, error) {
	f.calls = append(f.calls, req)
	return f.decide(req)
}

type userKey struct{}

type enrichingAuthorizer struct {
	fakeAuthorizer
	err error
}

func (e *enrichingAuthorizer) Enrich(ctx context.Context, s Subject) (context.Context, error) {
	if e.err != nil {
		return ctx, e.err
	}
	return context.WithValue(ctx, userKey{}, "local-"+s.Username), nil
}

func allow(Request) (Decision, error) { return Allow(), nil }

func TestPolicies(t *testing.T) {
	ctx := context.Background()
	d, _ := RequireIdentity().Authorize(ctx, Request{})
	if d.Allowed {
		t.Fatal("RequireIdentity must deny an empty username")
	}
	d, _ = RequireIdentity().Authorize(ctx, Request{Subject: Subject{Username: "u"}})
	if !d.Allowed {
		t.Fatal("RequireIdentity must allow a username")
	}
	d, _ = AllowAll().Authorize(ctx, Request{})
	if !d.Allowed {
		t.Fatal("AllowAll must allow")
	}
	if _, ok := FromFlag(true).(requireIdentity); !ok {
		t.Fatal("FromFlag(true) must be RequireIdentity")
	}
	if _, ok := FromFlag(false).(allowAll); !ok {
		t.Fatal("FromFlag(false) must be AllowAll")
	}
}

func TestHTTPMiddleware_EnrichReachesAuthorizeAndHandler(t *testing.T) {
	a := &enrichingAuthorizer{fakeAuthorizer: fakeAuthorizer{decide: allow}}
	var handlerUser any
	h := HTTPMiddleware(a, SurfaceV1, "things")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerUser = r.Context().Value(userKey{})
		if SubjectFromContext(r.Context()).Username != "alice" {
			t.Errorf("SubjectFromContext lost the subject")
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/things", nil)
	req = req.WithContext(auth.SetUsernameContext(req.Context(), "alice"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if handlerUser != "local-alice" {
		t.Fatalf("handler did not see enriched context, got %v", handlerUser)
	}
	if len(a.calls) != 1 || a.calls[0].Subject.Username != "alice" || a.calls[0].Action != ActionList {
		t.Fatalf("unexpected requests: %+v", a.calls)
	}
}

func TestHTTPMiddleware_EnrichErrorSkipsAuthorize(t *testing.T) {
	a := &enrichingAuthorizer{fakeAuthorizer: fakeAuthorizer{decide: allow}, err: errors.New("db down")}
	h := HTTPMiddleware(a, SurfaceV1, "things")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler must not run")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/things", nil))
	if rr.Code != http.StatusServiceUnavailable || len(a.calls) != 0 {
		t.Fatalf("expected 503 and no Authorize call, got %d and %d calls", rr.Code, len(a.calls))
	}
}

var table = CRUDResources("svc.v1.ThingService", "things", "Thing", "Things")

func lookup(m string) (GRPCResource, bool) { r, ok := table[m]; return r, ok }

func TestUnaryInterceptor(t *testing.T) {
	handler := func(ctx context.Context, req interface{}) (interface{}, error) { return "ok", nil }
	cases := []struct {
		name   string
		method string
		decide func(Request) (Decision, error)
		want   codes.Code
	}{
		{"allowed", "/svc.v1.ThingService/GetThing", allow, codes.OK},
		{"denied", "/svc.v1.ThingService/GetThing", func(Request) (Decision, error) { return Deny("no"), nil }, codes.PermissionDenied},
		{"hidden", "/svc.v1.ThingService/GetThing", func(Request) (Decision, error) { return Decision{HideExistence: true}, nil }, codes.NotFound},
		{"error", "/svc.v1.ThingService/GetThing", func(Request) (Decision, error) { return Allow(), errors.New("x") }, codes.Unavailable},
		{"undeclared", "/svc.v1.ThingService/Explode", allow, codes.PermissionDenied},
		{"bypass", "/grpc.health.v1.Health/Check", func(Request) (Decision, error) { return Deny("no"), nil }, codes.OK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &fakeAuthorizer{decide: tc.decide}
			i := UnaryInterceptor(a, lookup, []string{"/grpc.health.v1.Health/"})
			_, err := i(context.Background(), "msg", &grpc.UnaryServerInfo{FullMethod: tc.method}, handler)
			if status.Code(err) != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}

	a := &fakeAuthorizer{decide: allow}
	_, _ = UnaryInterceptor(a, lookup, nil)(context.Background(), "msg", &grpc.UnaryServerInfo{FullMethod: "/svc.v1.ThingService/ListThings"}, handler)
	if got := a.calls[0]; got.Resource.Kind != "things" || got.Action != ActionList || got.GRPCMsg != "msg" || got.GRPCMethod == "" {
		t.Fatalf("unexpected request: %+v", got)
	}
}

type fakeStream struct {
	grpc.ServerStream
	ctx  context.Context
	msgs []string
}

func (s *fakeStream) Context() context.Context { return s.ctx }

func (s *fakeStream) RecvMsg(m interface{}) error {
	if len(s.msgs) == 0 {
		return io.EOF
	}
	*(m.(*string)) = s.msgs[0]
	s.msgs = s.msgs[1:]
	return nil
}

func TestStreamInterceptor_ChecksOpenAndEachMessage(t *testing.T) {
	a := &fakeAuthorizer{decide: func(r Request) (Decision, error) {
		if r.GRPCMsg != nil && *(r.GRPCMsg.(*string)) == "forbidden" {
			return Deny("no"), nil
		}
		return Allow(), nil
	}}
	ss := &fakeStream{ctx: context.Background(), msgs: []string{"fine", "forbidden"}}
	var recvErr error
	err := StreamInterceptor(a, lookup, nil)(nil, ss, &grpc.StreamServerInfo{FullMethod: "/svc.v1.ThingService/WatchThings"},
		func(_ interface{}, s grpc.ServerStream) error {
			for {
				var m string
				if recvErr = s.RecvMsg(&m); recvErr != nil {
					return recvErr
				}
			}
		})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected the forbidden message to end the stream, got %v", err)
	}
	if len(a.calls) != 3 || a.calls[0].GRPCMsg != nil || a.calls[0].Action != ActionWatch {
		t.Fatalf("expected open + 2 message checks, got %+v", a.calls)
	}
}
