package cmd

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/cmd/trex/environments"
	pkgserver "github.com/openshift-online/rh-trex-ai/pkg/server"
)

func NewServeCommand(getSpecData func() ([]byte, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the application",
		Long:  "Serve the application.",
		Run: func(cmd *cobra.Command, args []string) {
			runServe(getSpecData)
		},
	}
	err := environments.Environment().AddFlags(cmd.PersistentFlags())
	if err != nil {
		glog.Fatalf("Unable to add environment flags to serve command: %s", err.Error())
	}

	return cmd
}

func runServe(getSpecData func() ([]byte, error)) {
	env := environments.Environment()
	err := env.Initialize()
	if err != nil {
		glog.Fatalf("Unable to initialize environment: %s", err.Error())
	}

	specData, err := getSpecData()
	if err != nil {
		glog.Fatalf("Unable to load OpenAPI spec: %s", err.Error())
	}

	go func() {
		apiserver := pkgserver.NewDefaultAPIServer(env, specData)
		apiserver.Start()
	}()

	go func() {
		metricsServer := pkgserver.NewDefaultMetricsServer(env)
		metricsServer.Start()
	}()

	go func() {
		healthcheckServer := pkgserver.NewDefaultHealthCheckServer(env)
		healthcheckServer.Start()
	}()

	go func() {
		controllersServer := pkgserver.NewDefaultControllersServer(env)
		controllersServer.Start()
	}()

	select {}
}
