package environments

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"github.com/openshift-online/rh-trex-ai/pkg/client/ocm"
	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/registry"
)

var (
	globalEnv  *Env
	globalOnce sync.Once
	envImpls   map[string]EnvironmentImpl
)

func NewEnvironment(impls map[string]EnvironmentImpl) *Env {
	globalOnce.Do(func() {
		globalEnv = &Env{}
		globalEnv.Config = config.NewApplicationConfig()
		globalEnv.Name = GetEnvironmentStrFromEnv()
		envImpls = impls
	})
	return globalEnv
}

func DefaultEnvironmentImpls(env *Env) map[string]EnvironmentImpl {
	return map[string]EnvironmentImpl{
		DevelopmentEnv:        &DevEnvImpl{Env: env},
		UnitTestingEnv:        &UnitTestingEnvImpl{Env: env},
		IntegrationTestingEnv: &IntegrationTestingEnvImpl{Env: env},
		ProductionEnv:         &ProductionEnvImpl{Env: env},
	}
}

func NewDefaultEnvironment() *Env {
	globalOnce.Do(func() {
		globalEnv = &Env{}
		globalEnv.Config = config.NewApplicationConfig()
		globalEnv.Name = GetEnvironmentStrFromEnv()
		envImpls = DefaultEnvironmentImpls(globalEnv)
	})
	return globalEnv
}

func Environment() *Env {
	return globalEnv
}

func GetEnvironmentStrFromEnv() string {
	envStr, specified := os.LookupEnv(EnvironmentStringKey)
	if !specified || envStr == "" {
		envStr = EnvironmentDefault
	}
	return envStr
}

func (e *Env) AddFlags(flags *pflag.FlagSet) error {
	e.Config.AddFlags(flags)
	return SetConfigDefaults(flags, envImpls[e.Name].Flags())
}

func (e *Env) Initialize() error {
	glog.Infof("Initializing %s environment", e.Name)

	envImpl, found := envImpls[e.Name]
	if !found {
		glog.Fatalf("Unknown runtime environment: %s", e.Name)
	}

	if err := envImpl.OverrideConfig(e.Config); err != nil {
		glog.Fatalf("Failed to configure ApplicationConfig: %s", err)
	}

	messages := globalEnv.Config.ReadFiles()
	if len(messages) != 0 {
		err := fmt.Errorf("unable to read configuration files:\n%s", strings.Join(messages, "\n"))
		sentry.CaptureException(err)
		glog.Fatalf("unable to read configuration files:\n%s", strings.Join(messages, "\n"))
	}

	if err := envImpl.OverrideDatabase(&e.Database); err != nil {
		glog.Fatalf("Failed to configure Database: %s", err)
	}

	err := e.LoadClients()
	if err != nil {
		return err
	}
	if err := envImpl.OverrideClients(&e.Clients); err != nil {
		glog.Fatalf("Failed to configure Clients: %s", err)
	}

	e.LoadServices()
	if err := envImpl.OverrideServices(&e.Services); err != nil {
		glog.Fatalf("Failed to configure Services: %s", err)
	}

	err = e.InitializeSentry()
	if err != nil {
		return err
	}

	seedErr := e.Seed()
	if seedErr != nil {
		return seedErr
	}

	if err := envImpl.OverrideHandlers(&e.Handlers); err != nil {
		glog.Fatalf("Failed to configure Handlers: %s", err)
	}

	return nil
}

func (e *Env) Seed() *errors.ServiceError {
	return nil
}

func (e *Env) LoadServices() {
	e.Services.InitRegistry()

	registry.LoadDiscoveredServices(&e.Services, e)
}

func (e *Env) LoadClients() error {
	var err error

	ocmConfig := ocm.Config{
		BaseURL:      e.Config.OCM.BaseURL,
		ClientID:     e.Config.OCM.ClientID,
		ClientSecret: e.Config.OCM.ClientSecret,
		SelfToken:    e.Config.OCM.SelfToken,
		TokenURL:     e.Config.OCM.TokenURL,
		Debug:        e.Config.OCM.Debug,
	}

	if e.Config.OCM.EnableMock {
		glog.Infof("Using Mock OCM Authz Client")
		e.Clients.OCM, err = ocm.NewClientMock(ocmConfig)
	} else {
		e.Clients.OCM, err = ocm.NewClient(ocmConfig)
	}
	if err != nil {
		glog.Errorf("Unable to create OCM Authz client: %s", err.Error())
		return err
	}

	return nil
}

func (e *Env) InitializeSentry() error {
	options := sentry.ClientOptions{}

	if e.Config.Sentry.Enabled {
		key := e.Config.Sentry.Key
		url := e.Config.Sentry.URL
		project := e.Config.Sentry.Project
		glog.Infof("Sentry error reporting enabled to %s on project %s", url, project)
		options.Dsn = fmt.Sprintf("https://%s@%s/%s", key, url, project)
	} else {
		glog.Infof("Disabling Sentry error reporting")
		options.Dsn = ""
	}

	transport := sentry.NewHTTPTransport()
	transport.Timeout = e.Config.Sentry.Timeout
	transport.BufferSize = 10
	options.Transport = transport
	options.Debug = e.Config.Sentry.Debug
	options.AttachStacktrace = true
	options.Environment = e.Name

	hostname, err := os.Hostname()
	if err != nil && hostname != "" {
		options.ServerName = hostname
	}

	err = sentry.Init(options)
	if err != nil {
		glog.Errorf("Unable to initialize sentry integration: %s", err.Error())
		return err
	}
	return nil
}

func (e *Env) Teardown() {
	if e.Database.SessionFactory != nil {
		if err := e.Database.SessionFactory.Close(); err != nil {
			glog.Errorf("Error closing database session factory: %s", err.Error())
		}
	}
	e.Clients.OCM.Close()
}

func SetConfigDefaults(flags *pflag.FlagSet, defaults map[string]string) error {
	for name, value := range defaults {
		if err := flags.Set(name, value); err != nil {
			glog.Errorf("Error setting flag %s: %v", name, err)
			return err
		}
	}
	return nil
}
