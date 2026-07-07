package relationships

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

var _ RelationshipDao = &relationshipDaoMock{}

type relationshipDaoMock struct {
	relationships RelationshipList
}

func NewMockRelationshipDao() *relationshipDaoMock {
	return &relationshipDaoMock{}
}

func (d *relationshipDaoMock) Get(ctx context.Context, id string) (*Relationship, error) {
	for _, relationship := range d.relationships {
		if relationship.ID == id {
			return relationship, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *relationshipDaoMock) Create(ctx context.Context, relationship *Relationship) (*Relationship, error) {
	d.relationships = append(d.relationships, relationship)
	return relationship, nil
}

func (d *relationshipDaoMock) Replace(ctx context.Context, relationship *Relationship) (*Relationship, error) {
	return nil, errors.NotImplemented("Relationship").AsError()
}

func (d *relationshipDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Relationship").AsError()
}

func (d *relationshipDaoMock) FindByIDs(ctx context.Context, ids []string) (RelationshipList, error) {
	return nil, errors.NotImplemented("Relationship").AsError()
}

func (d *relationshipDaoMock) FindByProjectID(ctx context.Context, projectID string) (RelationshipList, error) {
	var result RelationshipList
	for _, rel := range d.relationships {
		if rel.ProjectId == projectID {
			result = append(result, rel)
		}
	}
	return result, nil
}

func (d *relationshipDaoMock) FindByEntityID(ctx context.Context, entityID string) (RelationshipList, error) {
	var result RelationshipList
	for _, rel := range d.relationships {
		if rel.SourceEntityId == entityID || rel.TargetEntityId == entityID {
			result = append(result, rel)
		}
	}
	return result, nil
}

func (d *relationshipDaoMock) All(ctx context.Context) (RelationshipList, error) {
	return d.relationships, nil
}
