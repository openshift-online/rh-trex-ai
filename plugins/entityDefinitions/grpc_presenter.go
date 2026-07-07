package entityDefinitions

import (
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func entityDefinitionToProto(d *EntityDefinition) *pb.EntityDefinition {
	return &pb.EntityDefinition{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "EntityDefinition",
			Href:      "/api/rh-trex-ai/v1/entity_definitions/" + d.ID,
		},
		ProjectId:      d.ProjectId,
		KindName:       d.KindName,
		PluralOverride: d.PluralOverride,
		Description:    d.Description,
	}
}
