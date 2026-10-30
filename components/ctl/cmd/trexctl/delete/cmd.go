package delete

import (
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/delete/dinosaur"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/delete/fossil"
	"github.com/openshift-online/rh-trex-ai/components/ctl/cmd/trexctl/delete/scientist"
)

var Cmd = &cobra.Command{
	Use:   "delete RESOURCE ID",
	Short: "Delete a resource by ID",
	Long:  "Delete a resource by ID after asking for confirmation. Pass --yes to skip the prompt.",
}

func init() {
	Cmd.AddCommand(dinosaur.Cmd)
	Cmd.AddCommand(fossil.Cmd)
	Cmd.AddCommand(scientist.Cmd)
}
