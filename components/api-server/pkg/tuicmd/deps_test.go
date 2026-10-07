package tuicmd

import (
	"os/exec"
	"strings"
	"testing"
)

// TestTUICommandDoesNotLinkTheServerFramework keeps this package importable by a
// CLI: it may depend on cobra, bubbletea and pkg/tui, not on the database, gRPC,
// metrics or server packages that pkg/cmd's serve and migrate commands need.
func TestTUICommandDoesNotLinkTheServerFramework(t *testing.T) {
	output, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	const module = "github.com/openshift-online/rh-trex-ai/components/api-server/"
	forbiddenThirdParty := []string{
		"gorm.io/", "github.com/go-gormigrate/", "google.golang.org/grpc", "google.golang.org/genproto",
		"github.com/golang/glog", "github.com/prometheus/", "go.opentelemetry.io/", "github.com/getsentry/",
	}
	allowedInternal := map[string]bool{module + "pkg/tui": true, module + "pkg/tuicmd": true}
	var violations []string
	for _, path := range strings.Fields(string(output)) {
		if strings.HasPrefix(path, module) && !allowedInternal[path] {
			violations = append(violations, path)
		}
		for _, prefix := range forbiddenThirdParty {
			if strings.HasPrefix(path, prefix) {
				violations = append(violations, path)
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("pkg/tuicmd must stay free of the server framework, but depends on:\n%s", strings.Join(violations, "\n"))
	}
}
