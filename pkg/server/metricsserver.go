package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/handlers"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
)

type metricsServer struct {
	httpServer *http.Server
	config     ServerConfig
}

var _ Server = &metricsServer{}

func NewMetricsServer(cfg ServerConfig) Server {
	mainRouter := mux.NewRouter()
	mainRouter.NotFoundHandler = http.HandlerFunc(api.SendNotFound)

	prometheusMetricsHandler := handlers.NewPrometheusMetricsHandler()
	mainRouter.Handle("/metrics", prometheusMetricsHandler.Handler())

	var mainHandler http.Handler = mainRouter

	s := &metricsServer{config: cfg}
	s.httpServer = &http.Server{
		Addr:    cfg.BindAddress,
		Handler: mainHandler,
	}
	return s
}

func (s metricsServer) Listen() (listener net.Listener, err error) {
	return nil, nil
}

func (s metricsServer) Serve(listener net.Listener) {
}

func (s metricsServer) Start() {
	log := logger.NewOCMLogger(context.Background())
	var err error
	if s.config.EnableHTTPS {
		if s.config.HTTPSCertFile == "" || s.config.HTTPSKeyFile == "" {
			Check(
				fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
				"Can't start https server",
				s.config.SentryTimeout,
			)
		}

		log.Infof("Serving Metrics with TLS at %s", s.config.BindAddress)
		err = s.httpServer.ListenAndServeTLS(s.config.HTTPSCertFile, s.config.HTTPSKeyFile)
	} else {
		log.Infof("Serving Metrics without TLS at %s", s.config.BindAddress)
		err = s.httpServer.ListenAndServe()
	}
	Check(err, "Metrics server terminated with errors", s.config.SentryTimeout)
	log.Infof("Metrics server terminated")
}

func (s metricsServer) Stop() error {
	return s.httpServer.Shutdown(context.Background())
}
