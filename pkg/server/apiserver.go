package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	_ "github.com/auth0/go-jwt-middleware"
	_ "github.com/golang-jwt/jwt/v4"
	"github.com/golang/glog"
	gorillahandlers "github.com/gorilla/handlers"
	sdk "github.com/openshift-online/ocm-sdk-go"
	"github.com/openshift-online/ocm-sdk-go/authentication"

	"github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/trex"
)

type defaultAPIServer struct {
	httpServer *http.Server
	env        *environments.Env
}

var _ Server = &defaultAPIServer{}

var preAuthMiddlewares []func(http.Handler) http.Handler

func RegisterPreAuthMiddleware(mw func(http.Handler) http.Handler) {
	preAuthMiddlewares = append(preAuthMiddlewares, mw)
}

func NewDefaultAPIServer(env *environments.Env, specData []byte) Server {
	s := &defaultAPIServer{env: env}

	mainRouter := BuildDefaultRoutes(env, specData)

	var mainHandler http.Handler = mainRouter

	for _, mw := range preAuthMiddlewares {
		mainHandler = mw(mainHandler)
	}

	if env.Config.Server.EnableJWT {
		authnLogger, err := sdk.NewGlogLoggerBuilder().
			InfoV(glog.Level(1)).
			DebugV(glog.Level(5)).
			Build()
		Check(err, "Unable to create authentication logger")

		mainHandler, err = authentication.NewHandler().
			Logger(authnLogger).
			KeysFile(env.Config.Server.JwkCertFile).
			KeysURL(env.Config.Server.JwkCertURL).
			ACLFile(env.Config.Server.ACLFile).
			Public("^" + strings.TrimSuffix(trex.GetConfig().BasePath, "/v1") + "/?$").
			Public("^" + trex.GetConfig().BasePath + "/?$").
			Public("^" + trex.GetConfig().BasePath + "/openapi/?$").
			Public("^" + trex.GetConfig().BasePath + "/openapi.html/?$").
			Public("^" + trex.GetConfig().BasePath + "/errors(/.*)?$").
			Next(mainHandler).
			Build()
		Check(err, "Unable to create authentication handler")
	}

	corsOrigins := trex.GetCORSOrigins()
	if len(env.Config.Server.CORSAllowedOrigins) > 0 {
		corsOrigins = env.Config.Server.CORSAllowedOrigins
	}

	corsHeaders := []string{
		"Authorization",
		"Content-Type",
		"X-Forwarded-Access-Token",
	}
	if len(env.Config.Server.CORSAllowedHeaders) > 0 {
		corsHeaders = append(corsHeaders, env.Config.Server.CORSAllowedHeaders...)
	}

	mainHandler = gorillahandlers.CORS(
		gorillahandlers.AllowedOrigins(corsOrigins),
		gorillahandlers.AllowedMethods([]string{
			http.MethodDelete,
			http.MethodGet,
			http.MethodPatch,
			http.MethodPost,
		}),
		gorillahandlers.AllowedHeaders(corsHeaders),
		gorillahandlers.MaxAge(int((10 * time.Minute).Seconds())),
	)(mainHandler)

	mainHandler = RemoveTrailingSlash(mainHandler)

	s.httpServer = &http.Server{
		Addr:    env.Config.Server.BindAddress,
		Handler: mainHandler,
	}

	return s
}

func (s defaultAPIServer) Serve(listener net.Listener) {
	var err error
	if s.env.Config.Server.EnableHTTPS {
		if s.env.Config.Server.HTTPSCertFile == "" || s.env.Config.Server.HTTPSKeyFile == "" {
			Check(
				fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
				"Can't start https server",
			)
		}

		glog.Infof("Serving with TLS at %s", s.env.Config.Server.BindAddress)
		err = s.httpServer.ServeTLS(listener, s.env.Config.Server.HTTPSCertFile, s.env.Config.Server.HTTPSKeyFile)
	} else {
		glog.Infof("Serving without TLS at %s", s.env.Config.Server.BindAddress)
		err = s.httpServer.Serve(listener)
	}

	Check(err, "Web server terminated with errors")
	glog.Info("Web server terminated")
}

func (s defaultAPIServer) Listen() (listener net.Listener, err error) {
	return net.Listen("tcp", s.env.Config.Server.BindAddress)
}

func (s defaultAPIServer) Start() {
	listener, err := s.Listen()
	if err != nil {
		glog.Fatalf("Unable to start API server: %s", err)
	}
	s.Serve(listener)

	s.env.Database.SessionFactory.Close()
}

func (s defaultAPIServer) Stop() error {
	return s.httpServer.Shutdown(context.Background())
}
