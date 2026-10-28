//go:build ignore

package api

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// projectRoot returns the module root (the directory containing openapi/).
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "../..")
}

// GetOpenAPISpec returns the OpenAPI spec for this service. It prefers the
// generated client copy (pkg/api/openapi/api/openapi.yaml, produced by
// `make generate`) and falls back to the source spec at openapi/openapi.yaml.
func GetOpenAPISpec() ([]byte, error) {
	root := projectRoot()

	paths := []string{
		filepath.Join(root, "pkg", "api", "openapi", "api", "openapi.yaml"),
		filepath.Join(root, "openapi", "openapi.yaml"),
	}
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("openapi spec not found (run make generate or check openapi/openapi.yaml)")
}
