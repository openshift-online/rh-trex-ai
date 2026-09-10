package scientists

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"gorm.io/gorm"
)

type Scientist struct {
	api.Meta
	Name  string `json:"name"`
	Field string `json:"field"`
}

type ScientistList []*Scientist
type ScientistIndex map[string]*Scientist

func (l ScientistList) Index() ScientistIndex {
	index := ScientistIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Scientist) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type ScientistPatchRequest struct {
	Name  *string `json:"name,omitempty"`
	Field *string `json:"field,omitempty"`
}
