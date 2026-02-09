package environments

import (
	pkgenv "github.com/openshift-online/rh-trex/pkg/environments"
)

func init() {
	pkgenv.NewDefaultEnvironment()
}

func GetEnvironmentStrFromEnv() string {
	return pkgenv.GetEnvironmentStrFromEnv()
}

func Environment() *Env {
	return pkgenv.Environment()
}
