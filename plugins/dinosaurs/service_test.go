package dinosaurs

import (
	"context"
	"testing"

	gm "github.com/onsi/gomega"

	dbmocks "github.com/openshift-online/rh-trex-ai/pkg/db/mocks"
	"github.com/openshift-online/rh-trex-ai/pkg/services"

	daomocks "github.com/openshift-online/rh-trex-ai/pkg/dao/mocks"
)

func TestDinosaurFindBySpecies(t *testing.T) {
	gm.RegisterTestingT(t)

	dinoDAO := NewMockDinosaurDao()
	events := services.NewEventService(daomocks.NewEventDao())
	dinoService := NewDinosaurService(dbmocks.NewMockAdvisoryLockFactory(), dinoDAO, events)

	const Fukuisaurus = "Fukuisaurus"
	const Seismosaurus = "Seismosaurus"
	const Breviceratops = "Breviceratops"

	dinos := DinosaurList{
		&Dinosaur{Species: Fukuisaurus},
		&Dinosaur{Species: Fukuisaurus},
		&Dinosaur{Species: Fukuisaurus},
		&Dinosaur{Species: Seismosaurus},
		&Dinosaur{Species: Seismosaurus},
		&Dinosaur{Species: Breviceratops},
	}
	for _, dino := range dinos {
		_, err := dinoService.Create(context.Background(), dino)
		gm.Expect(err).To(gm.BeNil())
	}
	fukuisaurus, err := dinoService.FindBySpecies(context.Background(), Fukuisaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(fukuisaurus)).To(gm.Equal(3))

	seismosaurus, err := dinoService.FindBySpecies(context.Background(), Seismosaurus)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(seismosaurus)).To(gm.Equal(2))

	breviceratops, err := dinoService.FindBySpecies(context.Background(), Breviceratops)
	gm.Expect(err).To(gm.BeNil())
	gm.Expect(len(breviceratops)).To(gm.Equal(1))
}
