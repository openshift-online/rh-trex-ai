package scientists

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/util"
)

func ConvertScientist(scientist openapi.Scientist) *Scientist {
	c := &Scientist{
		Meta: api.Meta{
			ID: util.NilToEmptyString(scientist.Id),
		},
	}
	c.Name = scientist.Name
	c.Field = scientist.Field

	if scientist.CreatedAt != nil {
		c.CreatedAt = *scientist.CreatedAt
		c.UpdatedAt = *scientist.UpdatedAt
	}

	return c
}

func PresentScientist(scientist *Scientist) openapi.Scientist {
	reference := presenters.PresentReference(scientist.ID, scientist)
	return openapi.Scientist{
		Id:        reference.Id,
		Kind:      reference.Kind,
		Href:      reference.Href,
		CreatedAt: openapi.PtrTime(scientist.CreatedAt),
		UpdatedAt: openapi.PtrTime(scientist.UpdatedAt),
		Name:      scientist.Name,
		Field:     scientist.Field,
	}
}

// ConvertScientistPatch turns a REST patch body into the model patch the service applies.
func ConvertScientistPatch(patch openapi.ScientistPatchRequest) *ScientistPatchRequest {
	c := &ScientistPatchRequest{}
	c.Name = patch.Name
	c.Field = patch.Field
	return c
}
