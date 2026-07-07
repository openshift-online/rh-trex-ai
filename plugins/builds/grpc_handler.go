package builds

import (
	"context"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	pkgserver "github.com/openshift-online/rh-trex-ai/pkg/server"
	"github.com/openshift-online/rh-trex-ai/pkg/server/grpcutil"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

type buildGRPCHandler struct {
	pb.UnimplementedBuildServiceServer
	service    BuildService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewBuildGRPCHandler(svc BuildService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.BuildServiceServer {
	return &buildGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *buildGRPCHandler) GetBuild(ctx context.Context, req *pb.GetBuildRequest) (*pb.Build, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	build, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return buildToProto(build), nil
}

func (h *buildGRPCHandler) CreateBuild(ctx context.Context, req *pb.CreateBuildRequest) (*pb.Build, error) {
	if err := grpcutil.ValidateStringField("project_id", req.ProjectId, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("status", req.Status, true); err != nil {
		return nil, err
	}

	build := &Build{
		ProjectId:   req.ProjectId,
		Status:      req.Status,
		BuildLog:    req.BuildLog,
		TriggeredBy: req.TriggeredBy,
		CompletedAt: protoTimestampToTimePtr(req.CompletedAt),
	}
	result, svcErr := h.service.Create(ctx, build)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return buildToProto(result), nil
}

func (h *buildGRPCHandler) UpdateBuild(ctx context.Context, req *pb.UpdateBuildRequest) (*pb.Build, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.ProjectId != nil {
		if err := grpcutil.ValidateStringField("project_id", *req.ProjectId, false); err != nil {
			return nil, err
		}
	}
	if req.Status != nil {
		if err := grpcutil.ValidateStringField("status", *req.Status, false); err != nil {
			return nil, err
		}
	}
	if req.BuildLog != nil {
		if err := grpcutil.ValidateStringField("build_log", *req.BuildLog, false); err != nil {
			return nil, err
		}
	}
	if req.TriggeredBy != nil {
		if err := grpcutil.ValidateStringField("triggered_by", *req.TriggeredBy, false); err != nil {
			return nil, err
		}
	}

	build, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.ProjectId != nil {
		build.ProjectId = *req.ProjectId
	}
	if req.Status != nil {
		build.Status = *req.Status
	}
	if req.BuildLog != nil {
		build.BuildLog = req.BuildLog
	}
	if req.TriggeredBy != nil {
		build.TriggeredBy = req.TriggeredBy
	}
	if req.CompletedAt != nil {
		build.CompletedAt = protoTimestampToTimePtr(req.CompletedAt)
	}
	result, svcErr := h.service.Replace(ctx, build)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return buildToProto(result), nil
}

func (h *buildGRPCHandler) DeleteBuild(ctx context.Context, req *pb.DeleteBuildRequest) (*pb.DeleteBuildResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteBuildResponse{}, nil
}

func (h *buildGRPCHandler) ListBuilds(ctx context.Context, req *pb.ListBuildsRequest) (*pb.ListBuildsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var builds []Build
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &builds)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.Build, len(builds))
	for i, d := range builds {
		items[i] = buildToProto(&d)
	}

	return &pb.ListBuildsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *buildGRPCHandler) WatchBuilds(req *pb.WatchBuildsRequest, stream grpc.ServerStreamingServer[pb.BuildWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	glog.V(4).Infof("WatchBuilds: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchBuilds: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "Builds" {
				continue
			}

			watchEvent := &pb.BuildWatchEvent{
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				build, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchBuilds: failed to load build %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.Build = buildToProto(build)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchBuilds: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}

func protoTimestampToTimePtr(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}
