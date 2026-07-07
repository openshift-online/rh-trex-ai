package entityDefinitions

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertEntityDefinition(entityDefinition openapi.EntityDefinition) *EntityDefinition {
	c := &EntityDefinition{
		Meta: api.Meta{
			ID: util.NilToEmptyString(entityDefinition.Id),
		},
	}
	c.ProjectId = entityDefinition.ProjectId
	c.KindName = entityDefinition.KindName
	c.PluralOverride = entityDefinition.PluralOverride
	c.Description = entityDefinition.Description

	if entityDefinition.CreatedAt != nil {
		c.CreatedAt = *entityDefinition.CreatedAt
		c.UpdatedAt = *entityDefinition.UpdatedAt
	}

	return c
}

func PresentEntityDefinition(entityDefinition *EntityDefinition) openapi.EntityDefinition {
	reference := presenters.PresentReference(entityDefinition.ID, entityDefinition)
	return openapi.EntityDefinition{
		Id:             reference.Id,
		Kind:           reference.Kind,
		Href:           reference.Href,
		CreatedAt:      openapi.PtrTime(entityDefinition.CreatedAt),
		UpdatedAt:      openapi.PtrTime(entityDefinition.UpdatedAt),
		ProjectId:      entityDefinition.ProjectId,
		KindName:       entityDefinition.KindName,
		PluralOverride: entityDefinition.PluralOverride,
		Description:    entityDefinition.Description,
	}
}
