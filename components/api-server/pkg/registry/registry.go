package registry

import (
	"sync"
)

type ServiceLocatorFunc func(env interface{}) interface{}

type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]ServiceLocatorFunc
}

var globalRegistry = &ServiceRegistry{
	services: make(map[string]ServiceLocatorFunc),
}

func RegisterService(name string, locatorFunc ServiceLocatorFunc) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.services[name] = locatorFunc
}

type ServicesInterface interface {
	SetService(name string, service interface{})
}

func LoadDiscoveredServices(services ServicesInterface, env interface{}) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	for name, locatorFunc := range globalRegistry.services {
		serviceLocator := locatorFunc(env)
		services.SetService(name, serviceLocator)
	}
}
