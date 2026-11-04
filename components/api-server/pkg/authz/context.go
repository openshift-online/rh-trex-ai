package authz

import (
	"context"

	"github.com/golang-jwt/jwt/v4"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/auth"
)

type subjectKey struct{}

// SubjectFromContext returns the verified subject of the current request.
// It is set by TRex before Enrich and Authorize run. It falls back to the
// authenticated username when called outside the authorization chain.
func SubjectFromContext(ctx context.Context) Subject {
	if s, ok := ctx.Value(subjectKey{}).(Subject); ok {
		return s
	}
	return subjectFromAuth(ctx)
}

func withSubject(ctx context.Context, s Subject) context.Context {
	return context.WithValue(ctx, subjectKey{}, s)
}

// subjectFromAuth builds the subject from what authentication put in context.
func subjectFromAuth(ctx context.Context) Subject {
	s := Subject{Username: auth.GetUsernameFromContext(ctx)}
	if token, err := auth.TokenFromContext(ctx); err == nil && token != nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			s.Claims = claims
		}
	}
	return s
}

// prepare stores the subject and runs the optional enricher.
func prepare(ctx context.Context, a Authorizer) (context.Context, Subject, error) {
	s := subjectFromAuth(ctx)
	ctx = withSubject(ctx, s)
	if e, ok := a.(SubjectEnricher); ok {
		var err error
		if ctx, err = e.Enrich(ctx, s); err != nil {
			return ctx, s, err
		}
	}
	return ctx, s, nil
}
