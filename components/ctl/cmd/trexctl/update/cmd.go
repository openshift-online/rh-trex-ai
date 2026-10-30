package update

import (
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/update/dinosaur"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/update/fossil"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/update/scientist"
)

var Cmd = &cobra.Command{
	Use:   "update RESOURCE ID",
	Short: "Update a resource by ID",
	Long:  "Update a resource by ID, sending only the fields that were set.",
}

func init() {
	Cmd.AddCommand(dinosaur.Cmd)
	Cmd.AddCommand(fossil.Cmd)
	Cmd.AddCommand(scientist.Cmd)
}
