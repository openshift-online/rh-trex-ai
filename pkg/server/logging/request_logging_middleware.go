package logging

import (
	"net/http"
	"strings"
	"time"

	"github.com/openshift-online/rh-trex/pkg/trex"
)

func RequestLoggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := strings.TrimSuffix(request.URL.Path, "/")
		doLog := true

		basePath := trex.GetConfig().BasePath
		suppressPath := strings.TrimSuffix(strings.TrimSuffix(basePath, "/v1"), "/")
		if path == suppressPath {
			doLog = false
		}

		loggingWriter := NewLoggingWriter(writer, request, NewJSONLogFormatter())

		if doLog {
			loggingWriter.log(loggingWriter.prepareRequestLog())
		}

		before := time.Now()
		handler.ServeHTTP(loggingWriter, request)
		elapsed := time.Since(before).String()

		if doLog {
			loggingWriter.log(loggingWriter.prepareResponseLog(elapsed))
		}
	})
}
