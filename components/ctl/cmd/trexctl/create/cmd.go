package create

import (
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/create/dinosaur"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/create/fossil"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/create/scientist"
)

var Cmd = &cobra.Command{
	Use:   "create RESOURCE",
	Short: "Create a resource",
	Long:  "Create a resource",
}

func init() {
	Cmd.AddCommand(dinosaur.Cmd)
	Cmd.AddCommand(fossil.Cmd)
	Cmd.AddCommand(scientist.Cmd)
}
