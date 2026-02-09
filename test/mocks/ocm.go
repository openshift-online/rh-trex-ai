package mocks

import (
	"github.com/openshift-online/rh-trex/pkg/client/ocm"
	pkgmocks "github.com/openshift-online/rh-trex/pkg/testutil/mocks"
)

type OCMAuthzValidatorMock = pkgmocks.OCMAuthzValidatorMock

func NewOCMAuthzValidatorMockClient() (*OCMAuthzValidatorMock, *ocm.Client) {
	return pkgmocks.NewOCMAuthzValidatorMockClient()
}
