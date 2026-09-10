package server

import (
	"google.golang.org/grpc"
)

type GRPCServiceRegistrationFunc func(grpcServer *grpc.Server, services ServicesInterface)

var grpcServiceRegistry = make(map[string]GRPCServiceRegistrationFunc)

func RegisterGRPCService(name string, registrationFunc GRPCServiceRegistrationFunc) {
	grpcServiceRegistry[name] = registrationFunc
}

func LoadDiscoveredGRPCServices(grpcServer *grpc.Server, services ServicesInterface) {
	for _, registrationFunc := range grpcServiceRegistry {
		registrationFunc(grpcServer, services)
	}
}
