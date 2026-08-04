module github.com/example/my-service

go 1.24.0

toolchain go1.24.9

require (
	github.com/golang/glog v1.2.5
	github.com/openshift-online/rh-trex-ai v0.0.0-00010101000000-000000000000
	github.com/spf13/pflag v1.0.5
	gorm.io/gorm v1.20.5
)

// For local development, replace with path to TRex library source
// Update this path to point to your local TRex library directory
replace github.com/openshift-online/rh-trex-ai => ../path/to/rh-trex-ai