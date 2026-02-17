package cmd

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
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

	var servers []pkgserver.Server

	controllersServer := pkgserver.NewDefaultControllersServer(env)
	go controllersServer.Start()

	apiServer := pkgserver.NewDefaultAPIServer(env, specData)
	servers = append(servers, apiServer)
	go apiServer.Start()

	if env.Config.GRPC.EnableGRPC {
		grpcServer := pkgserver.NewDefaultGRPCServer(env)
		servers = append(servers, grpcServer)
		go grpcServer.Start()
	}

	metricsServer := pkgserver.NewDefaultMetricsServer(env)
	servers = append(servers, metricsServer)
	go metricsServer.Start()

	healthCheckServer := pkgserver.NewDefaultHealthCheckServer(env)
	servers = append(servers, healthCheckServer)
	go healthCheckServer.Start()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	glog.Infof("Received signal %v, shutting down", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(srv pkgserver.Server) {
			defer wg.Done()
			if err := srv.Stop(); err != nil {
				glog.Errorf("Error stopping server: %v", err)
			}
		}(s)
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		glog.Info("All servers stopped gracefully")
	case <-shutdownCtx.Done():
		glog.Warning("Shutdown timed out, forcing exit")
	}

	env.Database.SessionFactory.Close()
	glog.Info("Database connections closed")
}
