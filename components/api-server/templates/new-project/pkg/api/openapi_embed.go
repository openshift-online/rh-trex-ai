//go:build ignore

package api

import (
	"embed"
	"io/fs"
)

// The spec is compiled into the binary, so the service finds it wherever it
// runs (container image, another host, a -trimpath build). The file is the
// bundled spec written by `make generate`; `make binary` runs that first when
// pkg/api/openapi is missing.
//
//go:embed openapi/api/openapi.yaml
var openapiFS embed.FS

// GetOpenAPISpec returns the OpenAPI spec embedded in the binary.
func GetOpenAPISpec() ([]byte, error) {
	return fs.ReadFile(openapiFS, "openapi/api/openapi.yaml")
}
