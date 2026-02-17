package fossils

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

var _ handlers.RestHandler = fossilHandler{}

type fossilHandler struct {
	fossil  FossilService
	generic services.GenericService
}

func NewFossilHandler(fossil FossilService, generic services.GenericService) *fossilHandler {
	return &fossilHandler{
		fossil:  fossil,
		generic: generic,
	}
}

func (h fossilHandler) Create(w http.ResponseWriter, r *http.Request) {
	var fossil openapi.Fossil
	cfg := &handlers.HandlerConfig{
		Body: &fossil,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&fossil, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			fossilModel := ConvertFossil(fossil)
			fossilModel, err := h.fossil.Create(ctx, fossilModel)
			if err != nil {
				return nil, err
			}
			return PresentFossil(fossilModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h fossilHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.FossilPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.fossil.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.DiscoveryLocation != nil {
				found.DiscoveryLocation = *patch.DiscoveryLocation
			}
			if patch.EstimatedAge != nil {
				estimatedAgeVal := int(*patch.EstimatedAge)
				found.EstimatedAge = &estimatedAgeVal
			}
			if patch.FossilType != nil {
				found.FossilType = patch.FossilType
			}
			if patch.ExcavatorName != nil {
				found.ExcavatorName = patch.ExcavatorName
			}

			fossilModel, err := h.fossil.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentFossil(fossilModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h fossilHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var fossils []Fossil
			paging, err := h.generic.List(ctx, "id", listArgs, &fossils)
			if err != nil {
				return nil, err
			}
			fossilList := openapi.FossilList{
				Kind:  "FossilList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Fossil{},
			}

			for _, fossil := range fossils {
				converted := PresentFossil(&fossil)
				fossilList.Items = append(fossilList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, fossilList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return fossilList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h fossilHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			fossil, err := h.fossil.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentFossil(fossil), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h fossilHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.fossil.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
