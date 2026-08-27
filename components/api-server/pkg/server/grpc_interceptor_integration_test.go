package server

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

// TestGRPCInterceptorIntegration demonstrates how the API server would use the registry
func TestGRPCInterceptorIntegration(t *testing.T) {
	// Reset global state for clean testing
	originalUnary := preAuthUnaryInterceptors
	originalStream := preAuthStreamInterceptors
	defer func() {
		preAuthUnaryInterceptors = originalUnary
		preAuthStreamInterceptors = originalStream
	}()
	preAuthUnaryInterceptors = nil
	preAuthStreamInterceptors = nil

	// Example: API server would register bearer token interceptors
	executed := []string{}

	bearerTokenUnaryInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Simulate bearer token validation
		executed = append(executed, "bearer-unary")

		// In real implementation, would check for AMBIENT_API_TOKEN header
		// For this test, just pass through
		return handler(ctx, req)
	}

	bearerTokenStreamInterceptor := func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Simulate bearer token validation
		executed = append(executed, "bearer-stream")

		// In real implementation, would check for AMBIENT_API_TOKEN header
		// For this test, just pass through
		return handler(srv, ss)
	}

	// API server registers its interceptors (this is what API server would do)
	RegisterPreAuthGRPCUnaryInterceptor(bearerTokenUnaryInterceptor)
	RegisterPreAuthGRPCStreamInterceptor(bearerTokenStreamInterceptor)

	// Verify registration worked
	if len(preAuthUnaryInterceptors) != 1 {
		t.Errorf("Expected 1 unary interceptor registered, got %d", len(preAuthUnaryInterceptors))
	}
	if len(preAuthStreamInterceptors) != 1 {
		t.Errorf("Expected 1 stream interceptor registered, got %d", len(preAuthStreamInterceptors))
	}

	// Simulate creating a gRPC server (without actually starting it)
	// This would happen in NewDefaultGRPCServer
	jwtAuthInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		executed = append(executed, "jwt-auth")
		return handler(ctx, req)
	}

	// Build the chain as NewDefaultGRPCServer would
	unaryChain := []grpc.UnaryServerInterceptor{
		// Standard interceptors would be here (recovery, logging, etc.)
	}
	unaryChain = append(unaryChain, preAuthUnaryInterceptors...) // Pre-auth interceptors BEFORE JWT
	unaryChain = append(unaryChain, jwtAuthInterceptor)          // JWT auth interceptor

	// Verify chain order by simulating execution
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	finalHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		executed = append(executed, "final-handler")
		return "success", nil
	}

	// Execute the chain (in reverse order as gRPC does)
	var handler grpc.UnaryHandler = finalHandler
	for i := len(unaryChain) - 1; i >= 0; i-- {
		currentHandler := handler
		interceptor := unaryChain[i]
		handler = func(ctx context.Context, req interface{}) (interface{}, error) {
			return interceptor(ctx, req, info, currentHandler)
		}
	}

	result, err := handler(ctx, "test-request")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	// Verify execution order: bearer-token → jwt-auth → final-handler
	expectedOrder := []string{"bearer-unary", "jwt-auth", "final-handler"}
	if len(executed) != len(expectedOrder) {
		t.Errorf("Expected %d executions, got %d: %v", len(expectedOrder), len(executed), executed)
	}
	for i, expected := range expectedOrder {
		if i >= len(executed) || executed[i] != expected {
			t.Errorf("Expected execution %d to be %s, got %s", i, expected, executed[i])
		}
	}
}

// TestGRPCInterceptorWithoutRegistration verifies behavior when no interceptors are registered
func TestGRPCInterceptorWithoutRegistration(t *testing.T) {
	// Reset global state to empty
	originalUnary := preAuthUnaryInterceptors
	originalStream := preAuthStreamInterceptors
	defer func() {
		preAuthUnaryInterceptors = originalUnary
		preAuthStreamInterceptors = originalStream
	}()
	preAuthUnaryInterceptors = nil
	preAuthStreamInterceptors = nil

	// Build chain with no pre-auth interceptors (default behavior)
	jwtAuthInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}

	unaryChain := []grpc.UnaryServerInterceptor{
		// Standard interceptors would be here
	}
	// No pre-auth interceptors registered
	unaryChain = append(unaryChain, preAuthUnaryInterceptors...) // Empty slice
	unaryChain = append(unaryChain, jwtAuthInterceptor)

	// Should only have JWT auth interceptor
	if len(unaryChain) != 1 {
		t.Errorf("Expected 1 interceptor (JWT only), got %d", len(unaryChain))
	}

	// Verify it works normally
	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	result, err := unaryChain[0](ctx, "test", info, func(ctx context.Context, req interface{}) (interface{}, error) {
		return "no-preauth", nil
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != "no-preauth" {
		t.Errorf("Expected 'no-preauth', got %v", result)
	}
}
