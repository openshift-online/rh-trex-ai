package main

import (
	"github.com/golang/glog"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	pkgcmd "github.com/openshift-online/rh-trex-ai/pkg/cmd"

	_ "github.com/openshift-online/rh-trex-ai/cmd/trex/environments"
	_ "github.com/openshift-online/rh-trex-ai/plugins/dinosaurs"
	_ "github.com/openshift-online/rh-trex-ai/plugins/events"
	_ "github.com/openshift-online/rh-trex-ai/plugins/fossils"
	_ "github.com/openshift-online/rh-trex-ai/plugins/generic"
)

// nolint
//
//go:generate go-bindata -o ../../data/generated/openapi/openapi.go -pkg openapi -prefix ../../openapi/ ../../openapi

func main() {
	rootCmd := pkgcmd.NewRootCommand("trex", "rh-trex serves as a template for new microservices")
	rootCmd.AddCommand(
		pkgcmd.NewMigrateCommand("rh-trex"),
		pkgcmd.NewServeCommand(api.GetOpenAPISpec),
	)

	if err := rootCmd.Execute(); err != nil {
		glog.Fatalf("error running command: %v", err)
	}
}
