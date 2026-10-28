package fossils

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	pkgserver "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server/grpcutil"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

type fossilGRPCHandler struct {
	pb.UnimplementedFossilServiceServer
	service    FossilService
	brokerFunc func() *pkgserver.EventBroker
}

func NewFossilGRPCHandler(svc FossilService, brokerFunc func() *pkgserver.EventBroker) pb.FossilServiceServer {
	return &fossilGRPCHandler{service: svc, brokerFunc: brokerFunc}
}

func (h *fossilGRPCHandler) GetFossil(ctx context.Context, req *pb.GetFossilRequest) (*pb.Fossil, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	fossil, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return fossilToProto(fossil), nil
}

func (h *fossilGRPCHandler) CreateFossil(ctx context.Context, req *pb.CreateFossilRequest) (*pb.Fossil, error) {
	fossil := &Fossil{
		DiscoveryLocation: req.DiscoveryLocation,
		EstimatedAge: func() *int {
			if req.EstimatedAge != nil {
				v := int(*req.EstimatedAge)
				return &v
			}
			return nil
		}(),
		FossilType:    req.FossilType,
		ExcavatorName: req.ExcavatorName,
	}
	result, svcErr := h.service.Create(ctx, fossil)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return fossilToProto(result), nil
}

func (h *fossilGRPCHandler) UpdateFossil(ctx context.Context, req *pb.UpdateFossilRequest) (*pb.Fossil, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	result, svcErr := h.service.Patch(ctx, req.Id, fossilPatchFromProto(req))
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return fossilToProto(result), nil
}

func (h *fossilGRPCHandler) DeleteFossil(ctx context.Context, req *pb.DeleteFossilRequest) (*pb.DeleteFossilResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteFossilResponse{}, nil
}

func (h *fossilGRPCHandler) ListFossils(ctx context.Context, req *pb.ListFossilsRequest) (*pb.ListFossilsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	fossils, paging, svcErr := h.service.List(ctx, listArgs)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.Fossil, len(fossils))
	for i, d := range fossils {
		items[i] = fossilToProto(&d)
	}

	return &pb.ListFossilsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *fossilGRPCHandler) WatchFossils(req *pb.WatchFossilsRequest, stream grpc.ServerStreamingServer[pb.FossilWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	slog.DebugContext(ctx, "WatchFossils: subscriber connected", "subscriber_id", sub.ID)

	for {
		select {
		case <-ctx.Done():
			slog.DebugContext(ctx, "WatchFossils: subscriber disconnected", "subscriber_id", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != FossilEventSource {
				continue
			}

			watchEvent := &pb.FossilWatchEvent{
				Type:       pb.EventType(grpcutil.APIEventTypeToProto(evt.EventType)),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				fossil, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					slog.WarnContext(ctx, "WatchFossils: failed to load fossil", "resource_id", evt.SourceID, "error", svcErr)
					continue
				}
				watchEvent.Fossil = fossilToProto(fossil)
			}

			if err := stream.Send(watchEvent); err != nil {
				slog.DebugContext(ctx, "WatchFossils: send error", "subscriber_id", sub.ID, "error", err)
				return err
			}
		}
	}
}
