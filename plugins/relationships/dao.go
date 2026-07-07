package relationships

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

type RelationshipDao interface {
	Get(ctx context.Context, id string) (*Relationship, error)
	Create(ctx context.Context, relationship *Relationship) (*Relationship, error)
	Replace(ctx context.Context, relationship *Relationship) (*Relationship, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (RelationshipList, error)
	FindByProjectID(ctx context.Context, projectID string) (RelationshipList, error)
	FindByEntityID(ctx context.Context, entityID string) (RelationshipList, error)
	All(ctx context.Context) (RelationshipList, error)
}

var _ RelationshipDao = &sqlRelationshipDao{}

type sqlRelationshipDao struct {
	sessionFactory *db.SessionFactory
}

func NewRelationshipDao(sessionFactory *db.SessionFactory) RelationshipDao {
	return &sqlRelationshipDao{sessionFactory: sessionFactory}
}

func (d *sqlRelationshipDao) Get(ctx context.Context, id string) (*Relationship, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var relationship Relationship
	if err := g2.Take(&relationship, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &relationship, nil
}

func (d *sqlRelationshipDao) Create(ctx context.Context, relationship *Relationship) (*Relationship, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(relationship).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return relationship, nil
}

func (d *sqlRelationshipDao) Replace(ctx context.Context, relationship *Relationship) (*Relationship, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(relationship).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return relationship, nil
}

func (d *sqlRelationshipDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&Relationship{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlRelationshipDao) FindByIDs(ctx context.Context, ids []string) (RelationshipList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	relationships := RelationshipList{}
	if err := g2.Where("id in (?)", ids).Find(&relationships).Error; err != nil {
		return nil, err
	}
	return relationships, nil
}

func (d *sqlRelationshipDao) FindByProjectID(ctx context.Context, projectID string) (RelationshipList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	relationships := RelationshipList{}
	if err := g2.Where("project_id = ?", projectID).Find(&relationships).Error; err != nil {
		return nil, err
	}
	return relationships, nil
}

func (d *sqlRelationshipDao) FindByEntityID(ctx context.Context, entityID string) (RelationshipList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	relationships := RelationshipList{}
	if err := g2.Where("source_entity_id = ? OR target_entity_id = ?", entityID, entityID).Find(&relationships).Error; err != nil {
		return nil, err
	}
	return relationships, nil
}

func (d *sqlRelationshipDao) All(ctx context.Context) (RelationshipList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	relationships := RelationshipList{}
	if err := g2.Find(&relationships).Error; err != nil {
		return nil, err
	}
	return relationships, nil
}
