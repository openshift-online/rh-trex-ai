package main

import (
	"os"

	generatedtui "github.com/example/my-service/data/generated/tui"
	localapi "github.com/example/my-service/pkg/api"
	pkgcmd "github.com/openshift-online/rh-trex-ai/pkg/cmd"

	_ "github.com/example/my-service/cmd/my-service/environments"
	_ "github.com/openshift-online/rh-trex-ai/plugins/events"
	_ "github.com/openshift-online/rh-trex-ai/plugins/generic"
)

func main() {
	rootCmd := pkgcmd.NewRootCommand("my-service", "My service built with TRex library")
	rootCmd.AddCommand(
		pkgcmd.NewMigrateCommand("my-service"),
		pkgcmd.NewServeCommand(localapi.GetOpenAPISpec),
		pkgcmd.NewTUICommand(generatedtui.GetDescriptor),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
