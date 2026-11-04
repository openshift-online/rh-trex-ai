package authz

import "context"

// RequireIdentity allows a request when the subject has a username.
func RequireIdentity() Authorizer { return requireIdentity{} }

// AllowAll allows every request. Use it for local development and unit tests.
func AllowAll() Authorizer { return allowAll{} }

// FromFlag returns RequireIdentity when enableAuthz is true, else AllowAll.
// It maps the --enable-authz flag for environments that use it.
func FromFlag(enableAuthz bool) Authorizer {
	if enableAuthz {
		return RequireIdentity()
	}
	return AllowAll()
}

type requireIdentity struct{}

func (requireIdentity) Authorize(_ context.Context, req Request) (Decision, error) {
	if req.Subject.Username == "" {
		return Deny("missing identity"), nil
	}
	return Allow(), nil
}

type allowAll struct{}

func (allowAll) Authorize(context.Context, Request) (Decision, error) { return Allow(), nil }
