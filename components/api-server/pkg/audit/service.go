package audit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	dbContext "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_context"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

var (
	metricRecorded = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "trex_audit_events_recorded_total",
		Help: "Audit event writes, by outcome. Success events written in a request transaction that later rolls back are included.",
	}, []string{"outcome"})
	metricWriteFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "trex_audit_write_failures_total",
		Help: "Audit events that could not be written.",
	})
	metricSealFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "trex_audit_seal_failures_total",
		Help: "Sealing or export passes that failed.",
	})
	metricSealed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "trex_audit_sealed_total",
		Help: "Audit events linked into the hash chain.",
	})
	metricExported = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "trex_audit_exported_total",
		Help: "Audit events written to the export sink.",
	})
	metricStreamOnly = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "trex_audit_stream_only_total",
		Help: "Unauthenticated audit events written to the export sink without being stored.",
	})
	metricUnsealed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "trex_audit_unsealed_events",
		Help: "Committed audit events not yet sealed, as seen by the last sealing pass.",
	})
)

func init() {
	prometheus.MustRegister(metricRecorded, metricWriteFailures, metricSealFailures,
		metricSealed, metricExported, metricUnsealed, metricStreamOnly)
}

// Service records, seals, exports, and verifies audit events. A nil *Service
// and a disabled Service record nothing.
type Service struct {
	enabled      bool
	reads        bool
	sessions     db.SessionFactory
	key          []byte
	keyID        string
	sinkName     string
	sink         Sink
	sealInterval time.Duration
	batchSize    int
	// insert writes a row; tests replace it. Nil means the database.
	insert func(ctx context.Context, r *row) error

	runMu   sync.Mutex
	stop    context.CancelFunc
	stopped chan struct{}
}

// Option customizes a Service.
type Option func(*Service)

// WithSink replaces the sink selected by --audit-sink, for example to ship
// lines to syslog or a message queue. The Service closes it on Stop.
func WithSink(sink Sink) Option {
	return func(s *Service) { s.sink = sink }
}

// New builds a Service from configuration. It opens the export sink unless
// WithSink supplies one.
func New(sessions db.SessionFactory, cfg *config.AuditConfig, opts ...Option) (*Service, error) {
	if cfg == nil {
		cfg = config.NewAuditConfig()
	}
	s := &Service{
		enabled:      cfg.Enabled,
		reads:        cfg.Reads,
		sessions:     sessions,
		key:          cfg.HMACKey(),
		keyID:        KeyID(cfg.HMACKey()),
		sinkName:     cfg.Sink,
		sealInterval: cfg.SealInterval,
		batchSize:    500,
	}
	for _, opt := range opts {
		opt(s)
	}
	if !s.enabled || s.sink != nil {
		return s, nil
	}
	sink, err := OpenSink(cfg)
	if err != nil {
		return nil, err
	}
	s.sink = sink
	return s, nil
}

// Enabled reports whether the service records events.
func (s *Service) Enabled() bool { return s != nil && s.enabled }

// KeyID is the identifier of the configured HMAC key, or "none".
func (s *Service) KeyID() string { return s.keyID }

var defaultService atomic.Pointer[Service]
var warnUnconfigured sync.Once

// SetDefault makes s the Service used by the package-level Record.
// TRex calls it when the environment initializes.
func SetDefault(s *Service) { defaultService.Store(s) }

// Default returns the Service set by SetDefault, or nil.
func Default() *Service { return defaultService.Load() }

// Record writes an application event with the default Service. See Service.Record.
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

// ErrNoSessionFactory is returned when the Service has no database.
var ErrNoSessionFactory = errors.New("audit: no database session factory")

// Record writes e in the transaction held by ctx, if any, and on its own
// otherwise. Fields it can derive from ctx are filled when empty. A disabled
// Service returns nil. Callers must propagate the error.
func (s *Service) Record(ctx context.Context, e Event) error {
	if !s.Enabled() {
		return nil
	}
	fill(ctx, &e)
	if err := validate(e); err != nil {
		metricWriteFailures.Inc()
		return err
	}
	r, err := toRow(e)
	if err != nil {
		metricWriteFailures.Inc()
		return fmt.Errorf("audit: %w", err)
	}
	insert := s.insert
	if insert == nil {
		if s.sessions == nil {
			metricWriteFailures.Inc()
			return ErrNoSessionFactory
		}
		insert = func(ctx context.Context, r *row) error { return s.sessions.New(ctx).Create(r).Error }
	}
	if err := insert(ctx, r); err != nil {
		metricWriteFailures.Inc()
		return fmt.Errorf("audit: writing event: %w", err)
	}
	metricRecorded.WithLabelValues(e.Outcome).Inc()
	return nil
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

// recordDetached writes e outside any request transaction. A failure is
// logged and counted; it does not change the response.
func (s *Service) recordDetached(ctx context.Context, e Event) {
	dctx, cancel := context.WithTimeout(detach(ctx), detachedWriteTimeout)
	defer cancel()
	if err := s.Record(dctx, e); err != nil {
		slog.ErrorContext(ctx, "Unable to write audit event",
			"error", err, "outcome", e.Outcome, "action", e.Action, "resource_kind", e.Resource.Kind)
	}
}

// detachedWriteTimeout bounds an out-of-transaction write, so a saturated or
// unreachable database cannot hold a request open indefinitely.
const detachedWriteTimeout = 5 * time.Second

// detach returns a context that keeps request values (operation ID, subject,
// span) but holds no transaction and is not canceled with the request.
func detach(ctx context.Context) context.Context {
	return dbContext.WithTransaction(context.WithoutCancel(ctx), nil)
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

func readsRecordedFor(kind string) bool {
	if kind == "" {
		return false
	}
	readKindsMu.RLock()
	defer readKindsMu.RUnlock()
	return readKinds[kind]
}

// streamOnly writes an unauthenticated event to the export sink without
// storing it. Anyone can cause these, so they must not cost a database write
// or grow the chain. With no sink they are logged instead.
func (s *Service) streamOnly(ctx context.Context, e Event) {
	fill(ctx, &e)
	r, err := toRow(e)
	if err == nil {
		var line *Line
		if line, err = lineFor(r, 0, ""); err == nil {
			var b []byte
			if b, err = marshalCanonical(line); err == nil {
				if s.sink == nil {
					slog.InfoContext(ctx, "Unauthenticated request", "audit_event", string(b))
					metricStreamOnly.Inc()
					return
				}
				if err = s.sink.WriteLines([][]byte{append(b, '\n')}); err == nil {
					metricStreamOnly.Inc()
					return
				}
			}
		}
	}
	metricWriteFailures.Inc()
	slog.ErrorContext(ctx, "Unable to export unauthenticated audit event", "error", err)
}

// requestState is shared between the outer and inner audit hooks of one request.
type requestState struct {
	// inserted is true when the success event was written in the request transaction.
	inserted bool
	// insertFailed is true when that write failed and the request was refused.
	insertFailed bool
}

type stateKey struct{}

func withState(ctx context.Context) (context.Context, *requestState) {
	st := &requestState{}
	return context.WithValue(ctx, stateKey{}, st), st
}

func stateFrom(ctx context.Context) *requestState {
	st, _ := ctx.Value(stateKey{}).(*requestState)
	return st
}
