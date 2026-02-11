package trex

import (
	"sync"

	"github.com/openshift-online/rh-trex/pkg/api/presenters"
	"github.com/openshift-online/rh-trex/pkg/config"
	"github.com/openshift-online/rh-trex/pkg/errors"
	"github.com/openshift-online/rh-trex/pkg/handlers"
)

type Config struct {
	ServiceName    string
	BasePath       string
	ErrorHref      string
	MetadataID     string
	ProjectRootDir string
	CORSOrigins    []string
}

var (
	globalConfig Config
	once         sync.Once
	initialized  bool
)

func Init(cfg Config) {
	once.Do(func() {
		if cfg.ServiceName == "" {
			cfg.ServiceName = "rh-trex"
		}
		if cfg.BasePath == "" {
			cfg.BasePath = "/api/rh-trex/v1"
		}
		if cfg.ErrorHref == "" {
			cfg.ErrorHref = cfg.BasePath + "/errors/"
		}
		if cfg.MetadataID == "" {
			cfg.MetadataID = cfg.ServiceName
		}

		globalConfig = cfg
		initialized = true

		errors.SetErrorCodePrefix(cfg.ServiceName)
		errors.SetErrorHref(cfg.ErrorHref)
		presenters.SetBasePath(cfg.BasePath)
		handlers.SetMetadataID(cfg.MetadataID)

		if cfg.ProjectRootDir != "" {
			config.SetProjectRootDir(cfg.ProjectRootDir)
		}
	})
}

func GetConfig() Config {
	if !initialized {
		panic("trex.Init() must be called before GetConfig() - typically in environments package init()")
	}
	return globalConfig
}

func IsInitialized() bool {
	return initialized
}

var defaultCORSOrigins = []string{
	"https://qa.foo.redhat.com:1337",
	"https://prod.foo.redhat.com:1337",
	"https://ci.foo.redhat.com:1337",
	"https://cloud.redhat.com",
	"https://console.redhat.com",
	"https://qaprodauth.cloud.redhat.com",
	"https://qa.cloud.redhat.com",
	"https://ci.cloud.redhat.com",
	"https://qaprodauth.console.redhat.com",
	"https://qa.console.redhat.com",
	"https://ci.console.redhat.com",
	"https://console.stage.redhat.com",
	"https://api.stage.openshift.com",
	"https://api.openshift.com",
	"https://access.qa.redhat.com",
	"https://access.stage.redhat.com",
	"https://access.redhat.com",
}

func GetCORSOrigins() []string {
	if !initialized {
		panic("trex.Init() must be called before GetCORSOrigins() - typically in environments package init()")
	}
	if len(globalConfig.CORSOrigins) > 0 {
		return globalConfig.CORSOrigins
	}
	return defaultCORSOrigins
}
