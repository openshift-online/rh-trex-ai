package server

import (
	"testing"

	"github.com/gorilla/mux"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/authz"
)

func TestDeclaredKindsIncludesRESTResources(t *testing.T) {
	r := NewAPIRouter(mux.NewRouter(), authz.SurfaceV1, authz.AllowAll())
	r.Resource("test_declared_things")
	for _, k := range DeclaredKinds() {
		if k == "test_declared_things" {
			return
		}
	}
	t.Fatalf("DeclaredKinds() = %v, missing test_declared_things", DeclaredKinds())
}
