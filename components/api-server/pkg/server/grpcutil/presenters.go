package grpcutil

import (
	"net/http"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ServiceErrorToGRPC(svcErr *errors.ServiceError) error {
	code := HTTPStatusToGRPCCode(svcErr.HttpCode)
	return status.Error(code, svcErr.Reason)
}

func HTTPStatusToGRPCCode(httpCode int) codes.Code {
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

func APIEventTypeToProto(et api.EventType) pb.EventType {
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
