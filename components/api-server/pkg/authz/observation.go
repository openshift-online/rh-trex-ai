package authz

import "context"

// Observation records what the authorization chain saw and decided for one
// request. A caller that wraps the chain (the audit middleware) stores an
// empty Observation in the context with WithObservation and reads it after
// the request completes. The chain fills it in place; it never replaces it.
type Observation struct {
	// Request is set once the chain has built the authorization request.
	// On a public route it carries the subject, action, and surface only.
	Request *Request
	// Decided is true when the Authorizer returned a decision.
	Decided bool
	// Decision is the Authorizer's decision when Decided is true.
	Decision Decision
	// Err is the enricher or authorizer error that stopped the request.
	Err error
	// Undeclared is true when the route or gRPC method has no declared resource.
	Undeclared bool
	// Public is true for routes registered on APIRouter.Public.
	Public bool
	// Denied is true when the chain itself refused the request: a deny
	// decision, an undeclared route or method, or an error.
	Denied bool
}

type observationKey struct{}

// WithObservation returns a context carrying a fresh Observation.
func WithObservation(ctx context.Context) (context.Context, *Observation) {
	o := &Observation{}
	return context.WithValue(ctx, observationKey{}, o), o
}

// ObservationFromContext returns the Observation stored by WithObservation, or nil.
func ObservationFromContext(ctx context.Context) *Observation {
	o, _ := ctx.Value(observationKey{}).(*Observation)
	return o
}

func observeRequest(ctx context.Context, req Request, public bool) {
	if o := ObservationFromContext(ctx); o != nil {
		r := req
		r.HTTP = nil
		r.GRPCMsg = nil
		o.Request = &r
		o.Public = public
	}
}

func observeDecision(ctx context.Context, d Decision) {
	if o := ObservationFromContext(ctx); o != nil {
		o.Decided = true
		o.Decision = d
		if !d.Allowed {
			o.Denied = true
		}
	}
}

func observeError(ctx context.Context, err error) {
	if o := ObservationFromContext(ctx); o != nil {
		o.Err = err
		o.Denied = true
	}
}

func observeUndeclared(ctx context.Context, req Request) {
	if o := ObservationFromContext(ctx); o != nil {
		if o.Request == nil {
			r := req
			r.HTTP = nil
			r.GRPCMsg = nil
			o.Request = &r
		}
		o.Undeclared = true
		o.Denied = true
	}
}
