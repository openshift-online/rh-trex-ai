package fossils

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type Fossil struct {
	api.Meta
	DiscoveryLocation string  `json:"discovery_location"`
	EstimatedAge      *int    `json:"estimated_age"`
	FossilType        *string `json:"fossil_type"`
	ExcavatorName     *string `json:"excavator_name"`
}

type FossilList []*Fossil
type FossilIndex map[string]*Fossil

func (l FossilList) Index() FossilIndex {
	index := FossilIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Fossil) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type FossilPatchRequest struct {
	DiscoveryLocation *string `json:"discovery_location,omitempty"`
	EstimatedAge      *int    `json:"estimated_age,omitempty"`
	FossilType        *string `json:"fossil_type,omitempty"`
	ExcavatorName     *string `json:"excavator_name,omitempty"`
}
