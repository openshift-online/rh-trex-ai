package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

// HTTPBegin is the outer REST hook. TRex attaches it to each authenticated
// surface router before authentication. It sees the final status of every
// request and records the outcomes that the request transaction cannot hold:
// authentication failures, denials, authorization errors, and failures.
func (s *Service) HTTPBegin(next http.Handler) http.Handler {
	if !s.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m, ok := r.Context().Value(authnMarkerKey{}).(*authnMarker); ok {
			m.begun = true
		}
		ctx, obs := authz.WithObservation(r.Context())
		ctx, st := withState(ctx)
		r = r.WithContext(ctx)

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

		panicked := true
		defer func() {
			if panicked {
				s.finishHTTP(ctx, r, obs, st, http.StatusInternalServerError, "panic")
			}
		}()
		next.ServeHTTP(w, r)
		panicked = false
		if status == 0 {
			status = http.StatusOK
		}
		s.finishHTTP(ctx, r, obs, st, status, "")
	})
}

type authnMarkerKey struct{}

// authnMarker tells HTTPAuthentication that HTTPBegin saw the request.
type authnMarker struct{ begun bool }

// HTTPAuthentication records requests under pathPrefix that an outer
// authentication handler (one wrapping the whole router, such as the JWT
// handler) refuses with 401 before HTTPBegin runs. Requests that reach
// HTTPBegin are recorded there instead.
func (s *Service) HTTPAuthentication(pathPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if !s.Enabled() {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := &authnMarker{}
			r = r.WithContext(context.WithValue(r.Context(), authnMarkerKey{}, m))
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
			})
			next.ServeHTTP(w, r)
			if m.begun || status != http.StatusUnauthorized || !strings.HasPrefix(r.URL.Path, pathPrefix) {
				return
			}
			e := httpEvent(r, nil, OutcomeUnauthenticated, "authentication failed", status)
			e.Action = httpAction(r.Method)
			s.streamOnly(r.Context(), e)
		})
	}
}

// HTTPRecord is the inner REST hook. TRex attaches it after authorization on
// every Resource and Public subrouter. For an allowed request that is
// recorded, it writes the success event in the request transaction before the
// handler runs, and refuses the request with 503 when that write fails.
func (s *Service) HTTPRecord(next http.Handler) http.Handler {
	if !s.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		obs := authz.ObservationFromContext(ctx)
		st := stateFrom(ctx)
		if obs == nil || obs.Request == nil || st == nil || !s.recordsAllowed(obs.Request.Action, obs.Request.Resource.Kind) {
			next.ServeHTTP(w, r)
			return
		}
		e := httpEvent(r, obs.Request, OutcomeSuccess, "", 0)
		e.Actor = actorFromSubject(authz.SubjectFromContext(ctx))
		if err := s.Record(ctx, e); err != nil {
			st.insertFailed = true
			slog.ErrorContext(ctx, "Unable to write audit event; refusing request", "error", err)
			body := errors.GeneralError("audit unavailable").AsOpenapiError(logger.GetOperationID(ctx))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(body)
			return
		}
		st.inserted = true
		next.ServeHTTP(w, r)
	})
}

func (s *Service) finishHTTP(ctx context.Context, r *http.Request, obs *authz.Observation, st *requestState, status int, reason string) {
	req := obs.Request
	switch {
	case obs.Undeclared:
		e := httpEvent(r, req, OutcomeDenied, "route has no declared resource kind", status)
		if req == nil {
			e.Action = httpAction(r.Method)
		}
		s.recordDetached(ctx, e)
	case obs.Err != nil:
		s.recordDetached(ctx, httpEvent(r, req, OutcomeError, "authorization unavailable", status))
	case obs.Decided && !obs.Decision.Allowed:
		s.recordDetached(ctx, httpEvent(r, req, OutcomeDenied, denyReason(obs.Decision), status))
	case req == nil:
		// The request never reached authorization.
		if status == http.StatusUnauthorized {
			e := httpEvent(r, nil, OutcomeUnauthenticated, "authentication failed", status)
			e.Action = httpAction(r.Method)
			s.streamOnly(ctx, e)
		} else if status >= http.StatusBadRequest && !isRead(httpAction(r.Method)) {
			e := httpEvent(r, nil, OutcomeFailure, "request failed before authorization", status)
			e.Action = httpAction(r.Method)
			s.recordDetached(ctx, e)
		}
	case status >= http.StatusBadRequest || reason != "":
		// The request transaction rolled back, taking any success event with it.
		if !s.recordsAllowed(req.Action, req.Resource.Kind) {
			return
		}
		if reason == "" && st.insertFailed {
			reason = "audit write failed"
		}
		s.recordDetached(ctx, httpEvent(r, req, OutcomeFailure, reason, status))
	}
}

func httpEvent(r *http.Request, req *authz.Request, outcome, reason string, status int) Event {
	e := Event{
		Outcome:   outcome,
		Reason:    reason,
		Transport: TransportHTTP,
		HTTP:      HTTPInfo{Method: r.Method, Path: r.URL.Path, Status: status},
		Source:    Source{IP: remoteIP(r.RemoteAddr), UserAgent: r.UserAgent()},
		TraceID:   traceIDFromHeader(r.Header.Get("traceparent")),
	}
	if outcome == OutcomeSuccess {
		e.HTTP.Status = 0
	}
	if req != nil {
		e.Action = req.Action
		e.Resource = Resource{Surface: req.Resource.Surface, Kind: req.Resource.Kind, ID: req.Resource.ID, Parent: req.Resource.Parent}
		e.Actor = actorFromSubject(req.Subject)
	} else {
		e.Actor = Actor{Type: ActorAnonymous}
	}
	return e
}

func denyReason(d authz.Decision) string {
	reason := d.Reason
	if reason == "" {
		reason = "forbidden"
	}
	if d.HideExistence {
		reason += " (reported as not found)"
	}
	return reason
}

// httpAction mirrors the action mapping of SEC-002 for requests that never
// reached authorization.
func httpAction(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return "get"
	case http.MethodPost:
		return "create"
	case http.MethodPut:
		return "update"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		return "delete"
	}
	return strings.ToLower(method)
}

func remoteIP(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// traceIDFromHeader extracts the trace ID from a W3C traceparent header.
func traceIDFromHeader(h string) string {
	parts := strings.Split(strings.TrimSpace(h), "-")
	if len(parts) != 4 || len(parts[1]) != 32 || parts[1] == strings.Repeat("0", 32) {
		return ""
	}
	for _, c := range parts[1] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return ""
		}
	}
	return parts[1]
}
