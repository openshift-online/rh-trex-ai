package entityDefinitions

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

type EntityDefinitionDao interface {
	Get(ctx context.Context, id string) (*EntityDefinition, error)
	Create(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, error)
	Replace(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (EntityDefinitionList, error)
	FindByProjectID(ctx context.Context, projectID string) (EntityDefinitionList, error)
	All(ctx context.Context) (EntityDefinitionList, error)
}

var _ EntityDefinitionDao = &sqlEntityDefinitionDao{}

type sqlEntityDefinitionDao struct {
	sessionFactory *db.SessionFactory
}

func NewEntityDefinitionDao(sessionFactory *db.SessionFactory) EntityDefinitionDao {
	return &sqlEntityDefinitionDao{sessionFactory: sessionFactory}
}

func (d *sqlEntityDefinitionDao) Get(ctx context.Context, id string) (*EntityDefinition, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var entityDefinition EntityDefinition
	if err := g2.Take(&entityDefinition, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &entityDefinition, nil
}

func (d *sqlEntityDefinitionDao) Create(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(entityDefinition).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return entityDefinition, nil
}

func (d *sqlEntityDefinitionDao) Replace(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(entityDefinition).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return entityDefinition, nil
}

func (d *sqlEntityDefinitionDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&EntityDefinition{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlEntityDefinitionDao) FindByIDs(ctx context.Context, ids []string) (EntityDefinitionList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	entityDefinitions := EntityDefinitionList{}
	if err := g2.Where("id in (?)", ids).Find(&entityDefinitions).Error; err != nil {
		return nil, err
	}
	return entityDefinitions, nil
}

func (d *sqlEntityDefinitionDao) FindByProjectID(ctx context.Context, projectID string) (EntityDefinitionList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	entityDefinitions := EntityDefinitionList{}
	if err := g2.Where("project_id = ?", projectID).Find(&entityDefinitions).Error; err != nil {
		return nil, err
	}
	return entityDefinitions, nil
}

func (d *sqlEntityDefinitionDao) All(ctx context.Context) (EntityDefinitionList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	entityDefinitions := EntityDefinitionList{}
	if err := g2.Find(&entityDefinitions).Error; err != nil {
		return nil, err
	}
	return entityDefinitions, nil
}
