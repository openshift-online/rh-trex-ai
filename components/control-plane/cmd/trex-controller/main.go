package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/config"
	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/grpcclient"
	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/reconciler"
	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/watcher"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	log.Printf("INFO trex-controller starting")
	log.Printf("INFO grpc=%s api=%s namespace=%s", cfg.GRPCServerAddr, cfg.APIServerURL, cfg.Namespace)

	creds, err := grpcclient.TransportCredentials(cfg)
	if err != nil {
		log.Fatalf("configuring gRPC transport: %v", err)
	}
	if cfg.GRPCTLS {
		caSource := "system roots"
		if cfg.GRPCTLSCAFile != "" {
			caSource = cfg.GRPCTLSCAFile
		}
		log.Printf("INFO grpc transport=tls ca=%s server_name=%q", caSource, cfg.GRPCTLSServerName)
	} else {
		log.Printf("INFO grpc transport=plaintext")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conn, err := grpc.NewClient(cfg.GRPCServerAddr,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		log.Fatalf("connecting to gRPC server: %v", err)
	}
	defer func() { _ = conn.Close() }()

	dinosaurReconciler := reconciler.NewDinosaurReconciler()
	fossilReconciler := reconciler.NewFossilReconciler()
	scientistReconciler := reconciler.NewScientistReconciler()

	errCh := make(chan error, 3)

	go func() { errCh <- watcher.WatchDinosaurs(ctx, conn, dinosaurReconciler) }()
	go func() { errCh <- watcher.WatchFossils(ctx, conn, fossilReconciler) }()
	go func() { errCh <- watcher.WatchScientists(ctx, conn, scientistReconciler) }()

	log.Printf("INFO all 3 watch streams launched")

	watchErr := <-errCh
	cancel()

	if watchErr != nil && watchErr != context.Canceled {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", watchErr)
		os.Exit(1)
	}

	log.Printf("INFO trex-controller stopped")
}
