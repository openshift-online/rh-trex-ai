package generate

import (
	"net/http"

	"github.com/gorilla/mux"

	pkgserver "github.com/openshift-online/rh-trex-ai/pkg/server"
)

func init() {
	pkgserver.RegisterRootRoutes("generate", func(mainRouter *mux.Router) {
		h := NewGenerateHandler()
		generateRouter := mainRouter.PathPrefix("/generate").Subrouter()

		generateRouter.HandleFunc("/erd", h.GenerateERD).Methods(http.MethodPost)
		generateRouter.HandleFunc("/erd", h.LandingPage).Methods(http.MethodGet)
		generateRouter.HandleFunc("/{id}/{path:.*}", h.GetFile).Methods(http.MethodGet)
		generateRouter.HandleFunc("/{id}", h.GetResult).Methods(http.MethodGet)
		generateRouter.HandleFunc("", h.Generate).Methods(http.MethodPost)
	})
}
