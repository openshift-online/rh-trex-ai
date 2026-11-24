package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

var (
	metricRecorded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "trex_audit_events_total",
		Help: "Audit events handed to the recorder, by outcome.",
	}, []string{"outcome"})
	metricFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "trex_audit_record_failures_total",
		Help: "Audit events the recorder could not record.",
	})
)

func init() {
	prometheus.MustRegister(metricRecorded, metricFailures)
}

// hookTimeout bounds one recorder call made by the request hooks, so a slow
// recorder cannot hold requests open indefinitely.
const hookTimeout = 5 * time.Second

// Service captures audit events and hands them to a Recorder. A nil or
// disabled Service captures nothing.
type Service struct {
	enabled        bool
	reads          bool
	recorder       Recorder
	trustedProxies []*net.IPNet
}

// New builds a Service. A nil recorder disables the service.
func New(cfg *config.AuditConfig, recorder Recorder) *Service {
	if cfg == nil {
		cfg = config.NewAuditConfig()
	}
	// Validate has already rejected malformed entries; any left are skipped.
	trusted, _ := cfg.TrustedProxyNets()
	return &Service{
		enabled:        cfg.Enabled && recorder != nil,
		reads:          cfg.Reads,
		recorder:       recorder,
		trustedProxies: trusted,
	}
}

// Enabled reports whether the service captures events.
func (s *Service) Enabled() bool { return s != nil && s.enabled }

// Recorder returns the recorder events are handed to.
func (s *Service) Recorder() Recorder {
	if s == nil {
		return nil
	}
	return s.recorder
}

// Close closes the recorder when it implements io.Closer.
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	if c, ok := s.recorder.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

var defaultService atomic.Pointer[Service]
var warnUnconfigured sync.Once

// SetDefault makes s the Service used by the package-level Record.
// TRex calls it when the environment initializes.
func SetDefault(s *Service) { defaultService.Store(s) }

// Default returns the Service set by SetDefault, or nil.
func Default() *Service { return defaultService.Load() }

// Record hands an application event to the default Service. See Service.Record.
func Record(ctx context.Context, e Event) error {
	s := Default()
	if s == nil {
		warnUnconfigured.Do(func() {
			slog.WarnContext(ctx, "audit.Record called before the audit service was configured; event dropped")
		})
		return nil
	}
	return s.Record(ctx, e)
}

// Record hands e to the recorder after filling the fields it can derive from
// ctx. A disabled Service returns nil. Callers should propagate the error.
func (s *Service) Record(ctx context.Context, e Event) error {
	if !s.Enabled() {
		return nil
	}
	fill(ctx, &e)
	if err := validate(e); err != nil {
		metricFailures.Inc()
		return err
	}
	if err := s.recorder.Record(ctx, e); err != nil {
		metricFailures.Inc()
		return fmt.Errorf("audit: recording event: %w", err)
	}
	metricRecorded.WithLabelValues(e.Outcome).Inc()
	return nil
}

// recordFromHook records an event captured by a request hook. The request
// has completed, so a failure is logged and counted and nothing else.
func (s *Service) recordFromHook(ctx context.Context, e Event) {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), hookTimeout)
	defer cancel()
	if err := s.Record(rctx, e); err != nil {
		slog.ErrorContext(ctx, "Unable to record audit event",
			"error", err, "outcome", e.Outcome, "action", e.Action, "resource_kind", e.Resource.Kind)
	}
}

func validate(e Event) error {
	if e.Action == "" {
		return errors.New("audit: event has no action")
	}
	switch e.Outcome {
	case OutcomeSuccess, OutcomeFailure, OutcomeDenied, OutcomeUnauthenticated, OutcomeError:
	default:
		return fmt.Errorf("audit: unknown outcome %q", e.Outcome)
	}
	switch e.Actor.Type {
	case ActorUser, ActorSystem, ActorAnonymous:
	default:
		return fmt.Errorf("audit: unknown actor type %q", e.Actor.Type)
	}
	return nil
}

// fill sets the fields of e that are empty and can be derived from ctx.
func fill(ctx context.Context, e *Event) {
	if e.EventID == "" {
		e.EventID = api.NewID()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}
	if e.OperationID == "" {
		e.OperationID = logger.GetOperationID(ctx)
	}
	if e.TraceID == "" {
		if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
			e.TraceID = sc.TraceID().String()
		}
	}
	if e.Transport == "" {
		e.Transport = TransportInternal
	}
	if e.Actor.Type == "" {
		if a := actorFromSubject(authz.SubjectFromContext(ctx)); a.Username != "" {
			e.Actor = a
		} else {
			e.Actor.Type = ActorSystem
		}
	}
}

func actorFromSubject(s authz.Subject) Actor {
	if s.Username == "" {
		return Actor{Type: ActorAnonymous}
	}
	a := Actor{Type: ActorUser, Username: s.Username}
	a.Subject = claim(s.Claims, "sub")
	a.Issuer = claim(s.Claims, "iss")
	a.ClientID = claim(s.Claims, "azp")
	if a.ClientID == "" {
		a.ClientID = claim(s.Claims, "clientId")
	}
	return a
}

func claim(c jwt.MapClaims, name string) string {
	if c == nil {
		return ""
	}
	v, _ := c[name].(string)
	return v
}

// recordsAllowed reports whether an allowed request with this action on this
// kind is recorded. Reads are recorded only for kinds registered with
// RecordReads, or for every kind with --audit-reads.
func (s *Service) recordsAllowed(action, kind string) bool {
	return !isRead(action) || s.reads || readsRecordedFor(kind)
}

var (
	readKindsMu sync.RWMutex
	readKinds   = map[string]bool{}
)

// RecordReads makes TRex record allowed get, list, and watch requests on the
// given kinds, on REST and gRPC alike. Call it from a plugin's init() for
// kinds whose reads matter, such as secrets. Other kinds record reads only
// with --audit-reads.
func RecordReads(kinds ...string) {
	readKindsMu.Lock()
	defer readKindsMu.Unlock()
	for _, k := range kinds {
		readKinds[k] = true
	}
}

// UndeclaredReadKinds returns the kinds passed to RecordReads that are not in
// declared, sorted. A kind there is almost always a typo: its reads would
// silently go unrecorded.
func UndeclaredReadKinds(declared []string) []string {
	known := make(map[string]bool, len(declared))
	for _, k := range declared {
		known[k] = true
	}
	readKindsMu.RLock()
	defer readKindsMu.RUnlock()
	var out []string
	for k := range readKinds {
		if !known[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// WarnUndeclaredReadKinds logs one warning per kind passed to RecordReads
// that no REST route or gRPC method declares. TRex calls it at startup.
func WarnUndeclaredReadKinds(declared []string) {
	for _, k := range UndeclaredReadKinds(declared) {
		slog.Warn("audit.RecordReads names a kind no route or gRPC method declares; its reads will not be recorded", "kind", k)
	}
}

func readsRecordedFor(kind string) bool {
	if kind == "" {
		return false
	}
	readKindsMu.RLock()
	defer readKindsMu.RUnlock()
	return readKinds[kind]
}

// reasonCommitFailed is the reason of an event whose request reported success
// but whose transaction failed to commit.
const reasonCommitFailed = "commit failed"

// commitFailed reports whether the request transaction failed to commit.
func commitFailed(ctx context.Context) bool {
	r := db.TxResultFromContext(ctx)
	return r != nil && r.Outcome == db.TxCommitFailed
}
