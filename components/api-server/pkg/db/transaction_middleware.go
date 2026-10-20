package db

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/felixge/httpsnoop"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

// TransactionMiddleware creates a new HTTP middleware that begins a database transaction
// and stores it in the request context.
//
// The transaction is rolled back when the handler responds with a 4xx or 5xx status,
// or panics. Otherwise it is committed, unless something called MarkForRollback.
func TransactionMiddleware(next http.Handler, connection SessionFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new Context with the transaction stored in it.
		ctx, err := NewContext(r.Context(), connection)
		log := logger.NewLogger(ctx)
		if err != nil {
			log.Extra("error", err.Error()).Error("Could not create transaction")
			// use default error to avoid exposing internals to users
			err := errors.GeneralError("")
			operationID := logger.GetOperationID(ctx)
			writeJSONResponse(w, err.HttpCode, err.AsOpenapiError(operationID))
			return
		}

		// Set the value of the request pointer to the value of a new copy of the request with the new context key,vale
		// stored in it
		*r = *r.WithContext(ctx)

		// Record the response status. httpsnoop keeps the optional interfaces
		// (Flusher, Hijacker, ...) of the original writer.
		status := 0
		w = httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					if status == 0 {
						status = code
					}
					next(code)
				}
			},
			Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(b []byte) (int, error) {
					if status == 0 {
						status = http.StatusOK
					}
					return next(b)
				}
			},
		})

		// Resolve the transaction when the handler returns or panics.
		defer func() {
			if p := recover(); p != nil {
				MarkForRollback(r.Context(), fmt.Errorf("panic: %v", p))
				Resolve(r.Context())
				panic(p)
			}
			if status >= http.StatusBadRequest {
				MarkForRollback(r.Context(), fmt.Errorf("HTTP status %d", status))
			}
			Resolve(r.Context())
		}()

		// Continue handling requests.
		next.ServeHTTP(w, r)
	})
}

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}
