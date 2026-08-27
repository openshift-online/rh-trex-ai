package scientists

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

var _ handlers.RestHandler = scientistHandler{}

type scientistHandler struct {
	scientist ScientistService
	generic   services.GenericService
}

func NewScientistHandler(scientist ScientistService, generic services.GenericService) *scientistHandler {
	return &scientistHandler{
		scientist: scientist,
		generic:   generic,
	}
}

func (h scientistHandler) Create(w http.ResponseWriter, r *http.Request) {
	var scientist openapi.Scientist
	cfg := &handlers.HandlerConfig{
		Body: &scientist,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&scientist, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			scientistModel := ConvertScientist(scientist)
			scientistModel, err := h.scientist.Create(ctx, scientistModel)
			if err != nil {
				return nil, err
			}
			return PresentScientist(scientistModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h scientistHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.ScientistPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.scientist.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.Name != nil {
				found.Name = *patch.Name
			}
			if patch.Field != nil {
				found.Field = *patch.Field
			}

			scientistModel, err := h.scientist.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentScientist(scientistModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h scientistHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var scientists []Scientist
			paging, err := h.generic.List(ctx, "id", listArgs, &scientists)
			if err != nil {
				return nil, err
			}
			scientistList := openapi.ScientistList{
				Kind:  "ScientistList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Scientist{},
			}

			for _, scientist := range scientists {
				converted := PresentScientist(&scientist)
				scientistList.Items = append(scientistList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, scientistList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return scientistList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h scientistHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			scientist, err := h.scientist.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentScientist(scientist), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h scientistHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.scientist.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
