package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

func insertOne(ctx context.Context, sf db.SessionFactory, k string) error {
	return sf.New(ctx).Exec(`INSERT INTO commit_probes (k) VALUES (?)`, k).Error
}

func serveTx(sf db.SessionFactory, h http.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	db.TransactionMiddleware(h, sf).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	return rec
}

// A commit failure replaces the success response with 500 and drops the
// headers the handler set for it.
func TestCommitFailureIsReportedToRESTClient(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	rec := serveTx(sf, func(w http.ResponseWriter, r *http.Request) {
		Expect(insertDuplicates(r.Context(), sf)).To(Succeed())
		w.Header().Set("Location", "/commit_probes/dup")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"dup"}`))
	})

	Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	Expect(rec.Header().Get("Location")).To(BeEmpty())
	Expect(rec.Body.String()).NotTo(ContainSubstring(`"id":"dup"`))
	var body map[string]any
	Expect(json.Unmarshal(rec.Body.Bytes(), &body)).To(Succeed())
	Expect(body).To(HaveKey("code"))
	Expect(probeCount(sf)).To(BeZero())
}

func TestCommittedResponseIsSentUnchanged(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	rec := serveTx(sf, func(w http.ResponseWriter, r *http.Request) {
		Expect(insertOne(r.Context(), sf, "a")).To(Succeed())
		w.Header().Set("Location", "/commit_probes/a")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"a"}`))
	})

	Expect(rec.Code).To(Equal(http.StatusCreated))
	Expect(rec.Header().Get("Location")).To(Equal("/commit_probes/a"))
	Expect(rec.Body.String()).To(Equal(`{"id":"a"}`))
	Expect(probeCount(sf)).To(Equal(int64(1)))
}

func TestErrorResponseStillRollsBackAndPassesThrough(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	rec := serveTx(sf, func(w http.ResponseWriter, r *http.Request) {
		Expect(insertOne(r.Context(), sf, "a")).To(Succeed())
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"reason":"conflict"}`))
	})

	Expect(rec.Code).To(Equal(http.StatusConflict))
	Expect(rec.Body.String()).To(Equal(`{"reason":"conflict"}`))
	Expect(probeCount(sf)).To(BeZero())
}

// A handler that flushes commits before its first byte leaves, then streams.
func TestFlushCommitsBeforeStreaming(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	var visibleAfterFlush int64 = -1
	rec := serveTx(sf, func(w http.ResponseWriter, r *http.Request) {
		Expect(insertOne(r.Context(), sf, "a")).To(Succeed())
		_, _ = w.Write([]byte("first,"))
		w.(http.Flusher).Flush()
		visibleAfterFlush = probeCount(sf)
		_, _ = w.Write([]byte("second"))
	})

	Expect(visibleAfterFlush).To(Equal(int64(1)), "the flush must commit first")
	Expect(rec.Code).To(Equal(http.StatusOK))
	Expect(rec.Body.String()).To(Equal("first,second"))
	Expect(rec.Flushed).To(BeTrue())
}

func TestFlushWithFailedCommitSendsOnlyTheError(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	var writeErr error
	rec := serveTx(sf, func(w http.ResponseWriter, r *http.Request) {
		Expect(insertDuplicates(r.Context(), sf)).To(Succeed())
		_, _ = w.Write([]byte("first,"))
		w.(http.Flusher).Flush()
		_, writeErr = w.Write([]byte("second"))
	})

	Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	Expect(rec.Body.String()).NotTo(ContainSubstring("first"))
	Expect(writeErr).To(HaveOccurred(), "writes after a failed commit must fail")
	Expect(probeCount(sf)).To(BeZero())
}

// A response larger than the buffer commits before it starts streaming.
func TestLargeResponseCommitsEarly(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	big := strings.Repeat("x", db.MaxBufferedResponseBytes+1)
	var visibleAfterWrite int64 = -1
	rec := serveTx(sf, func(w http.ResponseWriter, r *http.Request) {
		Expect(insertOne(r.Context(), sf, "a")).To(Succeed())
		_, _ = w.Write([]byte(big))
		visibleAfterWrite = probeCount(sf)
	})

	Expect(visibleAfterWrite).To(Equal(int64(1)))
	Expect(rec.Code).To(Equal(http.StatusOK))
	Expect(rec.Body.Len()).To(Equal(len(big)))
}

func TestCommitFailureIsReportedToGRPCClient(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	createCommitProbeTable(t, sf)

	interceptor := server.TransactionUnaryInterceptor(sf)
	info := &grpc.UnaryServerInfo{FullMethod: "/test.v1.CommitProbeService/CreateCommitProbe"}
	resp, err := interceptor(context.Background(), "req", info, func(ctx context.Context, _ interface{}) (interface{}, error) {
		return "created", insertDuplicates(ctx, sf)
	})

	Expect(resp).To(BeNil())
	Expect(status.Code(err)).To(Equal(codes.Internal))
	Expect(probeCount(sf)).To(BeZero())

	resp, err = interceptor(context.Background(), "req", info, func(ctx context.Context, _ interface{}) (interface{}, error) {
		return "created", insertOne(ctx, sf, "ok")
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(resp).To(Equal("created"))
	Expect(probeCount(sf)).To(Equal(int64(1)))
}
