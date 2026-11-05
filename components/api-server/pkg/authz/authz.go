// Package authz is TRex's authorization layer (specs/security/authorization.spec.md).
//
// TRex decides where and when authorization runs, on REST and gRPC alike.
// The application decides the policy, by returning an Authorizer from its
// environment's OverrideAuthorizer hook.
package authz

import (
	"context"
	"net/http"

	"github.com/golang-jwt/jwt/v4"
)

// Standard actions. Custom verbs are allowed.
const (
	ActionGet    = "get"
	ActionList   = "list"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionPatch  = "patch"
	ActionDelete = "delete"
	ActionWatch  = "watch"
)

// SurfaceV1 is the Surface of routes on the main /v1 router.
const SurfaceV1 = "v1"

// Authorizer makes every authorization decision for REST and gRPC.
type Authorizer interface {
	// Authorize decides one request. An error means "could not decide"; it
	// is never treated as allow.
	Authorize(ctx context.Context, req Request) (Decision, error)
}

// SubjectEnricher is optional. When the Authorizer implements it, TRex calls
// Enrich once per request, after authentication and before Authorize. The
// returned context is used for Authorize and reaches the handler.
type SubjectEnricher interface {
	Enrich(ctx context.Context, s Subject) (context.Context, error)
}

// Request is what TRex hands the Authorizer.
type Request struct {
	Subject  Subject
	Action   string
	Resource Resource

	HTTP       *http.Request // REST only
	GRPCMethod string        // gRPC only, full method name
	GRPCMsg    any           // gRPC only, the request message (nil when a stream opens)
}

// Subject is the verified caller.
type Subject struct {
	Username string
	Claims   jwt.MapClaims // verified token claims; nil when JWT is disabled
}

// Resource is what the request acts on.
type Resource struct {
	Surface string            // "v1", or a route prefix such as "ext"; empty on gRPC
	Kind    string            // e.g. "dinosaurs"
	ID      string            // the {id} path variable; empty for list and create
	Parent  map[string]string // other path variables, e.g. {"dinosaur_id": "d-1"}
}

// Decision is the Authorizer's answer.
type Decision struct {
	Allowed bool
	Reason  string
	// HideExistence makes a deny look like "not found" (REST 404, gRPC NotFound).
	HideExistence bool
}

// Allow is a convenience for an allowed Decision.
func Allow() Decision { return Decision{Allowed: true} }

// Deny is a convenience for a denied Decision.
func Deny(reason string) Decision { return Decision{Reason: reason} }
