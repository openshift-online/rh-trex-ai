package scientists

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
)

type ScientistDao interface {
	Get(ctx context.Context, id string) (*Scientist, error)
	Create(ctx context.Context, scientist *Scientist) (*Scientist, error)
	Replace(ctx context.Context, scientist *Scientist) (*Scientist, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (ScientistList, error)
	All(ctx context.Context) (ScientistList, error)
}

var _ ScientistDao = &sqlScientistDao{}

type sqlScientistDao struct {
	sessionFactory *db.SessionFactory
}

func NewScientistDao(sessionFactory *db.SessionFactory) ScientistDao {
	return &sqlScientistDao{sessionFactory: sessionFactory}
}

func (d *sqlScientistDao) Get(ctx context.Context, id string) (*Scientist, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var scientist Scientist
	if err := g2.Take(&scientist, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &scientist, nil
}

func (d *sqlScientistDao) Create(ctx context.Context, scientist *Scientist) (*Scientist, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(scientist).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return scientist, nil
}

func (d *sqlScientistDao) Replace(ctx context.Context, scientist *Scientist) (*Scientist, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(scientist).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return scientist, nil
}

func (d *sqlScientistDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&Scientist{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlScientistDao) FindByIDs(ctx context.Context, ids []string) (ScientistList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	scientists := ScientistList{}
	if err := g2.Where("id in (?)", ids).Find(&scientists).Error; err != nil {
		return nil, err
	}
	return scientists, nil
}

func (d *sqlScientistDao) All(ctx context.Context) (ScientistList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	scientists := ScientistList{}
	if err := g2.Find(&scientists).Error; err != nil {
		return nil, err
	}
	return scientists, nil
}
