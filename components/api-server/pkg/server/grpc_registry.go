package server

import (
	"fmt"

	"google.golang.org/grpc"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
)

type GRPCServiceRegistrationFunc func(grpcServer *grpc.Server, services ServicesInterface)

type grpcServiceEntry struct {
	register  GRPCServiceRegistrationFunc
	resources authz.GRPCResources
}

var grpcServiceRegistry = make(map[string]grpcServiceEntry)

// RegisterGRPCService registers a gRPC service and the resource of each of its
// methods. A method missing from resources is denied by authorization.
func RegisterGRPCService(name string, registrationFunc GRPCServiceRegistrationFunc, resources authz.GRPCResources) {
	grpcServiceRegistry[name] = grpcServiceEntry{register: registrationFunc, resources: resources}
}

func LoadDiscoveredGRPCServices(grpcServer *grpc.Server, services ServicesInterface) {
	for _, entry := range grpcServiceRegistry {
		entry.register(grpcServer, services)
	}
}

// grpcResources merges every registered method table. Two services that
// declare the same method are a programming error.
func grpcResources() (authz.GRPCResources, error) {
	all := authz.GRPCResources{}
	for name, entry := range grpcServiceRegistry {
		for method, res := range entry.resources {
			if _, dup := all[method]; dup {
				return nil, fmt.Errorf("gRPC method %s declared twice (service %q)", method, name)
			}
			all[method] = res
		}
	}
	return all, nil
}
