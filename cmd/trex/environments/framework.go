package environments

import (
	"path/filepath"
	"runtime"

	pkgenv "github.com/openshift-online/rh-trex-ai/pkg/environments"
	"github.com/openshift-online/rh-trex-ai/pkg/trex"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../../..")

	trex.Init(trex.Config{
		ServiceName:    "rh-trex-ai",
		BasePath:       "/api/rh-trex-ai/v1",
		ErrorHref:      "/api/rh-trex-ai/v1/errors/",
		MetadataID:     "rh-trex-ai",
		ProjectRootDir: projectRoot,
		CORSOrigins:    []string{"https://console.redhat.com", "https://console.stage.redhat.com"},
	})

	env := pkgenv.NewEnvironment(nil)
	env.SetEnvironmentImpls(EnvironmentImpls(env))
}

func EnvironmentImpls(env *pkgenv.Env) map[string]pkgenv.EnvironmentImpl {
	return map[string]pkgenv.EnvironmentImpl{
		pkgenv.DevelopmentEnv:        &DevEnvImpl{Env: env},
		pkgenv.UnitTestingEnv:        &UnitTestingEnvImpl{Env: env},
		pkgenv.IntegrationTestingEnv: &IntegrationTestingEnvImpl{Env: env},
		pkgenv.ProductionEnv:         &ProductionEnvImpl{Env: env},
	}
}

func GetEnvironmentStrFromEnv() string {
	return pkgenv.GetEnvironmentStrFromEnv()
}

func Environment() *Env {
	return pkgenv.Environment()
}
