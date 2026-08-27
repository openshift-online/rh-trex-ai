package scientists

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

type scientistGRPCHandler struct {
	pb.UnimplementedScientistServiceServer
	service    ScientistService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewScientistGRPCHandler(svc ScientistService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.ScientistServiceServer {
	return &scientistGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *scientistGRPCHandler) GetScientist(ctx context.Context, req *pb.GetScientistRequest) (*pb.Scientist, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	scientist, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return scientistToProto(scientist), nil
}

func (h *scientistGRPCHandler) CreateScientist(ctx context.Context, req *pb.CreateScientistRequest) (*pb.Scientist, error) {
	if err := grpcutil.ValidateStringField("name", req.Name, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("field", req.Field, true); err != nil {
		return nil, err
	}

	scientist := &Scientist{
		Name:  req.Name,
		Field: req.Field,
	}
	result, svcErr := h.service.Create(ctx, scientist)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return scientistToProto(result), nil
}

func (h *scientistGRPCHandler) UpdateScientist(ctx context.Context, req *pb.UpdateScientistRequest) (*pb.Scientist, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.Name != nil {
		if err := grpcutil.ValidateStringField("name", *req.Name, false); err != nil {
			return nil, err
		}
	}
	if req.Field != nil {
		if err := grpcutil.ValidateStringField("field", *req.Field, false); err != nil {
			return nil, err
		}
	}

	scientist, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.Name != nil {
		scientist.Name = *req.Name
	}
	if req.Field != nil {
		scientist.Field = *req.Field
	}
	result, svcErr := h.service.Replace(ctx, scientist)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return scientistToProto(result), nil
}

func (h *scientistGRPCHandler) DeleteScientist(ctx context.Context, req *pb.DeleteScientistRequest) (*pb.DeleteScientistResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteScientistResponse{}, nil
}

func (h *scientistGRPCHandler) ListScientists(ctx context.Context, req *pb.ListScientistsRequest) (*pb.ListScientistsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var scientists []Scientist
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &scientists)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.Scientist, len(scientists))
	for i, d := range scientists {
		items[i] = scientistToProto(&d)
	}

	return &pb.ListScientistsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *scientistGRPCHandler) WatchScientists(req *pb.WatchScientistsRequest, stream grpc.ServerStreamingServer[pb.ScientistWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	glog.V(4).Infof("WatchScientists: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchScientists: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "Scientists" {
				continue
			}

			watchEvent := &pb.ScientistWatchEvent{
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				scientist, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchScientists: failed to load scientist %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.Scientist = scientistToProto(scientist)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchScientists: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
