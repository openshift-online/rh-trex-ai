package logging

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/golang/glog"
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
	if glog.V(10) {
		jsonlog.Header = r.Header
		jsonlog.Body = r.Body
	}

	log, err := json.Marshal(jsonlog)
	if err != nil {
		return "", err
	}
	return string(log[:]), nil
}

func (f *JSONLogFormatter) FormatResponseLog(info *ResponseInfo) (string, error) {
	jsonlog := jsonResponseLog{Header: nil, Status: info.Status, Elapsed: info.Elapsed}
	if glog.V(10) {
		jsonlog.Body = string(info.Body[:])
	}
	log, err := json.Marshal(jsonlog)
	if err != nil {
		return "", err
	}
	return string(log[:]), nil
}

type jsonRequestLog struct {
	Method     string        `json:"request_method"`
	RequestURI string        `json:"request_url"`
	Header     http.Header   `json:"request_header,omitempty"`
	Body       io.ReadCloser `json:"request_body,omitempty"`
	RemoteAddr string        `json:"request_remote_ip,omitempty"`
}

type jsonResponseLog struct {
	Header  http.Header `json:"response_header,omitempty"`
	Status  int         `json:"response_status,omitempty"`
	Body    string      `json:"response_body,omitempty"`
	Elapsed string      `json:"elapsed,omitempty"`
}
