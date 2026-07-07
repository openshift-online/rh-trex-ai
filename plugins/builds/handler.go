package builds

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

var _ handlers.RestHandler = buildHandler{}

type buildHandler struct {
	build   BuildService
	generic services.GenericService
}

func NewBuildHandler(build BuildService, generic services.GenericService) *buildHandler {
	return &buildHandler{
		build:   build,
		generic: generic,
	}
}

func (h buildHandler) Create(w http.ResponseWriter, r *http.Request) {
	var build openapi.Build
	cfg := &handlers.HandlerConfig{
		Body: &build,
		Validators: []handlers.Validate{
			handlers.ValidateEmpty(&build, "Id", "id"),
		},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			buildModel := ConvertBuild(build)
			buildModel, err := h.build.Create(ctx, buildModel)
			if err != nil {
				return nil, err
			}
			return PresentBuild(buildModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusCreated)
}

func (h buildHandler) Patch(w http.ResponseWriter, r *http.Request) {
	var patch openapi.BuildPatchRequest

	cfg := &handlers.HandlerConfig{
		Body:       &patch,
		Validators: []handlers.Validate{},
		Action: func() (interface{}, *errors.ServiceError) {
			ctx := r.Context()
			id := mux.Vars(r)["id"]
			found, err := h.build.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			if patch.ProjectId != nil {
				found.ProjectId = *patch.ProjectId
			}
			if patch.Status != nil {
				found.Status = *patch.Status
			}
			if patch.BuildLog != nil {
				found.BuildLog = patch.BuildLog
			}
			if patch.TriggeredBy != nil {
				found.TriggeredBy = patch.TriggeredBy
			}
			if patch.CompletedAt != nil {
				found.CompletedAt = patch.CompletedAt
			}

			buildModel, err := h.build.Replace(ctx, found)
			if err != nil {
				return nil, err
			}
			return PresentBuild(buildModel), nil
		},
		ErrorHandler: handlers.HandleError,
	}

	handlers.Handle(w, r, cfg, http.StatusOK)
}

func (h buildHandler) List(w http.ResponseWriter, r *http.Request) {
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
			var builds []Build
			paging, err := h.generic.List(ctx, "id", listArgs, &builds)
			if err != nil {
				return nil, err
			}
			buildList := openapi.BuildList{
				Kind:  "BuildList",
				Page:  int32(paging.Page),
				Size:  int32(paging.Size),
				Total: int32(paging.Total),
				Items: []openapi.Build{},
			}

			for _, build := range builds {
				converted := PresentBuild(&build)
				buildList.Items = append(buildList.Items, converted)
			}
			if listArgs.Fields != nil {
				filteredItems, err := presenters.SliceFilter(listArgs.Fields, buildList.Items)
				if err != nil {
					return nil, err
				}
				return filteredItems, nil
			}
			return buildList, nil
		},
	}

	handlers.HandleList(w, r, cfg)
}

func (h buildHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			build, err := h.build.Get(ctx, id)
			if err != nil {
				return nil, err
			}

			return PresentBuild(build), nil
		},
	}

	handlers.HandleGet(w, r, cfg)
}

func (h buildHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cfg := &handlers.HandlerConfig{
		Action: func() (interface{}, *errors.ServiceError) {
			id := mux.Vars(r)["id"]
			ctx := r.Context()
			err := h.build.Delete(ctx, id)
			if err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	handlers.HandleDelete(w, r, cfg, http.StatusNoContent)
}
