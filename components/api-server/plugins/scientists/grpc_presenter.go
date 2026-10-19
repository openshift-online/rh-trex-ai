package scientists

import (
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func scientistToProto(d *Scientist) *pb.Scientist {
	return &pb.Scientist{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "Scientist",
			Href:      "/api/rh-trex-ai/v1/scientists/" + d.ID,
		},
		Name:  d.Name,
		Field: d.Field,
	}
}

// scientistPatchFromProto turns a gRPC update request into the model patch the service applies.
func scientistPatchFromProto(req *pb.UpdateScientistRequest) *ScientistPatchRequest {
	patch := &ScientistPatchRequest{}
	patch.Name = req.Name
	patch.Field = req.Field
	return patch
}
