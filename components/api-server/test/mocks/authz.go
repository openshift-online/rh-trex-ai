package mocks

import (
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/client/apiclient"
	pkgmocks "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/testutil/mocks"
)

type AuthzValidatorMock = pkgmocks.AuthzValidatorMock

func NewAuthzValidatorMockClient() (*AuthzValidatorMock, *apiclient.Client) {
	return pkgmocks.NewAuthzValidatorMockClient()
}
