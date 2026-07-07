package relationships

import (
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func relationshipToProto(d *Relationship) *pb.Relationship {
	return &pb.Relationship{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "Relationship",
			Href:      "/api/rh-trex-ai/v1/relationships/" + d.ID,
		},
		ProjectId:        d.ProjectId,
		SourceEntityId:   d.SourceEntityId,
		TargetEntityId:   d.TargetEntityId,
		RelationshipType: d.RelationshipType,
		ForeignKey:       d.ForeignKey,
	}
}
