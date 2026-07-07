package relationships

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertRelationship(relationship openapi.Relationship) *Relationship {
	c := &Relationship{
		Meta: api.Meta{
			ID: util.NilToEmptyString(relationship.Id),
		},
	}
	c.ProjectId = relationship.ProjectId
	c.SourceEntityId = relationship.SourceEntityId
	c.TargetEntityId = relationship.TargetEntityId
	c.RelationshipType = relationship.RelationshipType
	c.ForeignKey = relationship.ForeignKey

	if relationship.CreatedAt != nil {
		c.CreatedAt = *relationship.CreatedAt
		c.UpdatedAt = *relationship.UpdatedAt
	}

	return c
}

func PresentRelationship(relationship *Relationship) openapi.Relationship {
	reference := presenters.PresentReference(relationship.ID, relationship)
	return openapi.Relationship{
		Id:               reference.Id,
		Kind:             reference.Kind,
		Href:             reference.Href,
		CreatedAt:        openapi.PtrTime(relationship.CreatedAt),
		UpdatedAt:        openapi.PtrTime(relationship.UpdatedAt),
		ProjectId:        relationship.ProjectId,
		SourceEntityId:   relationship.SourceEntityId,
		TargetEntityId:   relationship.TargetEntityId,
		RelationshipType: relationship.RelationshipType,
		ForeignKey:       relationship.ForeignKey,
	}
}
