package server

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
)

type Server interface {
	Start()
	Stop() error
	Listen() (net.Listener, error)
	Serve(net.Listener)
}

type ServerConfig struct {
	BindAddress   string
	EnableHTTPS   bool
	HTTPSCertFile string
	HTTPSKeyFile  string
	SentryTimeout time.Duration
}

func RemoveTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}

func Check(err error, msg string, sentryTimeout time.Duration) {
	if err != nil && err != http.ErrServerClosed {
		glog.Errorf("%s: %s", msg, err)
		sentry.CaptureException(err)
		sentry.Flush(sentryTimeout)
		os.Exit(1)
	}
}
