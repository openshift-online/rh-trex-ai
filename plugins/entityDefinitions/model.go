package entityDefinitions

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type EntityDefinition struct {
	api.Meta
	ProjectId      string  `json:"project_id"`
	KindName       string  `json:"kind_name"`
	PluralOverride *string `json:"plural_override"`
	Description    *string `json:"description"`
}

type EntityDefinitionList []*EntityDefinition
type EntityDefinitionIndex map[string]*EntityDefinition

func (l EntityDefinitionList) Index() EntityDefinitionIndex {
	index := EntityDefinitionIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *EntityDefinition) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type EntityDefinitionPatchRequest struct {
	ProjectId      *string `json:"project_id,omitempty"`
	KindName       *string `json:"kind_name,omitempty"`
	PluralOverride *string `json:"plural_override,omitempty"`
	Description    *string `json:"description,omitempty"`
}
