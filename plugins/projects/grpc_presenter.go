package projects

import (
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func projectToProto(d *Project) *pb.Project {
	return &pb.Project{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "Project",
			Href:      "/api/rh-trex-ai/v1/projects/" + d.ID,
		},
		Name:          d.Name,
		Description:   d.Description,
		RepositoryUrl: d.RepositoryUrl,
		Status:        d.Status,
	}
}
