package relationships_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/relationships"
)

func newRelationship(id string) (*relationships.Relationship, error) {
	relationshipService := relationships.Service(&environments.Environment().Services)

	relationship := &relationships.Relationship{
		ProjectId:        "test-project_id",
		SourceEntityId:   "test-source_entity_id",
		TargetEntityId:   "test-target_entity_id",
		RelationshipType: "test-relationship_type",
		ForeignKey:       stringPtr("test-foreign_key"),
	}

	sub, err := relationshipService.Create(context.Background(), relationship)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newRelationshipList(namePrefix string, count int) ([]*relationships.Relationship, error) {
	var items []*relationships.Relationship
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newRelationship(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
func stringPtr(s string) *string { return &s }
