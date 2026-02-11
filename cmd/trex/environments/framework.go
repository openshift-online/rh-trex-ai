package environments

import (
	"runtime"
	"path/filepath"
	
	"github.com/openshift-online/rh-trex-ai/pkg/trex"
	pkgenv "github.com/openshift-online/rh-trex-ai/pkg/environments"
)

func init() {
	// Get the project root directory for TRex itself
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../../..")
	
	// Initialize TRex library with default configuration  
	trex.Init(trex.Config{
		ServiceName:    "rh-trex",
		BasePath:       "/api/rh-trex/v1", 
		ErrorHref:      "/api/rh-trex/v1/errors/",
		MetadataID:     "rh-trex",
		ProjectRootDir: projectRoot,
		CORSOrigins:    []string{"https://console.redhat.com", "https://console.stage.redhat.com"},
	})
	
	pkgenv.NewDefaultEnvironment()
}

func GetEnvironmentStrFromEnv() string {
	return pkgenv.GetEnvironmentStrFromEnv()
}

func Environment() *Env {
	return pkgenv.Environment()
}
