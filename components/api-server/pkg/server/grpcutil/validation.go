package grpcutil

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MaxStringFieldLength = 255
	MaxPageSize          = 500
	DefaultPageSize      = 20
	DefaultPage          = 1
)

func ValidateRequiredID(id string) error {
	if id == "" {
		return status.Error(codes.InvalidArgument, "id is required")
	}
	return nil
}

func ValidateStringField(name, value string, required bool) error {
	if required && value == "" {
		return status.Errorf(codes.InvalidArgument, "%s is required", name)
	}
	if len(value) > MaxStringFieldLength {
		return status.Errorf(codes.InvalidArgument, "%s exceeds maximum length of %d", name, MaxStringFieldLength)
	}
	return nil
}

func NormalizePagination(page, size int32) (int32, int32) {
	if page < 1 {
		page = DefaultPage
	}
	if size < 1 || size > MaxPageSize {
		size = DefaultPageSize
	}
	return page, size
}
