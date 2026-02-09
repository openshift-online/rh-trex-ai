package mocks

import (
	"context"

	"github.com/openshift-online/rh-trex/pkg/client/ocm"
)

var _ ocm.Authorization = &OCMAuthzValidatorMock{}

type OCMAuthzValidatorMock struct {
	Action       string
	ResourceType string
}

func NewOCMAuthzValidatorMockClient() (*OCMAuthzValidatorMock, *ocm.Client) {
	authz := &OCMAuthzValidatorMock{
		Action:       "",
		ResourceType: "",
	}
	client := &ocm.Client{}
	client.Authorization = authz
	return authz, client
}

func (m *OCMAuthzValidatorMock) SelfAccessReview(ctx context.Context, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	m.Action = action
	m.ResourceType = resourceType
	return true, nil
}

func (m *OCMAuthzValidatorMock) AccessReview(ctx context.Context, username, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	m.Action = action
	m.ResourceType = resourceType
	return true, nil
}

func (m *OCMAuthzValidatorMock) Reset() {
	m.Action = ""
	m.ResourceType = ""
}
