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
