package services

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
)

// MaxStringFieldLength is the longest value an entity string field may hold.
const MaxStringFieldLength = 255

// ValidateRequiredString rejects an empty value or one longer than MaxStringFieldLength.
func ValidateRequiredString(field, value string) *errors.ServiceError {
	if value == "" {
		return errors.Validation("%s is required", field)
	}
	return ValidateStringLength(field, value)
}

// ValidateOptionalString checks the length of a nullable string field when it is set.
func ValidateOptionalString(field string, value *string) *errors.ServiceError {
	if value == nil {
		return nil
	}
	return ValidateStringLength(field, *value)
}

// ValidateStringLength rejects a value longer than MaxStringFieldLength.
func ValidateStringLength(field, value string) *errors.ServiceError {
	if len(value) > MaxStringFieldLength {
		return errors.Validation("%s exceeds maximum length of %d", field, MaxStringFieldLength)
	}
	return nil
}
