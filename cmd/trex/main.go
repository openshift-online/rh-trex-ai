package main

import (
	"os"

	"github.com/spf13/cobra"

	generatedtui "github.com/openshift-online/rh-trex-ai/data/generated/tui"
	"github.com/openshift-online/rh-trex-ai/pkg/api"
	pkgcmd "github.com/openshift-online/rh-trex-ai/pkg/cmd"

	_ "github.com/openshift-online/rh-trex-ai/cmd/trex/environments"
	_ "github.com/openshift-online/rh-trex-ai/plugins/dinosaurs"
	_ "github.com/openshift-online/rh-trex-ai/plugins/events"
	_ "github.com/openshift-online/rh-trex-ai/plugins/fossils"
	_ "github.com/openshift-online/rh-trex-ai/plugins/generic"
	_ "github.com/openshift-online/rh-trex-ai/plugins/scientists"
)

// nolint
//
//go:generate go-bindata -nometadata -o ../../data/generated/openapi/openapi.go -pkg openapi -prefix ../../openapi/ ../../openapi

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	rootCmd := pkgcmd.NewRootCommand("trex", "rh-trex serves as a template for new microservices")
	rootCmd.AddCommand(
		pkgcmd.NewMigrateCommand("rh-trex"),
		pkgcmd.NewServeCommand(api.GetOpenAPISpec),
		pkgcmd.NewTUICommand(generatedtui.GetDescriptor),
	)
	return rootCmd
}
