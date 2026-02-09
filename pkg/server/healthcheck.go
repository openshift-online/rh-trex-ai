package server

import (
	"context"
	"fmt"
	"net"
	"net/http"

	health "github.com/docker/go-healthcheck"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

var updater = health.NewStatusUpdater()

var _ Server = &HealthCheckServer{}

type HealthCheckServer struct {
	httpServer *http.Server
	config     ServerConfig
}

func NewHealthCheckServer(cfg ServerConfig) *HealthCheckServer {
	router := mux.NewRouter()
	health.DefaultRegistry = health.NewRegistry()
	health.Register("maintenance_status", updater)
	router.HandleFunc("/healthcheck", health.StatusHandler).Methods(http.MethodGet)
	router.HandleFunc("/healthcheck/down", downHandler).Methods(http.MethodPost)
	router.HandleFunc("/healthcheck/up", upHandler).Methods(http.MethodPost)

	srv := &http.Server{
		Handler: router,
		Addr:    cfg.BindAddress,
	}

	return &HealthCheckServer{
		httpServer: srv,
		config:     cfg,
	}
}

func (s HealthCheckServer) Start() {
	var err error
	if s.config.EnableHTTPS {
		if s.config.HTTPSCertFile == "" || s.config.HTTPSKeyFile == "" {
			Check(
				fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
				"Can't start https server",
				s.config.SentryTimeout,
			)
		}

		glog.Infof("Serving HealthCheck with TLS at %s", s.config.BindAddress)
		err = s.httpServer.ListenAndServeTLS(s.config.HTTPSCertFile, s.config.HTTPSKeyFile)
	} else {
		glog.Infof("Serving HealthCheck without TLS at %s", s.config.BindAddress)
		err = s.httpServer.ListenAndServe()
	}
	Check(err, "HealthCheck server terminated with errors", s.config.SentryTimeout)
	glog.Infof("HealthCheck server terminated")
}

func (s HealthCheckServer) Stop() error {
	return s.httpServer.Shutdown(context.Background())
}

func (s HealthCheckServer) Listen() (listener net.Listener, err error) {
	return nil, nil
}

func (s HealthCheckServer) Serve(listener net.Listener) {
}

func upHandler(w http.ResponseWriter, r *http.Request) {
	updater.Update(nil)
}

func downHandler(w http.ResponseWriter, r *http.Request) {
	updater.Update(fmt.Errorf("maintenance mode"))
}
