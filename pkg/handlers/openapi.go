package handlers

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/openshift-online/rh-trex/pkg/errors"
)

//go:embed openapi-ui.html
var openapiui embed.FS

type OpenAPIHandler struct {
	openAPIDefinitions []byte
	uiContent          []byte
}

func NewOpenAPIHandler(specData []byte) (*OpenAPIHandler, error) {
	data, err := yaml.YAMLToJSON(specData)
	if err != nil {
		return nil, errors.GeneralError(
			"can't convert OpenAPI specification from YAML to JSON: %v",
			err,
		)
	}
	glog.Info("Loaded OpenAPI specification")

	uiContent, err := fs.ReadFile(openapiui, "openapi-ui.html")
	if err != nil {
		return nil, errors.GeneralError(
			"can't load OpenAPI UI HTML from embedded file: %v",
			err,
		)
	}
	glog.Info("Loaded OpenAPI UI HTML from embedded file")

	return &OpenAPIHandler{
		openAPIDefinitions: data,
		uiContent:          uiContent,
	}, nil
}

func (h *OpenAPIHandler) GetOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.openAPIDefinitions)
}

func (h *OpenAPIHandler) GetOpenAPIUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.uiContent)
}
