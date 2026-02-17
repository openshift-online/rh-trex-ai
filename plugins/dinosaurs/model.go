package dinosaurs

import (
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type Dinosaur struct {
	api.Meta
	Species string `json:"species"`
}

type DinosaurList []*Dinosaur
type DinosaurIndex map[string]*Dinosaur

func (l DinosaurList) Index() DinosaurIndex {
	index := DinosaurIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Dinosaur) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type DinosaurPatchRequest struct {
	Species *string `json:"species,omitempty"`
}
