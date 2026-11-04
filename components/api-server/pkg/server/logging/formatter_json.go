package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

func NewJSONLogFormatter() *JSONLogFormatter {
	return &JSONLogFormatter{}
}

type JSONLogFormatter struct{}

var _ LogFormatter = &JSONLogFormatter{}

func (f *JSONLogFormatter) FormatRequestLog(r *http.Request) (string, error) {
	jsonlog := jsonRequestLog{
		Method:     r.Method,
		RequestURI: r.RequestURI,
		RemoteAddr: r.RemoteAddr,
	}
	if slog.Default().Enabled(r.Context(), slog.LevelDebug) {
		jsonlog.Header = redactHeaders(r.Header)
		jsonlog.Body = r.Body
	}

	log, err := json.Marshal(jsonlog)
	if err != nil {
		// A field may not be JSON-serializable: an otel-wrapped request body can
		// carry a non-serializable value (e.g. a func(int64)). Emit the record with
		// a placeholder for the offending field instead of dropping the whole log
		// line, so request/response detail needed to debug is not lost.
		jsonlog.Body = unserializablePlaceholder(err)
		log, err = json.Marshal(jsonlog)
		if err != nil {
			return "", err
		}
	}
	return string(log[:]), nil
}

func (f *JSONLogFormatter) FormatResponseLog(info *ResponseInfo) (string, error) {
	jsonlog := jsonResponseLog{Header: nil, Status: info.Status, Elapsed: info.Elapsed}
	ctx := info.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if slog.Default().Enabled(ctx, slog.LevelDebug) {
		jsonlog.Body = string(info.Body[:])
	}
	log, err := json.Marshal(jsonlog)
	if err != nil {
		return "", err
	}
	return string(log[:]), nil
}

// unserializablePlaceholder renders a stand-in for a field that failed JSON
// marshaling, preserving the root error so it still reaches the log.
func unserializablePlaceholder(err error) string {
	return fmt.Sprintf("[unserializable: %s]", err)
}

type jsonRequestLog struct {
	Method     string      `json:"request_method"`
	RequestURI string      `json:"request_url"`
	Header     http.Header `json:"request_header,omitempty"`
	Body       any         `json:"request_body,omitempty"`
	RemoteAddr string      `json:"request_remote_ip,omitempty"`
}

type jsonResponseLog struct {
	Header  http.Header `json:"response_header,omitempty"`
	Status  int         `json:"response_status,omitempty"`
	Body    string      `json:"response_body,omitempty"`
	Elapsed string      `json:"elapsed,omitempty"`
}

// sensitiveHeaders hold credentials. Their values are never logged.
var sensitiveHeaders = []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie"}

// redactHeaders returns a copy of h with credential values replaced.
func redactHeaders(h http.Header) http.Header {
	out := h.Clone()
	for _, name := range sensitiveHeaders {
		if _, ok := out[name]; ok {
			out[name] = []string{"REDACTED"}
		}
	}
	return out
}
