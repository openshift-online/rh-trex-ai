package fieldDefinitions

import (
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fieldDefinitionToProto(d *FieldDefinition) *pb.FieldDefinition {
	return &pb.FieldDefinition{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "FieldDefinition",
			Href:      "/api/rh-trex-ai/v1/field_definitions/" + d.ID,
		},
		EntityDefinitionId: d.EntityDefinitionId,
		FieldName:          d.FieldName,
		FieldType:          d.FieldType,
		IsRequired:         d.IsRequired,
	}
}
