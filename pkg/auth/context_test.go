package auth

import (
	"context"
	"testing"

	"github.com/golang-jwt/jwt/v4"
)

func TestGetRolesFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected []string
	}{
		{
			name:     "no token in context",
			ctx:      context.Background(),
			expected: nil,
		},
		{
			name: "token without realm_access",
			ctx: context.WithValue(context.Background(), ContextAuthKey,
				&jwt.Token{Claims: jwt.MapClaims{"username": "alice"}, Valid: true}),
			expected: nil,
		},
		{
			name: "token with roles",
			ctx: context.WithValue(context.Background(), ContextAuthKey,
				&jwt.Token{
					Claims: jwt.MapClaims{
						"realm_access": map[string]interface{}{
							"roles": []interface{}{"admin", "user"},
						},
					},
					Valid: true,
				}),
			expected: []string{"admin", "user"},
		},
		{
			name: "token with empty roles",
			ctx: context.WithValue(context.Background(), ContextAuthKey,
				&jwt.Token{
					Claims: jwt.MapClaims{
						"realm_access": map[string]interface{}{
							"roles": []interface{}{},
						},
					},
					Valid: true,
				}),
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roles := GetRolesFromContext(tt.ctx)
			if tt.expected == nil {
				if roles != nil {
					t.Errorf("expected nil, got %v", roles)
				}
				return
			}
			if len(roles) != len(tt.expected) {
				t.Errorf("expected %d roles, got %d", len(tt.expected), len(roles))
				return
			}
			for i, role := range roles {
				if role != tt.expected[i] {
					t.Errorf("role[%d] = %q, want %q", i, role, tt.expected[i])
				}
			}
		})
	}
}
