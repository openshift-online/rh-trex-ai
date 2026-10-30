package get

import (
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/get/dinosaur"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/get/fossil"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/get/scientist"
)

var Cmd = &cobra.Command{
	Use:   "get RESOURCE ID",
	Short: "Get a specific resource by ID",
	Long:  "Get a specific resource by ID",
}

func init() {
	Cmd.AddCommand(dinosaur.Cmd)
	Cmd.AddCommand(fossil.Cmd)
	Cmd.AddCommand(scientist.Cmd)
}
