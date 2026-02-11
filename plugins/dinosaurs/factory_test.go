package dinosaurs_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/cmd/trex/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/dinosaurs"
)

func newDinosaur(species string) (*dinosaurs.Dinosaur, error) {
	dinoService := dinosaurs.Service(&environments.Environment().Services)

	dinosaur := &dinosaurs.Dinosaur{
		Species: species,
	}

	dino, err := dinoService.Create(context.Background(), dinosaur)
	if err != nil {
		return nil, err
	}

	return dino, nil
}

func newDinosaurList(namePrefix string, count int) ([]*dinosaurs.Dinosaur, error) {
	var dinoList []*dinosaurs.Dinosaur
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newDinosaur(name)
		if err != nil {
			return nil, err
		}
		dinoList = append(dinoList, c)
	}
	return dinoList, nil
}
