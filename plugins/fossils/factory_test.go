package fossils_test

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/plugins/fossils"
)

func newFossil(id string) (*fossils.Fossil, error) {
	fossilService := fossils.Service(&environments.Environment().Services)

	fossil := &fossils.Fossil{
		DiscoveryLocation: "test-discovery_location",
		EstimatedAge:      intPtr(42),
		FossilType:        stringPtr("test-fossil_type"),
		ExcavatorName:     stringPtr("test-excavator_name"),
	}

	sub, err := fossilService.Create(context.Background(), fossil)
	if err != nil {
		return nil, err
	}

	return sub, nil
}

func newFossilList(namePrefix string, count int) ([]*fossils.Fossil, error) {
	var items []*fossils.Fossil
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s_%d", namePrefix, i)
		c, err := newFossil(name)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}
func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }
