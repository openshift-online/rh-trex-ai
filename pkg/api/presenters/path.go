package presenters

import (
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

type PathMappingFunc func(interface{}) string

var pathRegistry = make(map[string]PathMappingFunc)

func RegisterPath(objType interface{}, pathValue string) {
	typeName := fmt.Sprintf("%T", objType)
	pathRegistry[typeName] = func(interface{}) string {
		return pathValue
	}
}

func LoadDiscoveredPaths(i interface{}) string {
	typeName := fmt.Sprintf("%T", i)
	if mappingFunc, found := pathRegistry[typeName]; found {
		return mappingFunc(i)
	}
	return ""
}

var basePath = "/api/rh-trex-ai/v1"

func BasePath() string        { return basePath }
func SetBasePath(path string) { basePath = path }

func ObjectPath(id string, obj interface{}) *string {
	return openapi.PtrString(fmt.Sprintf("%s/%s/%s", basePath, path(obj), id))
}

func path(i interface{}) string {
	// Check auto-discovered paths first
	if discoveredPath := LoadDiscoveredPaths(i); discoveredPath != "" {
		return discoveredPath
	}

	// Built-in mappings
	switch i.(type) {
	case errors.ServiceError, *errors.ServiceError:
		return "errors"
	default:
		return ""
	}
}
