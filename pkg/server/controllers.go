package server

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-online/rh-trex-ai/pkg/controllers"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

type ControllersServer struct {
	KindControllerManager *controllers.KindControllerManager
	SyncController        *controllers.SyncController
	Broker                *EventBroker
	SessionFactory        db.SessionFactory
	cancel                context.CancelFunc
	done                  chan struct{}
}

func (s *ControllersServer) Start() {
	log := logger.NewLogger(context.Background())
	log.Infof("Kind controller listening for events")
	s.SessionFactory.NewListener(context.Background(), "events", func(id string) {
		s.KindControllerManager.Handle(id)
		if s.Broker != nil {
			s.Broker.Publish(id)
		}
	})

	// Start sync-the-world controller for missed event recovery
	if s.SyncController != nil {
		s.SyncController.Start()
		log.Infof("Sync controller started for missed event recovery")
	}
}

func (s *ControllersServer) Stop() {
	log := logger.NewLogger(context.Background())
	log.Infof("Stopping controllers server")

	if s.SyncController != nil {
		if err := s.SyncController.Stop(); err != nil {
			log.Error(fmt.Sprintf("Error stopping sync controller: %v", err))
		}
	}

	if s.Broker != nil {
		s.Broker.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.done != nil {
		<-s.done
	}
	log.Infof("Controllers server stopped")
}

func NewDefaultControllersServer(env *environments.Env) *ControllersServer {
	var eventService services.EventService
	if locator := env.Services.GetService("Events"); locator != nil {
		eventService = locator.(services.EventServiceLocator)()
	}

	broker := NewEventBroker(256, eventService)
	env.Services.SetService("EventBroker", broker)

	kindControllerManager := controllers.NewKindControllerManager(
		db.NewAdvisoryLockFactory(env.Database.SessionFactory),
		eventService,
	)

	// Create sync controller for missed event recovery
	var syncController *controllers.SyncController
	if eventService != nil {
		syncController = controllers.NewSyncController(
			kindControllerManager,
			eventService,
			controllers.SyncControllerConfig{
				Interval:         5 * time.Minute, // Sync every 5 minutes
				MaxAge:           1 * time.Hour,   // Process events up to 1 hour old
				MaxEventsPerSync: 1000,            // Limit events per sync cycle
			},
		)
	}

	s := &ControllersServer{
		KindControllerManager: kindControllerManager,
		SyncController:        syncController,
		Broker:                broker,
		SessionFactory:        env.Database.SessionFactory,
	}

	LoadDiscoveredControllers(s.KindControllerManager, &env.Services)

	return s
}

func NewDefaultHealthCheckServer(env *environments.Env) *HealthCheckServer {
	return NewHealthCheckServer(ServerConfig{
		BindAddress:   env.Config.HealthCheck.BindAddress,
		EnableHTTPS:   env.Config.HealthCheck.EnableHTTPS,
		HTTPSCertFile: env.Config.Server.HTTPSCertFile,
		HTTPSKeyFile:  env.Config.Server.HTTPSKeyFile,
	})
}

func NewDefaultMetricsServer(env *environments.Env) Server {
	return NewMetricsServer(ServerConfig{
		BindAddress:   env.Config.Metrics.BindAddress,
		EnableHTTPS:   env.Config.Metrics.EnableHTTPS,
		HTTPSCertFile: env.Config.Server.HTTPSCertFile,
		HTTPSKeyFile:  env.Config.Server.HTTPSKeyFile,
	})
}

type ControllerRegistrationFunc func(manager *controllers.KindControllerManager, services ServicesInterface)

var controllerRegistry = make(map[string]ControllerRegistrationFunc)

func RegisterController(name string, registrationFunc ControllerRegistrationFunc) {
	controllerRegistry[name] = registrationFunc
}

func LoadDiscoveredControllers(manager *controllers.KindControllerManager, services ServicesInterface) {
	for _, registrationFunc := range controllerRegistry {
		registrationFunc(manager, services)
	}
}
