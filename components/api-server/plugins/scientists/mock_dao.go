package scientists

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
)

var _ ScientistDao = &scientistDaoMock{}

type scientistDaoMock struct {
	scientists ScientistList
}

func NewMockScientistDao() *scientistDaoMock {
	return &scientistDaoMock{}
}

func (d *scientistDaoMock) Get(ctx context.Context, id string) (*Scientist, error) {
	for _, scientist := range d.scientists {
		if scientist.ID == id {
			return scientist, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *scientistDaoMock) Create(ctx context.Context, scientist *Scientist) (*Scientist, error) {
	d.scientists = append(d.scientists, scientist)
	return scientist, nil
}

func (d *scientistDaoMock) Replace(ctx context.Context, scientist *Scientist) (*Scientist, error) {
	return nil, errors.NotImplemented("Scientist").AsError()
}

func (d *scientistDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Scientist").AsError()
}

func (d *scientistDaoMock) FindByIDs(ctx context.Context, ids []string) (ScientistList, error) {
	return nil, errors.NotImplemented("Scientist").AsError()
}

func (d *scientistDaoMock) All(ctx context.Context) (ScientistList, error) {
	return d.scientists, nil
}
