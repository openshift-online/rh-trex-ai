package dinosaurs

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

var _ handlers.RestHandler = dinosaurHandler{}

type dinosaurHandler struct {
	dinosaur DinosaurService
	generic  services.GenericService
}

func NewDinosaurHandler(dinosaur DinosaurService, generic services.GenericService) *dinosaurHandler {
	return &dinosaurHandler{
		dinosaur: dinosaur,
		generic:  generic,
	}
}

func (h dinosaurHandler) Create(w http.ResponseWriter, r *http.Request) {
	var dinosaur openapi.Dinosaur
	cfg := &handlers.HandlerConfig{
		Body: &dinosaur,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&dinosaur, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			dinosaurModel := ConvertDinosaur(dinosaur)
			dinosaurModel, err := h.dinosaur.Create(ctx, dinosaurModel)
			if err != nil {
				return nil, err
			}
			return PresentDinosaur(dinosaurModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h dinosaurHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.DinosaurPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.dinosaur.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.Species != nil {
				found.Species = *patch.Species
			}

			dinosaurModel, err := h.dinosaur.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentDinosaur(dinosaurModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h dinosaurHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			var dinosaurs []Dinosaur
			paging, err := h.generic.List(ctx, "id", listArgs, &dinosaurs)
			if err != nil {
				return nil, err
			}
			dinosaurList := openapi.DinosaurList{
				Kind:  "DinosaurList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Dinosaur{},
			}

			for _, dinosaur := range dinosaurs {
				converted := PresentDinosaur(&dinosaur)
				dinosaurList.Items = append(dinosaurList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, dinosaurList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return dinosaurList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h dinosaurHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			dinosaur, err := h.dinosaur.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentDinosaur(dinosaur), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h dinosaurHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.dinosaur.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
