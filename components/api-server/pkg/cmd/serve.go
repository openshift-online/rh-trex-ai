package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/environments"
	pkgserver "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
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
		slog.Error("Unable to add environment flags to serve command", "error", err)
		os.Exit(1)
	}

	return cmd
}

func runServe(getSpecData func() ([]byte, error)) {
	env := environments.Environment()
	err := env.Initialize()
	if err != nil {
		slog.Error("Unable to initialize environment", "error", err)
		os.Exit(1)
	}

	specData, err := getSpecData()
	if err != nil {
		slog.Error("Unable to load OpenAPI spec", "error", err)
		os.Exit(1)
	}

	var servers []pkgserver.Server

	// Seal and export the audit trail in the background (SEC-004).
	env.Audit.Start()

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
	slog.Info("Received signal, shutting down", "signal", sig.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	controllersServer.Stop()
	slog.Info("Controllers server stopped")

	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(srv pkgserver.Server) {
			defer wg.Done()
			if err := srv.Stop(); err != nil {
				slog.Error("Error stopping server", "error", err)
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
		slog.Info("All servers stopped gracefully")
	case <-shutdownCtx.Done():
		slog.Warn("Shutdown timed out, forcing exit")
	}

	// Final sealing pass after the servers stop, so recent events are exported.
	if err := env.Audit.Stop(); err != nil {
		slog.Error("Error stopping audit sealer", "error", err)
	}
	slog.Info("Audit sealer stopped")

	_ = env.Database.SessionFactory.Close()
	slog.Info("Database connections closed")
}
