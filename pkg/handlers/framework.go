package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
)

// handlerConfig defines the common things each REST controller must do.
// The corresponding handle() func runs the basic handlerConfig.
// This is not meant to be an HTTP framework or anything larger than simple CRUD in handlers.
//
//	MarshalInto is a pointer to the object to hold the unmarshaled JSON.
//	Validate is a list of validation function that run in order, returning fast on the first error.
//	Action is the specific logic a handler must take (e.g, find an object, save an object)
//	ErrorHandler is the way errors are returned to the client
type HandlerConfig struct {
	Body         interface{}
	Validators   []Validate
	Action       HTTPAction
	ErrorHandler ErrorHandlerFunc
}

type Validate func() *errors.ServiceError
type ErrorHandlerFunc func(ctx context.Context, w http.ResponseWriter, err *errors.ServiceError)
type HTTPAction func() (interface{}, *errors.ServiceError)

func HandleError(ctx context.Context, w http.ResponseWriter, err *errors.ServiceError) {
	log := logger.NewOCMLogger(ctx)
	operationID := logger.GetOperationID(ctx)
	// If this is a 400 error, its the user's issue, log as info rather than error
	if err.HttpCode >= 400 && err.HttpCode <= 499 {
		log.Infof(err.Error())
	} else {
		log.Error(err.Error())
	}
	writeJSONResponse(w, err.HttpCode, err.AsOpenapiError(operationID))
}

func Handle(w http.ResponseWriter, r *http.Request, cfg *HandlerConfig, httpStatus int) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = HandleError
	}

	bytes, err := io.ReadAll(r.Body)
	if err != nil {
		HandleError(r.Context(), w, errors.MalformedRequest("Unable to read request body: %s", err))
		return
	}

	err = json.Unmarshal(bytes, &cfg.Body)
	if err != nil {
		HandleError(r.Context(), w, errors.MalformedRequest("Invalid request format: %s", err))
		return
	}

	for _, v := range cfg.Validators {
		err := v()
		if err != nil {
			cfg.ErrorHandler(r.Context(), w, err)
			return
		}
	}

	result, serviceErr := cfg.Action()

	switch {
	case serviceErr != nil:
		cfg.ErrorHandler(r.Context(), w, serviceErr)
	default:
		writeJSONResponse(w, httpStatus, result)
	}

}

func HandleDelete(w http.ResponseWriter, r *http.Request, cfg *HandlerConfig, httpStatus int) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = HandleError
	}
	for _, v := range cfg.Validators {
		err := v()
		if err != nil {
			cfg.ErrorHandler(r.Context(), w, err)
			return
		}
	}

	result, serviceErr := cfg.Action()

	switch {
	case serviceErr != nil:
		cfg.ErrorHandler(r.Context(), w, serviceErr)
	default:
		writeJSONResponse(w, httpStatus, result)
	}

}

func HandleGet(w http.ResponseWriter, r *http.Request, cfg *HandlerConfig) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = HandleError
	}

	result, serviceErr := cfg.Action()
	switch {
	case serviceErr == nil:
		writeJSONResponse(w, http.StatusOK, result)
	default:
		cfg.ErrorHandler(r.Context(), w, serviceErr)
	}
}

func HandleList(w http.ResponseWriter, r *http.Request, cfg *HandlerConfig) {
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = HandleError
	}

	results, serviceError := cfg.Action()
	if serviceError != nil {
		cfg.ErrorHandler(r.Context(), w, serviceError)
		return
	}
	writeJSONResponse(w, http.StatusOK, results)
}
