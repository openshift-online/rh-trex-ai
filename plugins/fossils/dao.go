package fossils

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

type FossilDao interface {
	Get(ctx context.Context, id string) (*Fossil, error)
	Create(ctx context.Context, fossil *Fossil) (*Fossil, error)
	Replace(ctx context.Context, fossil *Fossil) (*Fossil, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (FossilList, error)
	All(ctx context.Context) (FossilList, error)
}

var _ FossilDao = &sqlFossilDao{}

type sqlFossilDao struct {
	sessionFactory *db.SessionFactory
}

func NewFossilDao(sessionFactory *db.SessionFactory) FossilDao {
	return &sqlFossilDao{sessionFactory: sessionFactory}
}

func (d *sqlFossilDao) Get(ctx context.Context, id string) (*Fossil, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var fossil Fossil
	if err := g2.Take(&fossil, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &fossil, nil
}

func (d *sqlFossilDao) Create(ctx context.Context, fossil *Fossil) (*Fossil, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(fossil).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return fossil, nil
}

func (d *sqlFossilDao) Replace(ctx context.Context, fossil *Fossil) (*Fossil, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(fossil).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return fossil, nil
}

func (d *sqlFossilDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&Fossil{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlFossilDao) FindByIDs(ctx context.Context, ids []string) (FossilList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	fossils := FossilList{}
	if err := g2.Where("id in (?)", ids).Find(&fossils).Error; err != nil {
		return nil, err
	}
	return fossils, nil
}

func (d *sqlFossilDao) All(ctx context.Context) (FossilList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	fossils := FossilList{}
	if err := g2.Find(&fossils).Error; err != nil {
		return nil, err
	}
	return fossils, nil
}
