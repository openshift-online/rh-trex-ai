package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

// commitProbeTable has a deferred unique constraint: duplicate inserts succeed
// inside the transaction and fail only at COMMIT. That makes a real commit
// failure after the handler has already succeeded.
func createCommitProbeTable(t *testing.T, sf db.SessionFactory) {
	t.Helper()
	Expect(sf.New(context.Background()).Exec(`CREATE TABLE IF NOT EXISTS commit_probes (
		k text, CONSTRAINT commit_probes_k UNIQUE (k) DEFERRABLE INITIALLY DEFERRED)`).Error).To(Succeed())
	t.Cleanup(func() { _ = sf.New(context.Background()).Exec(`DROP TABLE IF EXISTS commit_probes`).Error })
}

func insertDuplicates(ctx context.Context, sf db.SessionFactory) error {
	return sf.New(ctx).Exec(`INSERT INTO commit_probes (k) VALUES ('dup'), ('dup')`).Error
}

func probeCount(sf db.SessionFactory) int64 {
	var n int64
	Expect(sf.New(context.Background()).Raw(`SELECT count(*) FROM commit_probes`).Scan(&n).Error).To(Succeed())
	return n
}

func TestAuditRecordsRESTCommitFailure(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)
	rec := audit.NewMemoryRecorder()
	svc := audit.New(config.NewAuditConfig(), rec)

	router := mux.NewRouter()
	v1 := router.PathPrefix("/api/x/v1").Subrouter()
	v1.Use(svc.HTTPBegin)
	v1.Use(func(next http.Handler) http.Handler { return db.TransactionMiddleware(next, sf) })
	sub := v1.NewRoute().Subrouter()
	sub.Use(authz.HTTPMiddleware(authz.AllowAll(), authz.SurfaceV1, "commit_probes"))
	sub.HandleFunc("/commit_probes", func(w http.ResponseWriter, r *http.Request) {
		if err := insertDuplicates(r.Context(), sf); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}).Methods(http.MethodPost)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/x/v1/commit_probes", nil))

	Expect(probeCount(sf)).To(BeZero(), "the commit must have failed")
	Expect(resp.Code).To(Equal(http.StatusInternalServerError), "the client must not be told success")
	events := rec.Events()
	Expect(events).To(HaveLen(1))
	Expect(events[0].Outcome).To(Equal(audit.OutcomeFailure))
	Expect(events[0].Reason).To(Equal("commit failed"))
	Expect(events[0].HTTP.Status).To(Equal(http.StatusInternalServerError))
}

func TestAuditRecordsGRPCCommitFailure(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)
	rec := audit.NewMemoryRecorder()
	svc := audit.New(config.NewAuditConfig(), rec)

	lookup := func(string) (authz.GRPCResource, bool) {
		return authz.GRPCResource{Kind: "commit_probes", Action: "create"}, true
	}
	chain := []grpc.UnaryServerInterceptor{
		svc.UnaryBegin(nil),
		server.TransactionUnaryInterceptor(sf),
		authz.UnaryInterceptor(authz.AllowAll(), lookup, nil),
	}
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		return "ok", insertDuplicates(ctx, sf)
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/test.v1.CommitProbeService/CreateCommitProbe"}
	var call func(i int, ctx context.Context, req interface{}) (interface{}, error)
	call = func(i int, ctx context.Context, req interface{}) (interface{}, error) {
		if i == len(chain) {
			return handler(ctx, req)
		}
		return chain[i](ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) { return call(i+1, ctx, req) })
	}
	_, _ = call(0, context.Background(), "req")

	Expect(probeCount(sf)).To(BeZero(), "the commit must have failed")
	events := rec.Events()
	Expect(events).To(HaveLen(1))
	Expect(events[0].Outcome).To(Equal(audit.OutcomeFailure))
	Expect(events[0].Reason).To(Equal("commit failed"))
}
