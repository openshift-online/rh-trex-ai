package relationships

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

var _ handlers.RestHandler = relationshipHandler{}

type relationshipHandler struct {
	relationship RelationshipService
	generic      services.GenericService
}

func NewRelationshipHandler(relationship RelationshipService, generic services.GenericService) *relationshipHandler {
	return &relationshipHandler{
		relationship: relationship,
		generic:      generic,
	}
}

func (h relationshipHandler) Create(w http.ResponseWriter, r *http.Request) {
	var relationship openapi.Relationship
	cfg := &handlers.HandlerConfig{
		Body: &relationship,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&relationship, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			relationshipModel := ConvertRelationship(relationship)
			relationshipModel, err := h.relationship.Create(ctx, relationshipModel)
			if err != nil {
				return nil, err
			}
			return PresentRelationship(relationshipModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h relationshipHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.RelationshipPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.relationship.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.ProjectId != nil {
				found.ProjectId = *patch.ProjectId
			}
			if patch.SourceEntityId != nil {
				found.SourceEntityId = *patch.SourceEntityId
			}
			if patch.TargetEntityId != nil {
				found.TargetEntityId = *patch.TargetEntityId
			}
			if patch.RelationshipType != nil {
				found.RelationshipType = *patch.RelationshipType
			}
			if patch.ForeignKey != nil {
				found.ForeignKey = patch.ForeignKey
			}

			relationshipModel, err := h.relationship.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentRelationship(relationshipModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h relationshipHandler) List(w http.ResponseWriter, r *http.Request) {
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
			var relationships []Relationship
			paging, err := h.generic.List(ctx, "id", listArgs, &relationships)
			if err != nil {
				return nil, err
			}
			relationshipList := openapi.RelationshipList{
				Kind:  "RelationshipList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Relationship{},
			}

			for _, relationship := range relationships {
				converted := PresentRelationship(&relationship)
				relationshipList.Items = append(relationshipList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, relationshipList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return relationshipList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h relationshipHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			relationship, err := h.relationship.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentRelationship(relationship), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h relationshipHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.relationship.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
