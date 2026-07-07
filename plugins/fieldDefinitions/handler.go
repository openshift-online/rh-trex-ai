package fieldDefinitions

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

var _ handlers.RestHandler = fieldDefinitionHandler{}

type fieldDefinitionHandler struct {
	fieldDefinition FieldDefinitionService
	generic         services.GenericService
}

func NewFieldDefinitionHandler(fieldDefinition FieldDefinitionService, generic services.GenericService) *fieldDefinitionHandler {
	return &fieldDefinitionHandler{
		fieldDefinition: fieldDefinition,
		generic:         generic,
	}
}

func (h fieldDefinitionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var fieldDefinition openapi.FieldDefinition
	cfg := &handlers.HandlerConfig{
		Body: &fieldDefinition,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&fieldDefinition, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			fieldDefinitionModel := ConvertFieldDefinition(fieldDefinition)
			fieldDefinitionModel, err := h.fieldDefinition.Create(ctx, fieldDefinitionModel)
			if err != nil {
				return nil, err
			}
			return PresentFieldDefinition(fieldDefinitionModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h fieldDefinitionHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.FieldDefinitionPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.fieldDefinition.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.EntityDefinitionId != nil {
				found.EntityDefinitionId = *patch.EntityDefinitionId
			}
			if patch.FieldName != nil {
				found.FieldName = *patch.FieldName
			}
			if patch.FieldType != nil {
				found.FieldType = *patch.FieldType
			}
			if patch.IsRequired != nil {
				found.IsRequired = patch.IsRequired
			}

			fieldDefinitionModel, err := h.fieldDefinition.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentFieldDefinition(fieldDefinitionModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h fieldDefinitionHandler) List(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()

			listArgs := services.NewListArguments(r.URL.Query())
			if entityDefID := r.URL.Query().Get("entity_definition_id"); entityDefID != "" {
				filter := fmt.Sprintf("entity_definition_id = '%s'", entityDefID)
				if listArgs.Search != "" {
					listArgs.Search = fmt.Sprintf("(%s) and (%s)", listArgs.Search, filter)
				} else {
					listArgs.Search = filter
				}
			}
			var fieldDefinitions []FieldDefinition
			paging, err := h.generic.List(ctx, "id", listArgs, &fieldDefinitions)
			if err != nil {
				return nil, err
			}
			fieldDefinitionList := openapi.FieldDefinitionList{
				Kind:  "FieldDefinitionList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.FieldDefinition{},
			}

			for _, fieldDefinition := range fieldDefinitions {
				converted := PresentFieldDefinition(&fieldDefinition)
				fieldDefinitionList.Items = append(fieldDefinitionList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, fieldDefinitionList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return fieldDefinitionList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h fieldDefinitionHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			fieldDefinition, err := h.fieldDefinition.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentFieldDefinition(fieldDefinition), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h fieldDefinitionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.fieldDefinition.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
