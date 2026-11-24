package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/felixge/httpsnoop"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

// legacyTransactionMiddleware is the TransactionMiddleware before "Resolve
// Before Responding" (DA-001): the response goes out as the handler writes
// it, and the transaction resolves after the handler returns. Kept here only
// to compare the two in benchmarks.
func legacyTransactionMiddleware(next http.Handler, connection db.SessionFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := db.NewContext(r.Context(), connection)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		*r = *r.WithContext(ctx)
		status := 0
		w = httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					if status == 0 {
						status = code
					}
					next(code)
				}
			},
			Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(b []byte) (int, error) {
					if status == 0 {
						status = http.StatusOK
					}
					return next(b)
				}
			},
		})
		defer func() {
			if status >= http.StatusBadRequest {
				db.MarkForRollback(r.Context(), fmt.Errorf("HTTP status %d", status))
			}
			db.Resolve(r.Context())
		}()
		next.ServeHTTP(w, r)
	})
}

var middlewares = []struct {
	name string
	wrap func(http.Handler, db.SessionFactory) http.Handler
}{
	{"legacy", legacyTransactionMiddleware},
	{"commit-first", db.TransactionMiddleware},
}

// discardWriter is a ResponseWriter that drops the body, so the benchmark
// measures the middleware rather than a growing test recorder.
type discardWriter struct {
	header http.Header
	delay  time.Duration // slept on the first Write: a slow client
	wrote  bool
}

func (d *discardWriter) Header() http.Header { return d.header }
func (d *discardWriter) WriteHeader(int)     {}
func (d *discardWriter) Write(b []byte) (int, error) {
	if !d.wrote && d.delay > 0 {
		time.Sleep(d.delay)
	}
	d.wrote = true
	return len(b), nil
}

func benchSetup(b *testing.B) db.SessionFactory {
	b.Helper()
	h := test.NewHelper(&testing.T{})
	h.DBFactory.ResetDB()
	sf := h.Env().Database.SessionFactory
	if err := sf.New(context.Background()).Exec(`CREATE TABLE IF NOT EXISTS bench_rows (id bigserial PRIMARY KEY, k text)`).Error; err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = sf.New(context.Background()).Exec(`DROP TABLE IF EXISTS bench_rows`).Error })
	return sf
}

// BenchmarkTransactionMiddleware: one row inserted and one body written per
// request, for typical API response sizes.
func BenchmarkTransactionMiddleware(b *testing.B) {
	sf := benchSetup(b)
	for _, size := range []int{1 << 10, 64 << 10, 512 << 10} {
		body := []byte(strings.Repeat("x", size))
		for _, mw := range middlewares {
			b.Run(fmt.Sprintf("%s/%dKiB", mw.name, size>>10), func(b *testing.B) {
				handler := mw.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if err := sf.New(r.Context()).Exec(`INSERT INTO bench_rows (k) VALUES ('b')`).Error; err != nil {
						b.Error(err)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(body)
				}), sf)
				req := httptest.NewRequest(http.MethodPost, "/", nil)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					handler.ServeHTTP(&discardWriter{header: http.Header{}}, req.Clone(context.Background()))
				}
			})
		}
	}
}

// BenchmarkTransactionHeldBySlowClient: the client takes 20ms to accept the
// response. Reports how long after the handler starts the inserted row
// becomes visible to other sessions, which is how long the transaction (and
// any row or advisory locks it holds) stays open.
func BenchmarkTransactionHeldBySlowClient(b *testing.B) {
	sf := benchSetup(b)
	const slowClient = 20 * time.Millisecond
	for _, mw := range middlewares {
		b.Run(mw.name, func(b *testing.B) {
			var totalOpen int64
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("%s-%d-%d", mw.name, time.Now().UnixNano(), i)
				var start time.Time
				visible := make(chan time.Duration, 1)
				handler := mw.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					start = time.Now()
					if err := sf.New(r.Context()).Exec(`INSERT INTO bench_rows (k) VALUES (?)`, key).Error; err != nil {
						b.Error(err)
					}
					go func(start time.Time) {
						for {
							var n int64
							_ = sf.New(context.Background()).Raw(`SELECT count(*) FROM bench_rows WHERE k = ?`, key).Scan(&n).Error
							if n > 0 {
								visible <- time.Since(start)
								return
							}
							time.Sleep(200 * time.Microsecond)
						}
					}(start)
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id":"b"}`))
				}), sf)
				handler.ServeHTTP(&discardWriter{header: http.Header{}, delay: slowClient}, httptest.NewRequest(http.MethodPost, "/", nil))
				atomic.AddInt64(&totalOpen, int64(<-visible))
			}
			b.ReportMetric(float64(totalOpen)/float64(b.N)/1e6, "ms-open/op")
		})
	}
}
