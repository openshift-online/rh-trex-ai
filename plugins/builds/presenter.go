package builds

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertBuild(build openapi.Build) *Build {
	c := &Build{
		Meta: api.Meta{
			ID: util.NilToEmptyString(build.Id),
		},
	}
	c.ProjectId = build.ProjectId
	c.Status = build.Status
	c.BuildLog = build.BuildLog
	c.TriggeredBy = build.TriggeredBy
	c.CompletedAt = build.CompletedAt

	if build.CreatedAt != nil {
		c.CreatedAt = *build.CreatedAt
		c.UpdatedAt = *build.UpdatedAt
	}

	return c
}

func PresentBuild(build *Build) openapi.Build {
	reference := presenters.PresentReference(build.ID, build)
	return openapi.Build{
		Id:          reference.Id,
		Kind:        reference.Kind,
		Href:        reference.Href,
		CreatedAt:   openapi.PtrTime(build.CreatedAt),
		UpdatedAt:   openapi.PtrTime(build.UpdatedAt),
		ProjectId:   build.ProjectId,
		Status:      build.Status,
		BuildLog:    build.BuildLog,
		TriggeredBy: build.TriggeredBy,
		CompletedAt: build.CompletedAt,
	}
}
