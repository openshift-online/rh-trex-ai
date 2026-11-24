package audit

import (
	"context"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
)

// HTTPBegin is the REST hook. TRex attaches it to each authenticated surface
// router, before authentication. When the request completes it records one
// event with the final outcome and status.
func (s *Service) HTTPBegin(next http.Handler) http.Handler {
	if !s.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m, ok := r.Context().Value(authnMarkerKey{}).(*authnMarker); ok {
			m.begun = true
		}
		ctx, obs := authz.WithObservation(r.Context())
		// The transaction middleware runs inside this hook; it reports how
		// the request transaction ended here.
		ctx, _ = db.WithTxResult(ctx)
		r = r.WithContext(ctx)

		status := 0
		w = httpsnoop.Wrap(w, statusHooks(&status))

		panicked := true
		defer func() {
			if panicked {
				s.finishHTTP(ctx, r, obs, http.StatusInternalServerError, "panic")
			}
		}()
		next.ServeHTTP(w, r)
		panicked = false
		if status == 0 {
			status = http.StatusOK
		}
		s.finishHTTP(ctx, r, obs, status, "")
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
			w = httpsnoop.Wrap(w, statusHooks(&status))
			next.ServeHTTP(w, r)
			if m.begun || status != http.StatusUnauthorized || !strings.HasPrefix(r.URL.Path, pathPrefix) {
				return
			}
			e := s.httpEvent(r, nil, OutcomeUnauthenticated, "authentication failed", status)
			e.Action = httpAction(r.Method)
			s.recordFromHook(r.Context(), e)
		})
	}
}

func statusHooks(status *int) httpsnoop.Hooks {
	return httpsnoop.Hooks{
		WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(code int) {
				if *status == 0 {
					*status = code
				}
				next(code)
			}
		},
		Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(b []byte) (int, error) {
				if *status == 0 {
					*status = http.StatusOK
				}
				return next(b)
			}
		},
	}
}

func (s *Service) finishHTTP(ctx context.Context, r *http.Request, obs *authz.Observation, status int, reason string) {
	req := obs.Request
	switch {
	case obs.Undeclared:
		e := s.httpEvent(r, req, OutcomeDenied, "route has no declared resource kind", status)
		if req == nil {
			e.Action = httpAction(r.Method)
		}
		s.recordFromHook(ctx, e)
	case obs.Err != nil:
		s.recordFromHook(ctx, s.httpEvent(r, req, OutcomeError, "authorization unavailable", status))
	case obs.Decided && !obs.Decision.Allowed:
		s.recordFromHook(ctx, s.httpEvent(r, req, OutcomeDenied, denyReason(obs.Decision), status))
	case req == nil:
		// The request never reached authorization.
		action := httpAction(r.Method)
		if status == http.StatusUnauthorized {
			e := s.httpEvent(r, nil, OutcomeUnauthenticated, "authentication failed", status)
			e.Action = action
			s.recordFromHook(ctx, e)
		} else if status >= http.StatusBadRequest && !isRead(action) {
			e := s.httpEvent(r, nil, OutcomeFailure, "request failed before authorization", status)
			e.Action = action
			s.recordFromHook(ctx, e)
		}
	case !s.recordsAllowed(req.Action, req.Resource.Kind):
	case status >= http.StatusBadRequest || reason != "":
		s.recordFromHook(ctx, s.httpEvent(r, req, OutcomeFailure, reason, status))
	case commitFailed(ctx):
		// The response said success, but nothing was persisted.
		s.recordFromHook(ctx, s.httpEvent(r, req, OutcomeFailure, reasonCommitFailed, status))
	default:
		s.recordFromHook(ctx, s.httpEvent(r, req, OutcomeSuccess, "", status))
	}
}

func (s *Service) httpEvent(r *http.Request, req *authz.Request, outcome, reason string, status int) Event {
	e := Event{
		Outcome:   outcome,
		Reason:    reason,
		Transport: TransportHTTP,
		HTTP:      HTTPInfo{Method: r.Method, Path: r.URL.Path, Status: status},
		Source:    s.resolveSource(r.RemoteAddr, r.Header.Values("X-Forwarded-For"), r.UserAgent()),
		TraceID:   traceIDFromHeader(r.Header.Get("traceparent")),
		Actor:     Actor{Type: ActorAnonymous},
	}
	if req != nil {
		e.Action = req.Action
		e.Resource = Resource{Surface: req.Resource.Surface, Kind: req.Resource.Kind, ID: req.Resource.ID, Parent: req.Resource.Parent}
		e.Actor = actorFromSubject(req.Subject)
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
