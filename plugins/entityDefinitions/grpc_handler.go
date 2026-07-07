package entityDefinitions

import (
	"context"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	pkgserver "github.com/openshift-online/rh-trex-ai/pkg/server"
	"github.com/openshift-online/rh-trex-ai/pkg/server/grpcutil"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

type entityDefinitionGRPCHandler struct {
	pb.UnimplementedEntityDefinitionServiceServer
	service    EntityDefinitionService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewEntityDefinitionGRPCHandler(svc EntityDefinitionService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.EntityDefinitionServiceServer {
	return &entityDefinitionGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *entityDefinitionGRPCHandler) GetEntityDefinition(ctx context.Context, req *pb.GetEntityDefinitionRequest) (*pb.EntityDefinition, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	entityDefinition, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return entityDefinitionToProto(entityDefinition), nil
}

func (h *entityDefinitionGRPCHandler) CreateEntityDefinition(ctx context.Context, req *pb.CreateEntityDefinitionRequest) (*pb.EntityDefinition, error) {
	if err := grpcutil.ValidateStringField("project_id", req.ProjectId, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("kind_name", req.KindName, true); err != nil {
		return nil, err
	}

	entityDefinition := &EntityDefinition{
		ProjectId:      req.ProjectId,
		KindName:       req.KindName,
		PluralOverride: req.PluralOverride,
		Description:    req.Description,
	}
	result, svcErr := h.service.Create(ctx, entityDefinition)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return entityDefinitionToProto(result), nil
}

func (h *entityDefinitionGRPCHandler) UpdateEntityDefinition(ctx context.Context, req *pb.UpdateEntityDefinitionRequest) (*pb.EntityDefinition, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.ProjectId != nil {
		if err := grpcutil.ValidateStringField("project_id", *req.ProjectId, false); err != nil {
			return nil, err
		}
	}
	if req.KindName != nil {
		if err := grpcutil.ValidateStringField("kind_name", *req.KindName, false); err != nil {
			return nil, err
		}
	}
	if req.PluralOverride != nil {
		if err := grpcutil.ValidateStringField("plural_override", *req.PluralOverride, false); err != nil {
			return nil, err
		}
	}
	if req.Description != nil {
		if err := grpcutil.ValidateStringField("description", *req.Description, false); err != nil {
			return nil, err
		}
	}

	entityDefinition, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.ProjectId != nil {
		entityDefinition.ProjectId = *req.ProjectId
	}
	if req.KindName != nil {
		entityDefinition.KindName = *req.KindName
	}
	if req.PluralOverride != nil {
		entityDefinition.PluralOverride = req.PluralOverride
	}
	if req.Description != nil {
		entityDefinition.Description = req.Description
	}
	result, svcErr := h.service.Replace(ctx, entityDefinition)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return entityDefinitionToProto(result), nil
}

func (h *entityDefinitionGRPCHandler) DeleteEntityDefinition(ctx context.Context, req *pb.DeleteEntityDefinitionRequest) (*pb.DeleteEntityDefinitionResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteEntityDefinitionResponse{}, nil
}

func (h *entityDefinitionGRPCHandler) ListEntityDefinitions(ctx context.Context, req *pb.ListEntityDefinitionsRequest) (*pb.ListEntityDefinitionsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var entityDefinitions []EntityDefinition
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &entityDefinitions)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.EntityDefinition, len(entityDefinitions))
	for i, d := range entityDefinitions {
		items[i] = entityDefinitionToProto(&d)
	}

	return &pb.ListEntityDefinitionsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *entityDefinitionGRPCHandler) WatchEntityDefinitions(req *pb.WatchEntityDefinitionsRequest, stream grpc.ServerStreamingServer[pb.EntityDefinitionWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	glog.V(4).Infof("WatchEntityDefinitions: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchEntityDefinitions: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "EntityDefinitions" {
				continue
			}

			watchEvent := &pb.EntityDefinitionWatchEvent{
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				entityDefinition, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchEntityDefinitions: failed to load entityDefinition %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.EntityDefinition = entityDefinitionToProto(entityDefinition)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchEntityDefinitions: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
