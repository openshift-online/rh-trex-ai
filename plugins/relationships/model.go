package relationships

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type Relationship struct {
	api.Meta
	ProjectId        string  `json:"project_id"`
	SourceEntityId   string  `json:"source_entity_id"`
	TargetEntityId   string  `json:"target_entity_id"`
	RelationshipType string  `json:"relationship_type"`
	ForeignKey       *string `json:"foreign_key"`
}

type RelationshipList []*Relationship
type RelationshipIndex map[string]*Relationship

func (l RelationshipList) Index() RelationshipIndex {
	index := RelationshipIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Relationship) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type RelationshipPatchRequest struct {
	ProjectId        *string `json:"project_id,omitempty"`
	SourceEntityId   *string `json:"source_entity_id,omitempty"`
	TargetEntityId   *string `json:"target_entity_id,omitempty"`
	RelationshipType *string `json:"relationship_type,omitempty"`
	ForeignKey       *string `json:"foreign_key,omitempty"`
}
