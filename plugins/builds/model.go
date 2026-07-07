package builds

import (
	"time"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"gorm.io/gorm"
)

type Build struct {
	api.Meta
	ProjectId   string     `json:"project_id"`
	Status      string     `json:"status"`
	BuildLog    *string    `json:"build_log"`
	TriggeredBy *string    `json:"triggered_by"`
	CompletedAt *time.Time `json:"completed_at"`
}

type BuildList []*Build
type BuildIndex map[string]*Build

func (l BuildList) Index() BuildIndex {
	index := BuildIndex{}
	for _, o := range l {
		index[o.ID] = o
	}
	return index
}

func (d *Build) BeforeCreate(tx *gorm.DB) error {
	d.ID = api.NewID()
	return nil
}

type BuildPatchRequest struct {
	ProjectId   *string    `json:"project_id,omitempty"`
	Status      *string    `json:"status,omitempty"`
	BuildLog    *string    `json:"build_log,omitempty"`
	TriggeredBy *string    `json:"triggered_by,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
