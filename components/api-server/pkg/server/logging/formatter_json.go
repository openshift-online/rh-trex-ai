package logging

import (
	"context"
	"encoding/json"
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
		// The body is never logged: it may hold credentials, and reading it
		// here would consume it before the handler. Log its length instead.
		// ContentLength is -1 when unknown, so only a declared length is kept.
		if r.ContentLength > 0 {
			jsonlog.ContentLength = r.ContentLength
		}
	}

	log, err := json.Marshal(jsonlog)
	if err != nil {
		return "", err
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

type jsonRequestLog struct {
	Method        string      `json:"request_method"`
	RequestURI    string      `json:"request_url"`
	Header        http.Header `json:"request_header,omitempty"`
	ContentLength int64       `json:"request_content_length,omitempty"`
	RemoteAddr    string      `json:"request_remote_ip,omitempty"`
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
