package fossils

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/pkg/api/presenters"
	"github.com/openshift-online/rh-trex-ai/pkg/util"
)

func ConvertFossil(fossil openapi.Fossil) *Fossil {
	c := &Fossil{
		Meta: api.Meta{
			ID: util.NilToEmptyString(fossil.Id),
		},
	}
	c.DiscoveryLocation = fossil.DiscoveryLocation
	if fossil.EstimatedAge != nil {
		c.EstimatedAge = openapi.PtrInt(int(*fossil.EstimatedAge))
	}
	c.FossilType = fossil.FossilType
	c.ExcavatorName = fossil.ExcavatorName

	if fossil.CreatedAt != nil {
		c.CreatedAt = *fossil.CreatedAt
		c.UpdatedAt = *fossil.UpdatedAt
	}

	return c
}

func PresentFossil(fossil *Fossil) openapi.Fossil {
	reference := presenters.PresentReference(fossil.ID, fossil)
	return openapi.Fossil{
		Id:                reference.Id,
		Kind:              reference.Kind,
		Href:              reference.Href,
		CreatedAt:         openapi.PtrTime(fossil.CreatedAt),
		UpdatedAt:         openapi.PtrTime(fossil.UpdatedAt),
		DiscoveryLocation: fossil.DiscoveryLocation,
		EstimatedAge: func() *int32 {
			if fossil.EstimatedAge != nil {
				return openapi.PtrInt32(int32(*fossil.EstimatedAge))
			}
			return nil
		}(),
		FossilType:    fossil.FossilType,
		ExcavatorName: fossil.ExcavatorName,
	}
}
