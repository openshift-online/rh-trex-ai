package builds

import (
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func buildToProto(d *Build) *pb.Build {
	return &pb.Build{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "Build",
			Href:      "/api/rh-trex-ai/v1/builds/" + d.ID,
		},
		ProjectId:   d.ProjectId,
		Status:      d.Status,
		BuildLog:    d.BuildLog,
		TriggeredBy: d.TriggeredBy,
		CompletedAt: func() *timestamppb.Timestamp {
			if d.CompletedAt != nil {
				return timestamppb.New(*d.CompletedAt)
			}
			return nil
		}(),
	}
}
