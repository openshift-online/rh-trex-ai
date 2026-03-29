package cmd

import (
	"flag"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const banner = `
 ████████╗██████╗ ███████╗██╗  ██╗
    ██╔══╝██╔══██╗██╔════╝╚██╗██╔╝
    ██║   ██████╔╝█████╗   ╚███╔╝ 
    ██║   ██╔══██╗██╔══╝   ██╔██╗ 
    ██║   ██║  ██║███████╗██╔╝ ██╗
    ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝

  TRex — REST + gRPC microservice template
`

func NewRootCommand(serviceName, description string) *cobra.Command {
	_ = flag.CommandLine.Parse([]string{})

	if err := flag.Set("logtostderr", "true"); err != nil {
		glog.Infof("Unable to set logtostderr to true")
	}

	rootCmd := &cobra.Command{
		Use:  serviceName,
		Long: description,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			fmt.Fprint(cmd.OutOrStderr(), banner)
		},
	}

	return rootCmd
}
