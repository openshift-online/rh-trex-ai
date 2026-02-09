package server

import (
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
)

func MetricsMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapper := &metricsResponseWrapper{
			wrapped: w,
		}

		before := time.Now()
		handler.ServeHTTP(wrapper, r)
		elapsed := time.Since(before)

		path := "/" + PathVarSub
		route := mux.CurrentRoute(r)
		if route != nil {
			template, err := route.GetPathTemplate()
			if err == nil {
				path = metricsPathVarRE.ReplaceAllString(template, PathVarSub)
			}
		}

		labels := prometheus.Labels{
			metricsMethodLabel: r.Method,
			metricsPathLabel:   path,
			metricsCodeLabel:   strconv.Itoa(wrapper.code),
		}

		requestCountMetric.With(labels).Inc()
		requestDurationMetric.With(labels).Observe(elapsed.Seconds())
	})
}

func ResetMetricCollectors() {
	requestCountMetric.Reset()
	requestDurationMetric.Reset()
}

var metricsPathVarRE = regexp.MustCompile(`{[^}]*}`)

var PathVarSub = "-"

const metricsSubsystem = "api_inbound"

const (
	metricsMethodLabel = "method"
	metricsPathLabel   = "path"
	metricsCodeLabel   = "code"
)

var MetricsLabels = []string{
	metricsMethodLabel,
	metricsPathLabel,
	metricsCodeLabel,
}

const (
	requestCount    = "request_count"
	requestDuration = "request_duration"
)

var MetricsNames = []string{
	requestCount,
	requestDuration,
}

var requestCountMetric = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: metricsSubsystem,
		Name:      requestCount,
		Help:      "Number of requests served.",
	},
	MetricsLabels,
)

var requestDurationMetric = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Subsystem: metricsSubsystem,
		Name:      requestDuration,
		Help:      "Request duration in seconds.",
		Buckets: []float64{
			0.1,
			1.0,
			10.0,
			30.0,
		},
	},
	MetricsLabels,
)

type metricsResponseWrapper struct {
	wrapped http.ResponseWriter
	code    int
}

func (w *metricsResponseWrapper) Header() http.Header {
	return w.wrapped.Header()
}

func (w *metricsResponseWrapper) Write(b []byte) (n int, err error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	n, err = w.wrapped.Write(b)
	return
}

func (w *metricsResponseWrapper) WriteHeader(code int) {
	w.code = code
	w.wrapped.WriteHeader(code)
}

var metricsOnce sync.Once

func RegisterMetrics() {
	metricsOnce.Do(func() {
		prometheus.MustRegister(requestCountMetric)
		prometheus.MustRegister(requestDurationMetric)
	})
}

func init() {
	RegisterMetrics()
}
