package authz

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

// HTTPMiddleware authorizes requests for one kind on one surface. TRex
// attaches it to the subrouter returned by APIRouter.Resource.
func HTTPMiddleware(a Authorizer, surface, kind string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, subject, err := prepare(r.Context(), a)
			id, parent := pathVars(r)
			req := Request{
				Subject:  subject,
				Action:   httpAction(r.Method, id != ""),
				Resource: Resource{Surface: surface, Kind: kind, ID: id, Parent: parent},
				HTTP:     r,
			}
			observeRequest(ctx, req, false)
			if err != nil {
				writeUnavailable(ctx, w, err)
				return
			}
			d, err := a.Authorize(ctx, req)
			if err != nil {
				writeUnavailable(ctx, w, err)
				return
			}
			observeDecision(ctx, d)
			if !d.Allowed {
				writeDenied(ctx, w, d)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HTTPPublicMiddleware runs the optional enricher but no decision.
// Deprecated: use HTTPPublicSurfaceMiddleware, which also records the surface.
func HTTPPublicMiddleware(a Authorizer) func(http.Handler) http.Handler {
	return HTTPPublicSurfaceMiddleware(a, "")
}

// HTTPPublicSurfaceMiddleware runs the optional enricher but no decision. TRex
// attaches it to the subrouter returned by APIRouter.Public.
func HTTPPublicSurfaceMiddleware(a Authorizer, surface string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, subject, err := prepare(r.Context(), a)
			id, parent := pathVars(r)
			observeRequest(ctx, Request{
				Subject:  subject,
				Action:   httpAction(r.Method, id != ""),
				Resource: Resource{Surface: surface, ID: id, Parent: parent},
			}, true)
			if err != nil {
				writeUnavailable(ctx, w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UndeclaredHandler denies a route registered outside Resource and Public.
func UndeclaredHandler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.ErrorContext(r.Context(), "Route has no declared resource kind; denying", "route", path)
		id, parent := pathVars(r)
		observeUndeclared(r.Context(), Request{
			Subject:  subjectFromAuth(r.Context()),
			Action:   httpAction(r.Method, id != ""),
			Resource: Resource{ID: id, Parent: parent},
		})
		writeError(r.Context(), w, errors.Forbidden("route has no declared resource kind"), http.StatusForbidden)
	})
}

func pathVars(r *http.Request) (string, map[string]string) {
	vars := mux.Vars(r)
	id := vars["id"]
	var parent map[string]string
	for k, v := range vars {
		if k == "id" {
			continue
		}
		if parent == nil {
			parent = make(map[string]string, len(vars))
		}
		parent[k] = v
	}
	return id, parent
}

func httpAction(method string, hasID bool) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		if hasID {
			return ActionGet
		}
		return ActionList
	case http.MethodPost:
		return ActionCreate
	case http.MethodPut:
		return ActionUpdate
	case http.MethodPatch:
		return ActionPatch
	case http.MethodDelete:
		return ActionDelete
	default:
		return strings.ToLower(method)
	}
}

func writeDenied(ctx context.Context, w http.ResponseWriter, d Decision) {
	if d.HideExistence {
		writeError(ctx, w, errors.NotFound("resource not found"), http.StatusNotFound)
		return
	}
	reason := d.Reason
	if reason == "" {
		reason = "forbidden"
	}
	writeError(ctx, w, errors.Forbidden("%s", reason), http.StatusForbidden)
}

func writeUnavailable(ctx context.Context, w http.ResponseWriter, err error) {
	slog.ErrorContext(ctx, "Authorization could not decide", "error", err)
	observeError(ctx, err)
	writeError(ctx, w, errors.GeneralError("authorization unavailable"), http.StatusServiceUnavailable)
}

func writeError(ctx context.Context, w http.ResponseWriter, svcErr *errors.ServiceError, status int) {
	body := svcErr.AsOpenapiError(logger.GetOperationID(ctx))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
