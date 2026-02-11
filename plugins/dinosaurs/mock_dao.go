package dinosaurs

import (
	"context"

	"gorm.io/gorm"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

var _ DinosaurDao = &dinosaurDaoMock{}

type dinosaurDaoMock struct {
	dinosaurs DinosaurList
}

func NewMockDinosaurDao() *dinosaurDaoMock {
	return &dinosaurDaoMock{}
}

func (d *dinosaurDaoMock) Get(ctx context.Context, id string) (*Dinosaur, error) {
	for _, dino := range d.dinosaurs {
		if dino.ID == id {
			return dino, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (d *dinosaurDaoMock) Create(ctx context.Context, dinosaur *Dinosaur) (*Dinosaur, error) {
	d.dinosaurs = append(d.dinosaurs, dinosaur)
	return dinosaur, nil
}

func (d *dinosaurDaoMock) Replace(ctx context.Context, dinosaur *Dinosaur) (*Dinosaur, error) {
	return nil, errors.NotImplemented("Dinosaur").AsError()
}

func (d *dinosaurDaoMock) Delete(ctx context.Context, id string) error {
	return errors.NotImplemented("Dinosaur").AsError()
}

func (d *dinosaurDaoMock) FindByIDs(ctx context.Context, ids []string) (DinosaurList, error) {
	return nil, errors.NotImplemented("Dinosaur").AsError()
}

func (d *dinosaurDaoMock) FindBySpecies(ctx context.Context, species string) (DinosaurList, error) {
	var dinos DinosaurList
	for _, dino := range d.dinosaurs {
		if dino.Species == species {
			dinos = append(dinos, dino)
		}
	}
	return dinos, nil
}

func (d *dinosaurDaoMock) All(ctx context.Context) (DinosaurList, error) {
	return d.dinosaurs, nil
}
