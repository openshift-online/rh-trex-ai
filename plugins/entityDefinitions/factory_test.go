package entityDefinitions_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/entityDefinitions"
)

func newEntityDefinition(id string) (*entityDefinitions.EntityDefinition, error) {
	entityDefinitionService := entityDefinitions.Service(&environments.Environment().Services)

	entityDefinition := &entityDefinitions.EntityDefinition{
		ProjectId:      "test-project_id",
		KindName:       "test-kind_name",
		PluralOverride: stringPtr("test-plural_override"),
		Description:    stringPtr("test-description"),
	}

	sub, err := entityDefinitionService.Create(context.Background(), entityDefinition)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newEntityDefinitionList(namePrefix string, count int) ([]*entityDefinitions.EntityDefinition, error) {
	var items []*entityDefinitions.EntityDefinition
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newEntityDefinition(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
func stringPtr(s string) *string { return &s }
