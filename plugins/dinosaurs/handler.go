package dinosaurs

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex/pkg/api"
	"github.com/openshift-online/rh-trex/pkg/api/openapi"
	"github.com/openshift-online/rh-trex/pkg/api/presenters"
	"github.com/openshift-online/rh-trex/pkg/errors"
	"github.com/openshift-online/rh-trex/pkg/handlers"
	"github.com/openshift-online/rh-trex/pkg/services"
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
			handlers.ValidateNotEmpty(&dinosaur, "Species", "species"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			dino := ConvertDinosaur(dinosaur)
			dino, err := h.dinosaur.Create(ctx, dino)
			if err != nil {
				return nil, err
			}
			return PresentDinosaur(dino), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h dinosaurHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.DinosaurPatchRequest

	cfg := &handlers.HandlerConfig{
		Body: &patch,
		Validators: []handlers.Validate{
			validateDinosaurPatch(&patch),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			dino, err := h.dinosaur.Replace(ctx, &Dinosaur{
				Meta:    api.Meta{ID: id},
				Species: *patch.Species,
			})
			if err != nil {
				return nil, err
			}
			return PresentDinosaur(dino), nil
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
			paging, err := h.generic.List(ctx, "username", listArgs, &dinosaurs)
			if err != nil {
				return nil, err
			}
			dinoList := openapi.DinosaurList{
				Kind:  "DinosaurList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Dinosaur{},
			}

			for _, dino := range dinosaurs {
				converted := PresentDinosaur(&dino)
				dinoList.Items = append(dinoList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, dinoList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return dinoList, nil
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

func validateDinosaurPatch(patch *openapi.DinosaurPatchRequest) handlers.Validate {
	return func() *errors.ServiceError {
		if patch.Species == nil {
			return errors.Validation("species cannot be nil")
		}
		if len(*patch.Species) == 0 {
			return errors.Validation("species cannot be empty")
		}
		return nil
	}
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
