package projects

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertProject(project openapi.Project) *Project {
	c := &Project{
		Meta: api.Meta{
			ID: util.NilToEmptyString(project.Id),
		},
	}
	c.Name = project.Name
	c.Description = project.Description
	c.RepositoryUrl = project.RepositoryUrl
	c.Status = project.Status

	if project.CreatedAt != nil {
		c.CreatedAt = *project.CreatedAt
		c.UpdatedAt = *project.UpdatedAt
	}

	return c
}

func PresentProject(project *Project) openapi.Project {
	reference := presenters.PresentReference(project.ID, project)
	return openapi.Project{
		Id:            reference.Id,
		Kind:          reference.Kind,
		Href:          reference.Href,
		CreatedAt:     openapi.PtrTime(project.CreatedAt),
		UpdatedAt:     openapi.PtrTime(project.UpdatedAt),
		Name:          project.Name,
		Description:   project.Description,
		RepositoryUrl: project.RepositoryUrl,
		Status:        project.Status,
	}
}
