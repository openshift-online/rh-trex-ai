package dinosaurs_test

import (
	"flag"
	"os"
	"runtime"
	"testing"

	"github.com/golang/glog"

	"github.com/openshift-online/rh-trex-ai/test"
)

func TestMain(m *testing.M) {
	
	flag.Parse()
	glog.Infof("Starting dinosaurs integration test using go version %s", runtime.Version())
	helper := test.NewHelper(&testing.T{})
	exitCode := m.Run()
	helper.Teardown()
	os.Exit(exitCode)
}