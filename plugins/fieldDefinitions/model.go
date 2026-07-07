package fieldDefinitions

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type FieldDefinition struct {
	api.Meta
	EntityDefinitionId string `json:"entity_definition_id"`
	FieldName          string `json:"field_name"`
	FieldType          string `json:"field_type"`
	IsRequired         *bool  `json:"is_required"`
}

type FieldDefinitionList []*FieldDefinition
type FieldDefinitionIndex map[string]*FieldDefinition

func (l FieldDefinitionList) Index() FieldDefinitionIndex {
	index := FieldDefinitionIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *FieldDefinition) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type FieldDefinitionPatchRequest struct {
	EntityDefinitionId *string `json:"entity_definition_id,omitempty"`
	FieldName          *string `json:"field_name,omitempty"`
	FieldType          *string `json:"field_type,omitempty"`
	IsRequired         *bool   `json:"is_required,omitempty"`
}
