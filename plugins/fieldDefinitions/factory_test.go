package fieldDefinitions_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/fieldDefinitions"
)

func newFieldDefinition(id string) (*fieldDefinitions.FieldDefinition, error) {
	fieldDefinitionService := fieldDefinitions.Service(&environments.Environment().Services)

	fieldDefinition := &fieldDefinitions.FieldDefinition{
		EntityDefinitionId: "test-entity_definition_id",
		FieldName:          "test-field_name",
		FieldType:          "test-field_type",
		IsRequired:         boolPtr(true),
	}

	sub, err := fieldDefinitionService.Create(context.Background(), fieldDefinition)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newFieldDefinitionList(namePrefix string, count int) ([]*fieldDefinitions.FieldDefinition, error) {
	var items []*fieldDefinitions.FieldDefinition
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newFieldDefinition(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
func boolPtr(b bool) *bool { return &b }
