package handlers

import (
	"reflect"
	"strings"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

func ValidateNotEmpty(i interface{}, fieldName string, field string) Validate {
	return func() *errors.ServiceError {
		value := reflect.ValueOf(i).Elem().FieldByName(fieldName)
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return errors.Validation("%s is required", field)
			}
			value = value.Elem()
		}
		if len(value.String()) == 0 {
			return errors.Validation("%s is required", field)
		}
		return nil
	}
}

func ValidateEmpty(i interface{}, fieldName string, field string) Validate {
	return func() *errors.ServiceError {
		value := reflect.ValueOf(i).Elem().FieldByName(fieldName)
		if value.Kind() == reflect.Ptr {
			if value.IsNil() {
				return nil
			}
			value = value.Elem()
		}
		if len(value.String()) != 0 {
			return errors.Validation("%s must be empty", field)
		}
		return nil
	}
}

// Note that because this uses strings.EqualFold, it is case-insensitive
func ValidateInclusionIn(value *string, list []string, category *string) Validate {
	return func() *errors.ServiceError {
		for _, item := range list {
			if strings.EqualFold(*value, item) {
				return nil
			}
		}
		if category == nil {
			category = &[]string{"value"}[0]
		}
		return errors.Validation("%s is not a valid %s", *value, *category)
	}
}
