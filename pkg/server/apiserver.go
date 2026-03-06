package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	gorillahandlers "github.com/gorilla/handlers"

	"github.com/openshift-online/rh-trex-ai/pkg/auth"
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
		glog.Info("Enabling JWT authentication middleware")

		jwtHandler, err := auth.NewJWTHandler().
			WithKeysFile(env.Config.Server.JwkCertFile).
			WithKeysURL(env.Config.Server.JwkCertURL).
			WithACLFile(env.Config.Server.ACLFile).
			WithPublicPath(strings.TrimSuffix(trex.GetConfig().BasePath, "/v1")).
			WithPublicPath(trex.GetConfig().BasePath).
			WithPublicPath(trex.GetConfig().BasePath + "/openapi").
			WithPublicPath(trex.GetConfig().BasePath + "/openapi.html").
			WithPublicPath(trex.GetConfig().BasePath + "/errors").
			Build()
		Check(err, "Unable to create JWT authentication handler")

		mainHandler = jwtHandler(mainHandler)
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
	
	// Apply TLS configuration using the new TLS framework (mirrors gRPC server pattern)
	if s.env.Config.TLS.EnableTLS || s.env.Config.Server.EnableHTTPS {
		// Use new TLS framework for server configuration
		tlsConfig, tlsErr := s.env.Config.TLS.BuildServerTLSConfig()
		if tlsErr != nil {
			// Fall back to legacy HTTPS configuration if new framework fails
			if s.env.Config.Server.EnableHTTPS {
				if s.env.Config.Server.HTTPSCertFile == "" || s.env.Config.Server.HTTPSKeyFile == "" {
					Check(
						fmt.Errorf("unspecified required --https-cert-file, --https-key-file"),
						"Can't start HTTPS server",
					)
				}
				
				glog.Info("Using legacy HTTPS configuration")
				glog.Infof("Serving with TLS (legacy) at %s", s.env.Config.Server.BindAddress)
				err = s.httpServer.ServeTLS(listener, s.env.Config.Server.HTTPSCertFile, s.env.Config.Server.HTTPSKeyFile)
			} else {
				glog.Infof("TLS configuration failed: %v", tlsErr)
				glog.Infof("Serving without TLS at %s", s.env.Config.Server.BindAddress)
				err = s.httpServer.Serve(listener)
			}
		} else if tlsConfig != nil {
			// Use enhanced TLS configuration
			s.httpServer.TLSConfig = tlsConfig
			glog.Infof("Using enhanced TLS configuration with minimum version %s", 
				func() string {
					switch tlsConfig.MinVersion {
					case 0x0303: // tls.VersionTLS12
						return "TLS 1.2"
					case 0x0304: // tls.VersionTLS13
						return "TLS 1.3"
					default:
						return "unknown"
					}
				}())
			glog.Infof("Serving with enhanced TLS at %s", s.env.Config.Server.BindAddress)
			
			// Serve with TLS configuration applied to the server
			err = s.httpServer.ServeTLS(listener, "", "")
		} else {
			glog.Infof("Serving without TLS at %s", s.env.Config.Server.BindAddress)
			err = s.httpServer.Serve(listener)
		}
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
