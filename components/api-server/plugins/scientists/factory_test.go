package scientists_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/components/api-server/plugins/scientists"
)

func newScientist(id string) (*scientists.Scientist, error) {
	scientistService := scientists.Service(&environments.Environment().Services)

	scientist := &scientists.Scientist{
		Name:  "test-name",
		Field: "test-field",
	}

	sub, err := scientistService.Create(context.Background(), scientist)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newScientistList(namePrefix string, count int) ([]*scientists.Scientist, error) {
	var items []*scientists.Scientist
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newScientist(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
