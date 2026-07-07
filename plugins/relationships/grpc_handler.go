package relationships

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

type relationshipGRPCHandler struct {
	pb.UnimplementedRelationshipServiceServer
	service    RelationshipService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewRelationshipGRPCHandler(svc RelationshipService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.RelationshipServiceServer {
	return &relationshipGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *relationshipGRPCHandler) GetRelationship(ctx context.Context, req *pb.GetRelationshipRequest) (*pb.Relationship, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	relationship, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return relationshipToProto(relationship), nil
}

func (h *relationshipGRPCHandler) CreateRelationship(ctx context.Context, req *pb.CreateRelationshipRequest) (*pb.Relationship, error) {
	if err := grpcutil.ValidateStringField("project_id", req.ProjectId, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("source_entity_id", req.SourceEntityId, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("target_entity_id", req.TargetEntityId, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("relationship_type", req.RelationshipType, true); err != nil {
		return nil, err
	}

	relationship := &Relationship{
		ProjectId:        req.ProjectId,
		SourceEntityId:   req.SourceEntityId,
		TargetEntityId:   req.TargetEntityId,
		RelationshipType: req.RelationshipType,
		ForeignKey:       req.ForeignKey,
	}
	result, svcErr := h.service.Create(ctx, relationship)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return relationshipToProto(result), nil
}

func (h *relationshipGRPCHandler) UpdateRelationship(ctx context.Context, req *pb.UpdateRelationshipRequest) (*pb.Relationship, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.ProjectId != nil {
		if err := grpcutil.ValidateStringField("project_id", *req.ProjectId, false); err != nil {
			return nil, err
		}
	}
	if req.SourceEntityId != nil {
		if err := grpcutil.ValidateStringField("source_entity_id", *req.SourceEntityId, false); err != nil {
			return nil, err
		}
	}
	if req.TargetEntityId != nil {
		if err := grpcutil.ValidateStringField("target_entity_id", *req.TargetEntityId, false); err != nil {
			return nil, err
		}
	}
	if req.RelationshipType != nil {
		if err := grpcutil.ValidateStringField("relationship_type", *req.RelationshipType, false); err != nil {
			return nil, err
		}
	}
	if req.ForeignKey != nil {
		if err := grpcutil.ValidateStringField("foreign_key", *req.ForeignKey, false); err != nil {
			return nil, err
		}
	}

	relationship, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.ProjectId != nil {
		relationship.ProjectId = *req.ProjectId
	}
	if req.SourceEntityId != nil {
		relationship.SourceEntityId = *req.SourceEntityId
	}
	if req.TargetEntityId != nil {
		relationship.TargetEntityId = *req.TargetEntityId
	}
	if req.RelationshipType != nil {
		relationship.RelationshipType = *req.RelationshipType
	}
	if req.ForeignKey != nil {
		relationship.ForeignKey = req.ForeignKey
	}
	result, svcErr := h.service.Replace(ctx, relationship)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return relationshipToProto(result), nil
}

func (h *relationshipGRPCHandler) DeleteRelationship(ctx context.Context, req *pb.DeleteRelationshipRequest) (*pb.DeleteRelationshipResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteRelationshipResponse{}, nil
}

func (h *relationshipGRPCHandler) ListRelationships(ctx context.Context, req *pb.ListRelationshipsRequest) (*pb.ListRelationshipsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var relationships []Relationship
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &relationships)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.Relationship, len(relationships))
	for i, d := range relationships {
		items[i] = relationshipToProto(&d)
	}

	return &pb.ListRelationshipsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *relationshipGRPCHandler) WatchRelationships(req *pb.WatchRelationshipsRequest, stream grpc.ServerStreamingServer[pb.RelationshipWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	glog.V(4).Infof("WatchRelationships: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchRelationships: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "Relationships" {
				continue
			}

			watchEvent := &pb.RelationshipWatchEvent{
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				relationship, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchRelationships: failed to load relationship %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.Relationship = relationshipToProto(relationship)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchRelationships: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
