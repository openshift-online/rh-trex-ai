package mocks

import (
	"crypto"
	"testing"

	pkgmocks "github.com/openshift-online/rh-trex/pkg/testutil/mocks"
)

func NewJWKCertServerMock(t *testing.T, pubKey crypto.PublicKey, jwkKID string, jwkAlg string) (url string, teardown func() error) {
	return pkgmocks.NewJWKCertServerMock(t, pubKey, jwkKID, jwkAlg)
}
