package entityDefinitions

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

var _ EntityDefinitionDao = &entityDefinitionDaoMock{}

type entityDefinitionDaoMock struct {
	entityDefinitions EntityDefinitionList
}

func NewMockEntityDefinitionDao() *entityDefinitionDaoMock {
	return &entityDefinitionDaoMock{}
}

func (d *entityDefinitionDaoMock) Get(ctx context.Context, id string) (*EntityDefinition, error) {
	for _, entityDefinition := range d.entityDefinitions {
		if entityDefinition.ID == id {
			return entityDefinition, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *entityDefinitionDaoMock) Create(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, error) {
	d.entityDefinitions = append(d.entityDefinitions, entityDefinition)
	return entityDefinition, nil
}

func (d *entityDefinitionDaoMock) Replace(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, error) {
	return nil, errors.NotImplemented("EntityDefinition").AsError()
}

func (d *entityDefinitionDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("EntityDefinition").AsError()
}

func (d *entityDefinitionDaoMock) FindByIDs(ctx context.Context, ids []string) (EntityDefinitionList, error) {
	return nil, errors.NotImplemented("EntityDefinition").AsError()
}

func (d *entityDefinitionDaoMock) FindByProjectID(ctx context.Context, projectID string) (EntityDefinitionList, error) {
	var result EntityDefinitionList
	for _, ed := range d.entityDefinitions {
		if ed.ProjectId == projectID {
			result = append(result, ed)
		}
	}
	return result, nil
}

func (d *entityDefinitionDaoMock) All(ctx context.Context) (EntityDefinitionList, error) {
	return d.entityDefinitions, nil
}
