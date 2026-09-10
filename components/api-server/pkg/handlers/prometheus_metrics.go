package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusMetricsHandler struct {
}

// NewPrometheusMetricsHandler adds custom metrics and proxy to prometheus handler
func NewPrometheusMetricsHandler() *PrometheusMetricsHandler {
	return &PrometheusMetricsHandler{}
}

func (h *PrometheusMetricsHandler) Handler() http.Handler {
	handler := promhttp.Handler()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
}
