package mocks

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/client/apiclient"
)

var _ apiclient.Authorization = &AuthzValidatorMock{}

type AuthzValidatorMock struct {
	Action       string
	ResourceType string
}

func NewAuthzValidatorMockClient() (*AuthzValidatorMock, *apiclient.Client) {
	authz := &AuthzValidatorMock{
		Action:       "",
		ResourceType: "",
	}
	client := &apiclient.Client{}
	client.Authorization = authz
	return authz, client
}

func (m *AuthzValidatorMock) SelfAccessReview(ctx context.Context, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	m.Action = action
	m.ResourceType = resourceType
	return true, nil
}

func (m *AuthzValidatorMock) AccessReview(ctx context.Context, username, action, resourceType, organizationID, subscriptionID, clusterID string) (allowed bool, err error) {
	m.Action = action
	m.ResourceType = resourceType
	return true, nil
}

func (m *AuthzValidatorMock) Reset() {
	m.Action = ""
	m.ResourceType = ""
}
