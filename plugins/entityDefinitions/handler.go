package entityDefinitions

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

var _ handlers.RestHandler = entityDefinitionHandler{}

type entityDefinitionHandler struct {
	entityDefinition EntityDefinitionService
	generic          services.GenericService
}

func NewEntityDefinitionHandler(entityDefinition EntityDefinitionService, generic services.GenericService) *entityDefinitionHandler {
	return &entityDefinitionHandler{
		entityDefinition: entityDefinition,
		generic:          generic,
	}
}

func (h entityDefinitionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var entityDefinition openapi.EntityDefinition
	cfg := &handlers.HandlerConfig{
		Body: &entityDefinition,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&entityDefinition, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			entityDefinitionModel := ConvertEntityDefinition(entityDefinition)
			entityDefinitionModel, err := h.entityDefinition.Create(ctx, entityDefinitionModel)
			if err != nil {
				return nil, err
			}
			return PresentEntityDefinition(entityDefinitionModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h entityDefinitionHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.EntityDefinitionPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.entityDefinition.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.ProjectId != nil {
				found.ProjectId = *patch.ProjectId
			}
			if patch.KindName != nil {
				found.KindName = *patch.KindName
			}
			if patch.PluralOverride != nil {
				found.PluralOverride = patch.PluralOverride
			}
			if patch.Description != nil {
				found.Description = patch.Description
			}

			entityDefinitionModel, err := h.entityDefinition.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentEntityDefinition(entityDefinitionModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h entityDefinitionHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			if projectID := r.URL.Query().Get("project_id"); projectID != "" {
				filter := fmt.Sprintf("project_id = '%s'", projectID)
				if listArgs.Search != "" {
					listArgs.Search = fmt.Sprintf("(%s) and (%s)", listArgs.Search, filter)
				} else {
					listArgs.Search = filter
				}
			}
			var entityDefinitions []EntityDefinition
			paging, err := h.generic.List(ctx, "id", listArgs, &entityDefinitions)
			if err != nil {
				return nil, err
			}
			entityDefinitionList := openapi.EntityDefinitionList{
				Kind:  "EntityDefinitionList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.EntityDefinition{},
			}

			for _, entityDefinition := range entityDefinitions {
				converted := PresentEntityDefinition(&entityDefinition)
				entityDefinitionList.Items = append(entityDefinitionList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, entityDefinitionList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return entityDefinitionList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h entityDefinitionHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			entityDefinition, err := h.entityDefinition.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentEntityDefinition(entityDefinition), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h entityDefinitionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.entityDefinition.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
