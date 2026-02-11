package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

// SendNotFound sends a 404 response with some details about the non existing resource.
func SendNotFound(w http.ResponseWriter, r *http.Request) {
	// Set the content type:
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	id := "404"
	reason := fmt.Sprintf(
		"The requested resource '%s' doesn't exist",
		r.URL.Path,
	)
	body := Error{
		Type:   ErrorType,
		ID:     id,
		HREF:   errors.ErrorHrefBase() + id,
		Code:   errors.ErrorCodePrefix() + "-" + id,
		Reason: reason,
	}
	data, err := json.Marshal(body)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	w.WriteHeader(http.StatusNotFound)
	_, err = w.Write(data)
	if err != nil {
		err = fmt.Errorf("can't send response body for request '%s'", r.URL.Path)
		glog.Error(err)
		sentry.CaptureException(err)
		return
	}
}

func SendUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	apiError := errors.Unauthorized("%s", message)
	data, err := json.Marshal(apiError)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	w.WriteHeader(http.StatusUnauthorized)
	_, err = w.Write(data)
	if err != nil {
		err = fmt.Errorf("can't send response body for request '%s'", r.URL.Path)
		glog.Error(err)
		sentry.CaptureException(err)
		return
	}
}

// SendPanic sends a panic error response to the client, but it doesn't end the process.
func SendPanic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, err := w.Write(getPanicBody())
	if err != nil {
		err = fmt.Errorf(
			"can't send panic response for request '%s': %s",
			r.URL.Path,
			err.Error(),
		)
		glog.Error(err)
		sentry.CaptureException(err)
	}
}

var (
	panicOnce sync.Once
	panicBody []byte
)

func getPanicBody() []byte {
	panicOnce.Do(func() {
		panicID := "1000"
		panicError := Error{
			Type: ErrorType,
			ID:   panicID,
			HREF: errors.ErrorHrefBase() + panicID,
			Code: errors.ErrorCodePrefix() + "-" + panicID,
			Reason: "An unexpected error happened, please check the log of the service " +
				"for details",
		}
		var err error
		panicBody, err = json.Marshal(panicError)
		if err != nil {
			err = fmt.Errorf(
				"can't create the panic error body: %s",
				err.Error(),
			)
			glog.Error(err)
			sentry.CaptureException(err)
			panic(err)
		}
	})
	return panicBody
}
