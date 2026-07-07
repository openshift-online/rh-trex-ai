package projects

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

type projectGRPCHandler struct {
	pb.UnimplementedProjectServiceServer
	service    ProjectService
	generic    services.GenericService
	brokerFunc func() *pkgserver.EventBroker
}

func NewProjectGRPCHandler(svc ProjectService, generic services.GenericService, brokerFunc func() *pkgserver.EventBroker) pb.ProjectServiceServer {
	return &projectGRPCHandler{service: svc, generic: generic, brokerFunc: brokerFunc}
}

func (h *projectGRPCHandler) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.Project, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	project, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return projectToProto(project), nil
}

func (h *projectGRPCHandler) CreateProject(ctx context.Context, req *pb.CreateProjectRequest) (*pb.Project, error) {
	if err := grpcutil.ValidateStringField("name", req.Name, true); err != nil {
		return nil, err
	}
	if err := grpcutil.ValidateStringField("status", req.Status, true); err != nil {
		return nil, err
	}

	project := &Project{
		Name:          req.Name,
		Description:   req.Description,
		RepositoryUrl: req.RepositoryUrl,
		Status:        req.Status,
	}
	result, svcErr := h.service.Create(ctx, project)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return projectToProto(result), nil
}

func (h *projectGRPCHandler) UpdateProject(ctx context.Context, req *pb.UpdateProjectRequest) (*pb.Project, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}
	if req.Name != nil {
		if err := grpcutil.ValidateStringField("name", *req.Name, false); err != nil {
			return nil, err
		}
	}
	if req.Description != nil {
		if err := grpcutil.ValidateStringField("description", *req.Description, false); err != nil {
			return nil, err
		}
	}
	if req.RepositoryUrl != nil {
		if err := grpcutil.ValidateStringField("repository_url", *req.RepositoryUrl, false); err != nil {
			return nil, err
		}
	}
	if req.Status != nil {
		if err := grpcutil.ValidateStringField("status", *req.Status, false); err != nil {
			return nil, err
		}
	}

	project, svcErr := h.service.Get(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = req.Description
	}
	if req.RepositoryUrl != nil {
		project.RepositoryUrl = req.RepositoryUrl
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	result, svcErr := h.service.Replace(ctx, project)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return projectToProto(result), nil
}

func (h *projectGRPCHandler) DeleteProject(ctx context.Context, req *pb.DeleteProjectRequest) (*pb.DeleteProjectResponse, error) {
	if err := grpcutil.ValidateRequiredID(req.Id); err != nil {
		return nil, err
	}

	svcErr := h.service.Delete(ctx, req.Id)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}
	return &pb.DeleteProjectResponse{}, nil
}

func (h *projectGRPCHandler) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	page, size := grpcutil.NormalizePagination(req.Page, req.Size)

	listArgs := &services.ListArguments{
		Page: int(page),
		Size: int64(size),
	}

	var projects []Project
	paging, svcErr := h.generic.List(ctx, "id", listArgs, &projects)
	if svcErr != nil {
		return nil, grpcutil.ServiceErrorToGRPC(svcErr)
	}

	items := make([]*pb.Project, len(projects))
	for i, d := range projects {
		items[i] = projectToProto(&d)
	}

	return &pb.ListProjectsResponse{
		Items:    items,
		Metadata: &pb.ListMeta{Page: page, Size: size, Total: int32(paging.Total)},
	}, nil
}

func (h *projectGRPCHandler) WatchProjects(req *pb.WatchProjectsRequest, stream grpc.ServerStreamingServer[pb.ProjectWatchEvent]) error {
	broker := h.brokerFunc()
	if broker == nil {
		return status.Error(codes.Unavailable, "event broker not available")
	}

	ctx := stream.Context()
	sub, err := broker.Subscribe(ctx)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to subscribe: %v", err)
	}
	glog.V(4).Infof("WatchProjects: subscriber %s connected", sub.ID)

	for {
		select {
		case <-ctx.Done():
			glog.V(4).Infof("WatchProjects: subscriber %s disconnected", sub.ID)
			return nil
		case evt, ok := <-sub.Events:
			if !ok {
				return nil
			}

			if evt.Source != "Projects" {
				continue
			}

			watchEvent := &pb.ProjectWatchEvent{
				Type:       grpcutil.APIEventTypeToProto(evt.EventType),
				ResourceId: evt.SourceID,
			}

			if evt.EventType != api.DeleteEventType {
				project, svcErr := h.service.Get(ctx, evt.SourceID)
				if svcErr != nil {
					glog.Warningf("WatchProjects: failed to load project %s: %v", evt.SourceID, svcErr)
					continue
				}
				watchEvent.Project = projectToProto(project)
			}

			if err := stream.Send(watchEvent); err != nil {
				glog.V(4).Infof("WatchProjects: send error for subscriber %s: %v", sub.ID, err)
				return err
			}
		}
	}
}
