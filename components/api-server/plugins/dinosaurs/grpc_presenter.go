package dinosaurs

import (
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func dinosaurToProto(d *Dinosaur) *pb.Dinosaur {
	return &pb.Dinosaur{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "Dinosaur",
			Href:      "/api/rh-trex-ai/v1/dinosaurs/" + d.ID,
		},
		Species: d.Species,
	}
}

// dinosaurPatchFromProto turns a gRPC update request into the model patch the service applies.
func dinosaurPatchFromProto(req *pb.UpdateDinosaurRequest) *DinosaurPatchRequest {
	patch := &DinosaurPatchRequest{}
	patch.Species = req.Species
	return patch
}
