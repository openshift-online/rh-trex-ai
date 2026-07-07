package projects_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/projects"
)

func newProject(id string) (*projects.Project, error) {
	projectService := projects.Service(&environments.Environment().Services)

	project := &projects.Project{
		Name:          "test-name",
		Description:   stringPtr("test-description"),
		RepositoryUrl: stringPtr("test-repository_url"),
		Status:        "test-status",
	}

	sub, err := projectService.Create(context.Background(), project)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newProjectList(namePrefix string, count int) ([]*projects.Project, error) {
	var items []*projects.Project
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newProject(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
func stringPtr(s string) *string { return &s }
