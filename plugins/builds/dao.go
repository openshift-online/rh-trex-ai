package builds

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
)

type BuildDao interface {
	Get(ctx context.Context, id string) (*Build, error)
	Create(ctx context.Context, build *Build) (*Build, error)
	Replace(ctx context.Context, build *Build) (*Build, error)
	Delete(ctx context.Context, id string) error
	FindByIDs(ctx context.Context, ids []string) (BuildList, error)
	FindByProjectID(ctx context.Context, projectID string) (BuildList, error)
	All(ctx context.Context) (BuildList, error)
}

var _ BuildDao = &sqlBuildDao{}

type sqlBuildDao struct {
	sessionFactory *db.SessionFactory
}

func NewBuildDao(sessionFactory *db.SessionFactory) BuildDao {
	return &sqlBuildDao{sessionFactory: sessionFactory}
}

func (d *sqlBuildDao) Get(ctx context.Context, id string) (*Build, error) {
	g2 := (*d.sessionFactory).New(ctx)
	var build Build
	if err := g2.Take(&build, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &build, nil
}

func (d *sqlBuildDao) Create(ctx context.Context, build *Build) (*Build, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Create(build).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return build, nil
}

func (d *sqlBuildDao) Replace(ctx context.Context, build *Build) (*Build, error) {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Save(build).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return nil, err
	}
	return build, nil
}

func (d *sqlBuildDao) Delete(ctx context.Context, id string) error {
	g2 := (*d.sessionFactory).New(ctx)
	if err := g2.Omit(clause.Associations).Delete(&Build{Meta: api.Meta{ID: id}}).Error; err != nil {
		db.MarkForRollback(ctx, err)
		return err
	}
	return nil
}

func (d *sqlBuildDao) FindByIDs(ctx context.Context, ids []string) (BuildList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	builds := BuildList{}
	if err := g2.Where("id in (?)", ids).Find(&builds).Error; err != nil {
		return nil, err
	}
	return builds, nil
}

func (d *sqlBuildDao) FindByProjectID(ctx context.Context, projectID string) (BuildList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	builds := BuildList{}
	if err := g2.Where("project_id = ?", projectID).Find(&builds).Error; err != nil {
		return nil, err
	}
	return builds, nil
}

func (d *sqlBuildDao) All(ctx context.Context) (BuildList, error) {
	g2 := (*d.sessionFactory).New(ctx)
	builds := BuildList{}
	if err := g2.Find(&builds).Error; err != nil {
		return nil, err
	}
	return builds, nil
}
