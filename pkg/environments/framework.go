package environments

import (
	"os"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"github.com/openshift-online/rh-trex-ai/pkg/client/apiclient"
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

func (e *Env) SetEnvironmentImpls(impls map[string]EnvironmentImpl) {
	envImpls = impls
}

func Environment() *Env {
	if globalEnv == nil {
		panic("environments.Environment() called before NewEnvironment() — ensure your cmd/<service>/environments package is imported in main.go for its init() side-effect")
	}
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

	apiClientConfig := apiclient.Config{
		BaseURL:      e.Config.APIClient.BaseURL,
		ClientID:     e.Config.APIClient.ClientID,
		ClientSecret: e.Config.APIClient.ClientSecret,
		SelfToken:    e.Config.APIClient.SelfToken,
		TokenURL:     e.Config.APIClient.TokenURL,
		Debug:        e.Config.APIClient.Debug,
	}

	if e.Config.APIClient.EnableMock {
		glog.Infof("Using Mock Authz Client")
		e.Clients.APIClient, err = apiclient.NewClientMock(apiClientConfig)
	} else {
		e.Clients.APIClient, err = apiclient.NewClient(apiClientConfig)
	}
	if err != nil {
		glog.Errorf("Unable to create API Authz client: %s", err.Error())
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
	e.Clients.APIClient.Close()
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
