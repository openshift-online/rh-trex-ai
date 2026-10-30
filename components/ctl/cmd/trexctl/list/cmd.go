package list

import (
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/list/dinosaurs"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/list/fossils"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/list/scientists"
)

var Cmd = &cobra.Command{
	Use:   "list RESOURCE",
	Short: "List all resources of a specific type",
	Long:  "List all resources of a specific type",
}

func init() {
	Cmd.AddCommand(dinosaurs.Cmd)
	Cmd.AddCommand(fossils.Cmd)
	Cmd.AddCommand(scientists.Cmd)
}
