package dinosaurs

import (
	"net/http"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func serviceErrorToGRPC(svcErr *errors.ServiceError) error {
	code := httpStatusToGRPCCode(svcErr.HttpCode)
	return status.Error(code, svcErr.Reason)
}

func httpStatusToGRPCCode(httpCode int) codes.Code {
	switch httpCode {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusUnprocessableEntity:
		return codes.InvalidArgument
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	default:
		if httpCode >= 400 && httpCode < 500 {
			return codes.InvalidArgument
		}
		return codes.Internal
	}
}

func apiEventTypeToProto(et api.EventType) pb.EventType {
	switch et {
	case api.CreateEventType:
		return pb.EventType_EVENT_TYPE_CREATED
	case api.UpdateEventType:
		return pb.EventType_EVENT_TYPE_UPDATED
	case api.DeleteEventType:
		return pb.EventType_EVENT_TYPE_DELETED
	default:
		return pb.EventType_EVENT_TYPE_UNSPECIFIED
	}
}
