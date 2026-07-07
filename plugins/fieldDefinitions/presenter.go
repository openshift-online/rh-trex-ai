package fieldDefinitions

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertFieldDefinition(fieldDefinition openapi.FieldDefinition) *FieldDefinition {
	c := &FieldDefinition{
		Meta: api.Meta{
			ID: util.NilToEmptyString(fieldDefinition.Id),
		},
	}
	c.EntityDefinitionId = fieldDefinition.EntityDefinitionId
	c.FieldName = fieldDefinition.FieldName
	c.FieldType = fieldDefinition.FieldType
	c.IsRequired = fieldDefinition.IsRequired

	if fieldDefinition.CreatedAt != nil {
		c.CreatedAt = *fieldDefinition.CreatedAt
		c.UpdatedAt = *fieldDefinition.UpdatedAt
	}

	return c
}

func PresentFieldDefinition(fieldDefinition *FieldDefinition) openapi.FieldDefinition {
	reference := presenters.PresentReference(fieldDefinition.ID, fieldDefinition)
	return openapi.FieldDefinition{
		Id:                 reference.Id,
		Kind:               reference.Kind,
		Href:               reference.Href,
		CreatedAt:          openapi.PtrTime(fieldDefinition.CreatedAt),
		UpdatedAt:          openapi.PtrTime(fieldDefinition.UpdatedAt),
		EntityDefinitionId: fieldDefinition.EntityDefinitionId,
		FieldName:          fieldDefinition.FieldName,
		FieldType:          fieldDefinition.FieldType,
		IsRequired:         fieldDefinition.IsRequired,
	}
}
