package fossils

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

var _ FossilDao = &fossilDaoMock{}

type fossilDaoMock struct {
	fossils FossilList
}

func NewMockFossilDao() *fossilDaoMock {
	return &fossilDaoMock{}
}

func (d *fossilDaoMock) Get(ctx context.Context, id string) (*Fossil, error) {
	for _, fossil := range d.fossils {
		if fossil.ID == id {
			return fossil, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *fossilDaoMock) Create(ctx context.Context, fossil *Fossil) (*Fossil, error) {
	d.fossils = append(d.fossils, fossil)
	return fossil, nil
}

func (d *fossilDaoMock) Replace(ctx context.Context, fossil *Fossil) (*Fossil, error) {
	return nil, errors.NotImplemented("Fossil").AsError()
}

func (d *fossilDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Fossil").AsError()
}

func (d *fossilDaoMock) FindByIDs(ctx context.Context, ids []string) (FossilList, error) {
	return nil, errors.NotImplemented("Fossil").AsError()
}

func (d *fossilDaoMock) All(ctx context.Context) (FossilList, error) {
	return d.fossils, nil
}
