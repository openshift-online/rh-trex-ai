package fossils

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
		return nil, serviceErrorToGRPC(svcErr)
	}
	return fossilToProto(fossil), nil
}

func (h *fossilGRPCHandler) CreateFossil(ctx context.Context, req *pb.CreateFossilRequest) (*pb.Fossil, error) {
	if err := grpcutil.ValidateStringField("discovery_location", req.DiscoveryLocation, true); err != nil {
		return nil, err
	}

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
		return nil, serviceErrorToGRPC(svcErr)
	}
	return fossilToProto(result), nil
}

func (h *fossilGRPCHandler) UpdateFossil(ctx context.Context, req *pb.UpdateFossilRequest) (*pb.Fossil, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.DiscoveryLocation != nil {
		if err := grpcutil.ValidateStringField("discovery_location", *req.DiscoveryLocation, false); err != nil {
			return nil, err
		}
	}
	if req.FossilType != nil {
		if err := grpcutil.ValidateStringField("fossil_type", *req.FossilType, false); err != nil {
			return nil, err
		}
	}
	if req.ExcavatorName != nil {
		if err := grpcutil.ValidateStringField("excavator_name", *req.ExcavatorName, false); err != nil {
			return nil, err
		}
	}

	fossil, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	if req.DiscoveryLocation != nil {
		fossil.DiscoveryLocation = *req.DiscoveryLocation
	}
	if req.EstimatedAge != nil {
		fossil.EstimatedAge = func() *int { v := int(*req.EstimatedAge); return &v }()
	}
	if req.FossilType != nil {
		fossil.FossilType = req.FossilType
	}
	if req.ExcavatorName != nil {
		fossil.ExcavatorName = req.ExcavatorName
	}
	result, svcErr := h.service.Replace(ctx, fossil)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	return fossilToProto(result), nil
}

func (h *fossilGRPCHandler) DeleteFossil(ctx context.Context, req *pb.DeleteFossilRequest) (*pb.DeleteFossilResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	return &pb.DeleteFossilResponse{}, nil
}

func (h *fossilGRPCHandler) ListFossils(ctx context.Context, req *pb.ListFossilsRequest) (*pb.ListFossilsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	allFossils, svcErr := h.service.All(ctx)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}

	total := int32(len(allFossils))
	start := (page - 1) * size
	if start >= total {
		return &pb.ListFossilsResponse{
			Items:    []*pb.Fossil{},
			Metadata: &pb.ListMeta{Page: page, Size: size, Total: total},
		}, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	pageItems := allFossils[start:end]

	items := make([]*pb.Fossil, len(pageItems))
	for i, d := range pageItems {
		items[i] = fossilToProto(d)
	}

	return &pb.ListFossilsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: total},
	}, nil
}

func (h *fossilGRPCHandler) WatchFossils(req *pb.WatchFossilsRequest, stream grpc.ServerStreamingServer[pb.FossilWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub := broker.Subscribe(ctx)
	glog.V(4).Infof("WatchFossils: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchFossils: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "Fossils" {
				continue
			}

			watchEvent := &pb.FossilWatchEvent{
				Type:       apiEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				fossil, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchFossils: failed to load fossil %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.Fossil = fossilToProto(fossil)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchFossils: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
