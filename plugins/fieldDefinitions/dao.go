package fieldDefinitions

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

type FieldDefinitionDao interface {
	Get(ctx context.Context, id string) (*FieldDefinition, error)
	Create(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, error)
	Replace(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (FieldDefinitionList, error)
	FindByEntityDefinitionID(ctx context.Context, entityDefinitionID string) (FieldDefinitionList, error)
	All(ctx context.Context) (FieldDefinitionList, error)
}

var _ FieldDefinitionDao = &sqlFieldDefinitionDao{}

type sqlFieldDefinitionDao struct {
	sessionFactory *db.SessionFactory
}

func NewFieldDefinitionDao(sessionFactory *db.SessionFactory) FieldDefinitionDao {
	return &sqlFieldDefinitionDao{sessionFactory: sessionFactory}
}

func (d *sqlFieldDefinitionDao) Get(ctx context.Context, id string) (*FieldDefinition, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var fieldDefinition FieldDefinition
	if err := g2.Take(&fieldDefinition, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &fieldDefinition, nil
}

func (d *sqlFieldDefinitionDao) Create(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(fieldDefinition).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return fieldDefinition, nil
}

func (d *sqlFieldDefinitionDao) Replace(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(fieldDefinition).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return fieldDefinition, nil
}

func (d *sqlFieldDefinitionDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&FieldDefinition{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlFieldDefinitionDao) FindByIDs(ctx context.Context, ids []string) (FieldDefinitionList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	fieldDefinitions := FieldDefinitionList{}
	if err := g2.Where("id in (?)", ids).Find(&fieldDefinitions).Error; err != nil {
		return nil, err
	}
	return fieldDefinitions, nil
}

func (d *sqlFieldDefinitionDao) FindByEntityDefinitionID(ctx context.Context, entityDefinitionID string) (FieldDefinitionList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	fieldDefinitions := FieldDefinitionList{}
	if err := g2.Where("entity_definition_id = ?", entityDefinitionID).Find(&fieldDefinitions).Error; err != nil {
		return nil, err
	}
	return fieldDefinitions, nil
}

func (d *sqlFieldDefinitionDao) All(ctx context.Context) (FieldDefinitionList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	fieldDefinitions := FieldDefinitionList{}
	if err := g2.Find(&fieldDefinitions).Error; err != nil {
		return nil, err
	}
	return fieldDefinitions, nil
}
