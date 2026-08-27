package fossils

import (
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func fossilToProto(d *Fossil) *pb.Fossil {
	return &pb.Fossil{
		Metadata: &pb.ObjectReference{
			Id:        d.ID,
			CreatedAt: timestamppb.New(d.CreatedAt),
			UpdatedAt: timestamppb.New(d.UpdatedAt),
			Kind:      "Fossil",
			Href:      "/api/rh-trex-ai/v1/fossils/" + d.ID,
		},
		DiscoveryLocation: d.DiscoveryLocation,
		EstimatedAge: func() *int32 {
			if d.EstimatedAge != nil {
				v := int32(*d.EstimatedAge)
				return &v
			}
			return nil
		}(),
		FossilType:    d.FossilType,
		ExcavatorName: d.ExcavatorName,
	}
}
