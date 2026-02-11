package api

// Package api provides OpenAPI specification embedding for the TRex template.
//
// WHY THIS FILE EXISTS:
// - The TRex serve command expects a GetOpenAPISpec function for API documentation
// - Generated plugins may extend this with their own OpenAPI specifications
// - Provides a minimal OpenAPI spec for the template to serve
// - Allows the template to compile and serve independently as a valid API
//
// This is a STUB implementation that provides minimal OpenAPI spec.
// Real OpenAPI specifications should be generated from your API definitions
// and embedded here for serving via the API documentation endpoints.

func GetOpenAPISpec() ([]byte, error) {
	// Return empty spec for minimal template - implement custom OpenAPI as needed
	spec := `
openapi: 3.0.0
info:
  title: My Service API
  version: 1.0.0
paths:
  /health:
    get:
      summary: Health check
      responses:
        '200':
          description: Service is healthy
`
	return []byte(spec), nil
}