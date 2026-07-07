package builds_test

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/builds"
)

func newBuild(id string) (*builds.Build, error) {
	buildService := builds.Service(&environments.Environment().Services)

	build := &builds.Build{
		ProjectId:   "test-project_id",
		Status:      "test-status",
		BuildLog:    stringPtr("test-build_log"),
		TriggeredBy: stringPtr("test-triggered_by"),
		CompletedAt: timePtr(time.Now()),
	}

	sub, err := buildService.Create(context.Background(), build)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newBuildList(namePrefix string, count int) ([]*builds.Build, error) {
	var items []*builds.Build
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newBuild(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
func stringPtr(s string) *string     { return &s }
func timePtr(t time.Time) *time.Time { return &t }
