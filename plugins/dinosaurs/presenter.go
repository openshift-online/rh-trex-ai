package dinosaurs

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertDinosaur(dinosaur openapi.Dinosaur) *Dinosaur {
	c := &Dinosaur{
		Meta: api.Meta{
			ID: util.NilToEmptyString(dinosaur.Id),
		},
	}
	c.Species = dinosaur.Species

	if dinosaur.CreatedAt != nil {
		c.CreatedAt = *dinosaur.CreatedAt
		c.UpdatedAt = *dinosaur.UpdatedAt
	}

	return c
}

func PresentDinosaur(dinosaur *Dinosaur) openapi.Dinosaur {
	reference := presenters.PresentReference(dinosaur.ID, dinosaur)
	return openapi.Dinosaur{
		Id:        reference.Id,
		Kind:      reference.Kind,
		Href:      reference.Href,
		CreatedAt: openapi.PtrTime(dinosaur.CreatedAt),
		UpdatedAt: openapi.PtrTime(dinosaur.UpdatedAt),
		Species:   dinosaur.Species,
	}
}
