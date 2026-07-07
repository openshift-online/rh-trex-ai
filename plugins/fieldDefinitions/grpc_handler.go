package fieldDefinitions

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

type fieldDefinitionGRPCHandler struct {
	pb.UnimplementedFieldDefinitionServiceServer
	service    FieldDefinitionService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewFieldDefinitionGRPCHandler(svc FieldDefinitionService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.FieldDefinitionServiceServer {
	return &fieldDefinitionGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *fieldDefinitionGRPCHandler) GetFieldDefinition(ctx context.Context, req *pb.GetFieldDefinitionRequest) (*pb.FieldDefinition, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	fieldDefinition, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return fieldDefinitionToProto(fieldDefinition), nil
}

func (h *fieldDefinitionGRPCHandler) CreateFieldDefinition(ctx context.Context, req *pb.CreateFieldDefinitionRequest) (*pb.FieldDefinition, error) {
	if err := grpcutil.ValidateStringField("entity_definition_id", req.EntityDefinitionId, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("field_name", req.FieldName, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("field_type", req.FieldType, true); err != nil {
		return nil, err
	}

	fieldDefinition := &FieldDefinition{
		EntityDefinitionId: req.EntityDefinitionId,
		FieldName:          req.FieldName,
		FieldType:          req.FieldType,
		IsRequired:         req.IsRequired,
	}
	result, svcErr := h.service.Create(ctx, fieldDefinition)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return fieldDefinitionToProto(result), nil
}

func (h *fieldDefinitionGRPCHandler) UpdateFieldDefinition(ctx context.Context, req *pb.UpdateFieldDefinitionRequest) (*pb.FieldDefinition, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.EntityDefinitionId != nil {
		if err := grpcutil.ValidateStringField("entity_definition_id", *req.EntityDefinitionId, false); err != nil {
			return nil, err
		}
	}
	if req.FieldName != nil {
		if err := grpcutil.ValidateStringField("field_name", *req.FieldName, false); err != nil {
			return nil, err
		}
	}
	if req.FieldType != nil {
		if err := grpcutil.ValidateStringField("field_type", *req.FieldType, false); err != nil {
			return nil, err
		}
	}

	fieldDefinition, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.EntityDefinitionId != nil {
		fieldDefinition.EntityDefinitionId = *req.EntityDefinitionId
	}
	if req.FieldName != nil {
		fieldDefinition.FieldName = *req.FieldName
	}
	if req.FieldType != nil {
		fieldDefinition.FieldType = *req.FieldType
	}
	if req.IsRequired != nil {
		fieldDefinition.IsRequired = req.IsRequired
	}
	result, svcErr := h.service.Replace(ctx, fieldDefinition)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return fieldDefinitionToProto(result), nil
}

func (h *fieldDefinitionGRPCHandler) DeleteFieldDefinition(ctx context.Context, req *pb.DeleteFieldDefinitionRequest) (*pb.DeleteFieldDefinitionResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteFieldDefinitionResponse{}, nil
}

func (h *fieldDefinitionGRPCHandler) ListFieldDefinitions(ctx context.Context, req *pb.ListFieldDefinitionsRequest) (*pb.ListFieldDefinitionsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var fieldDefinitions []FieldDefinition
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &fieldDefinitions)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.FieldDefinition, len(fieldDefinitions))
	for i, d := range fieldDefinitions {
		items[i] = fieldDefinitionToProto(&d)
	}

	return &pb.ListFieldDefinitionsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *fieldDefinitionGRPCHandler) WatchFieldDefinitions(req *pb.WatchFieldDefinitionsRequest, stream grpc.ServerStreamingServer[pb.FieldDefinitionWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	glog.V(4).Infof("WatchFieldDefinitions: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchFieldDefinitions: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "FieldDefinitions" {
				continue
			}

			watchEvent := &pb.FieldDefinitionWatchEvent{
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				fieldDefinition, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchFieldDefinitions: failed to load fieldDefinition %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.FieldDefinition = fieldDefinitionToProto(fieldDefinition)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchFieldDefinitions: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
