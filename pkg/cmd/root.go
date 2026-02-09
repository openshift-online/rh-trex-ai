package cmd

import (
	"flag"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

func NewRootCommand(serviceName, description string) *cobra.Command {
	_ = flag.CommandLine.Parse([]string{})

	if err := flag.Set("logtostderr", "true"); err != nil {
		glog.Infof("Unable to set logtostderr to true")
	}

	rootCmd := &cobra.Command{
		Use:  serviceName,
		Long: description,
	}

	return rootCmd
}
