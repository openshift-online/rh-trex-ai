package builds

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

var _ BuildDao = &buildDaoMock{}

type buildDaoMock struct {
	builds BuildList
}

func NewMockBuildDao() *buildDaoMock {
	return &buildDaoMock{}
}

func (d *buildDaoMock) Get(ctx context.Context, id string) (*Build, error) {
	for _, build := range d.builds {
		if build.ID == id {
			return build, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *buildDaoMock) Create(ctx context.Context, build *Build) (*Build, error) {
	d.builds = append(d.builds, build)
	return build, nil
}

func (d *buildDaoMock) Replace(ctx context.Context, build *Build) (*Build, error) {
	return nil, errors.NotImplemented("Build").AsError()
}

func (d *buildDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Build").AsError()
}

func (d *buildDaoMock) FindByIDs(ctx context.Context, ids []string) (BuildList, error) {
	return nil, errors.NotImplemented("Build").AsError()
}

func (d *buildDaoMock) FindByProjectID(ctx context.Context, projectID string) (BuildList, error) {
	var result BuildList
	for _, b := range d.builds {
		if b.ProjectId == projectID {
			result = append(result, b)
		}
	}
	return result, nil
}

func (d *buildDaoMock) All(ctx context.Context) (BuildList, error) {
	return d.builds, nil
}
