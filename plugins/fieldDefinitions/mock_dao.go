package fieldDefinitions

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

var _ FieldDefinitionDao = &fieldDefinitionDaoMock{}

type fieldDefinitionDaoMock struct {
	fieldDefinitions FieldDefinitionList
}

func NewMockFieldDefinitionDao() *fieldDefinitionDaoMock {
	return &fieldDefinitionDaoMock{}
}

func (d *fieldDefinitionDaoMock) Get(ctx context.Context, id string) (*FieldDefinition, error) {
	for _, fieldDefinition := range d.fieldDefinitions {
		if fieldDefinition.ID == id {
			return fieldDefinition, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *fieldDefinitionDaoMock) Create(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, error) {
	d.fieldDefinitions = append(d.fieldDefinitions, fieldDefinition)
	return fieldDefinition, nil
}

func (d *fieldDefinitionDaoMock) Replace(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, error) {
	return nil, errors.NotImplemented("FieldDefinition").AsError()
}

func (d *fieldDefinitionDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("FieldDefinition").AsError()
}

func (d *fieldDefinitionDaoMock) FindByIDs(ctx context.Context, ids []string) (FieldDefinitionList, error) {
	return nil, errors.NotImplemented("FieldDefinition").AsError()
}

func (d *fieldDefinitionDaoMock) FindByEntityDefinitionID(ctx context.Context, entityDefinitionID string) (FieldDefinitionList, error) {
	var result FieldDefinitionList
	for _, fd := range d.fieldDefinitions {
		if fd.EntityDefinitionId == entityDefinitionID {
			result = append(result, fd)
		}
	}
	return result, nil
}

func (d *fieldDefinitionDaoMock) All(ctx context.Context) (FieldDefinitionList, error) {
	return d.fieldDefinitions, nil
}
