//go:build ignore

package main

import (
	"os"

	localapi "github.com/example/my-service/pkg/api"
	pkgcmd "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/cmd"

	_ "github.com/example/my-service/cmd/my-service/environments"
	_ "github.com/openshift-online/rh-trex-ai/components/api-server/plugins/events"
	_ "github.com/openshift-online/rh-trex-ai/components/api-server/plugins/generic"
)

func main() {
	rootCmd := pkgcmd.NewRootCommand("my-service", "My service built with TRex library")
	rootCmd.AddCommand(
		pkgcmd.NewMigrateCommand("my-service"),
		pkgcmd.NewServeCommand(localapi.GetOpenAPISpec),
		pkgcmd.NewGenerateEntityCommand(),
		pkgcmd.NewProjectCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
