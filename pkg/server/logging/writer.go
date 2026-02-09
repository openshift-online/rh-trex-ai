package logging

import (
	"net/http"

	"github.com/openshift-online/rh-trex/pkg/logger"
)

func NewLoggingWriter(w http.ResponseWriter, r *http.Request, f LogFormatter) *LoggingWriter {
	return &LoggingWriter{ResponseWriter: w, request: r, formatter: f}
}

type LoggingWriter struct {
	http.ResponseWriter
	request        *http.Request
	formatter      LogFormatter
	responseStatus int
	responseBody   []byte
}

func (writer *LoggingWriter) Write(body []byte) (int, error) {
	writer.responseBody = body
	return writer.ResponseWriter.Write(body)
}

func (writer *LoggingWriter) WriteHeader(status int) {
	writer.responseStatus = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *LoggingWriter) log(logMsg string, err error) {
	log := logger.NewOCMLogger(writer.request.Context())
	switch err {
	case nil:
		log.V(Threshold).Infof(logMsg)
	default:
		log.Extra("error", err.Error()).Error("Unable to format request/response for log.")
	}
}

func (writer *LoggingWriter) prepareRequestLog() (string, error) {
	return writer.formatter.FormatRequestLog(writer.request)
}

func (writer *LoggingWriter) prepareResponseLog(elapsed string) (string, error) {
	info := &ResponseInfo{
		Header:  writer.ResponseWriter.Header(),
		Body:    writer.responseBody,
		Status:  writer.responseStatus,
		Elapsed: elapsed,
	}

	return writer.formatter.FormatResponseLog(info)
}
