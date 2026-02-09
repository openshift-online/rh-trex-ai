package main

import (
	"github.com/golang/glog"

	"github.com/openshift-online/rh-trex/pkg/api"
	pkgcmd "github.com/openshift-online/rh-trex/pkg/cmd"

	_ "github.com/openshift-online/rh-trex/plugins/dinosaurs"
	_ "github.com/openshift-online/rh-trex/plugins/events"
	_ "github.com/openshift-online/rh-trex/plugins/generic"
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
