package dinosaurs

import (
	"context"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	pkgserver "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/server/grpcutil"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

type dinosaurGRPCHandler struct {
	pb.UnimplementedDinosaurServiceServer
	service    DinosaurService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewDinosaurGRPCHandler(svc DinosaurService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.DinosaurServiceServer {
	return &dinosaurGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *dinosaurGRPCHandler) GetDinosaur(ctx context.Context, req *pb.GetDinosaurRequest) (*pb.Dinosaur, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	dinosaur, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
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
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
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
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.Species != nil {
		dinosaur.Species = *req.Species
	}
	result, svcErr := h.service.Replace(ctx, dinosaur)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return dinosaurToProto(result), nil
}

func (h *dinosaurGRPCHandler) DeleteDinosaur(ctx context.Context, req *pb.DeleteDinosaurRequest) (*pb.DeleteDinosaurResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteDinosaurResponse{}, nil
}

func (h *dinosaurGRPCHandler) ListDinosaurs(ctx context.Context, req *pb.ListDinosaursRequest) (*pb.ListDinosaursResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var dinosaurs []Dinosaur
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &dinosaurs)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.Dinosaur, len(dinosaurs))
	for i, d := range dinosaurs {
		items[i] = dinosaurToProto(&d)
	}

	return &pb.ListDinosaursResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *dinosaurGRPCHandler) WatchDinosaurs(req *pb.WatchDinosaursRequest, stream grpc.ServerStreamingServer[pb.DinosaurWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
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
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
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
