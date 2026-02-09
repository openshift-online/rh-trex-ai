package mocks

import (
	"net/http/httptest"
	"time"

	pkgmocks "github.com/openshift-online/rh-trex/pkg/testutil/mocks"
)

func NewMockServerTimeout(endpoint string, waitTime time.Duration) (*httptest.Server, func()) {
	return pkgmocks.NewMockServerTimeout(endpoint, waitTime)
}
