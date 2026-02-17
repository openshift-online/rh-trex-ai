package dinosaurs

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

type dinosaurGRPCHandler struct {
	pb.UnimplementedDinosaurServiceServer
	service    DinosaurService
	brokerFunc func() *pkgserver.EventBroker
}

func NewDinosaurGRPCHandler(svc DinosaurService, brokerFunc func() *pkgserver.EventBroker) pb.DinosaurServiceServer {
	return &dinosaurGRPCHandler{service: svc, brokerFunc: brokerFunc}
}

func (h *dinosaurGRPCHandler) GetDinosaur(ctx context.Context, req *pb.GetDinosaurRequest) (*pb.Dinosaur, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	dinosaur, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	return dinosaurToProto(dinosaur), nil
}

func (h *dinosaurGRPCHandler) CreateDinosaur(ctx context.Context, req *pb.CreateDinosaurRequest) (*pb.Dinosaur, error) {
	if err := grpcutil.ValidateStringField("species", req.Species, true); err != nil {
		return nil, err
	}

	dinosaur := &Dinosaur{
		Species: req.Species,
	}
	result, svcErr := h.service.Create(ctx, dinosaur)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	return dinosaurToProto(result), nil
}

func (h *dinosaurGRPCHandler) UpdateDinosaur(ctx context.Context, req *pb.UpdateDinosaurRequest) (*pb.Dinosaur, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.Species != nil {
		if err := grpcutil.ValidateStringField("species", *req.Species, false); err != nil {
			return nil, err
		}
	}

	dinosaur, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	if req.Species != nil {
		dinosaur.Species = *req.Species
	}
	result, svcErr := h.service.Replace(ctx, dinosaur)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	return dinosaurToProto(result), nil
}

func (h *dinosaurGRPCHandler) DeleteDinosaur(ctx context.Context, req *pb.DeleteDinosaurRequest) (*pb.DeleteDinosaurResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}
	return &pb.DeleteDinosaurResponse{}, nil
}

func (h *dinosaurGRPCHandler) ListDinosaurs(ctx context.Context, req *pb.ListDinosaursRequest) (*pb.ListDinosaursResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	allDinosaurs, svcErr := h.service.All(ctx)
	if svcErr != nil {
		return nil, serviceErrorToGRPC(svcErr)
	}

	total := int32(len(allDinosaurs))
	start := (page - 1) * size
	if start >= total {
		return &pb.ListDinosaursResponse{
			Items:    []*pb.Dinosaur{},
			Metadata: &pb.ListMeta{Page: page, Size: size, Total: total},
		}, nil
	}
	end := start + size
	if end > total {
		end = total
	}
	pageItems := allDinosaurs[start:end]

	items := make([]*pb.Dinosaur, len(pageItems))
	for i, d := range pageItems {
		items[i] = dinosaurToProto(d)
	}

	return &pb.ListDinosaursResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: total},
	}, nil
}

func (h *dinosaurGRPCHandler) WatchDinosaurs(req *pb.WatchDinosaursRequest, stream grpc.ServerStreamingServer[pb.DinosaurWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub := broker.Subscribe(ctx)
	glog.V(4).Infof("WatchDinosaurs: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchDinosaurs: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "Dinosaurs" {
				continue
			}

			watchEvent := &pb.DinosaurWatchEvent{
				Type:       apiEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				dinosaur, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchDinosaurs: failed to load dinosaur %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.Dinosaur = dinosaurToProto(dinosaur)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchDinosaurs: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
