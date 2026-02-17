package dinosaurs_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/dinosaurs"
)

func newDinosaur(id string) (*dinosaurs.Dinosaur, error) {
	dinosaurService := dinosaurs.Service(&environments.Environment().Services)

	dinosaur := &dinosaurs.Dinosaur{
		Species: "test-species",
	}

	sub, err := dinosaurService.Create(context.Background(), dinosaur)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newDinosaurList(namePrefix string, count int) ([]*dinosaurs.Dinosaur, error) {
	var items []*dinosaurs.Dinosaur
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newDinosaur(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
