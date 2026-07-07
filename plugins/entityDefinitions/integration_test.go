package entityDefinitions_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/test"
)

func TestEntityDefinitionGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	entityDefinitionModel, err := newEntityDefinition(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	entityDefinitionOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsIdGet(ctx, entityDefinitionModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*entityDefinitionOutput.Id).To(Equal(entityDefinitionModel.ID), "found object does not match test object")
	Expect(*entityDefinitionOutput.Kind).To(Equal("EntityDefinition"))
	Expect(*entityDefinitionOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/entity_definitions/%s", entityDefinitionModel.ID)))
	Expect(*entityDefinitionOutput.CreatedAt).To(BeTemporally("~", entityDefinitionModel.CreatedAt))
	Expect(*entityDefinitionOutput.UpdatedAt).To(BeTemporally("~", entityDefinitionModel.UpdatedAt))
}

func TestEntityDefinitionPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	entityDefinitionInput := openapi.EntityDefinition{
		ProjectId:      "test-project_id",
		KindName:       "test-kind_name",
		PluralOverride: openapi.PtrString("test-plural_override"),
		Description:    openapi.PtrString("test-description"),
	}

	entityDefinitionOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsPost(ctx).EntityDefinition(entityDefinitionInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*entityDefinitionOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*entityDefinitionOutput.Kind).To(Equal("EntityDefinition"))
	Expect(*entityDefinitionOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/entity_definitions/%s", *entityDefinitionOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/entity_definitions"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestEntityDefinitionPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	entityDefinitionModel, err := newEntityDefinition(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	entityDefinitionOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsIdPatch(ctx, entityDefinitionModel.ID).EntityDefinitionPatchRequest(openapi.EntityDefinitionPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*entityDefinitionOutput.Id).To(Equal(entityDefinitionModel.ID))
	Expect(*entityDefinitionOutput.CreatedAt).To(BeTemporally("~", entityDefinitionModel.CreatedAt))
	Expect(*entityDefinitionOutput.Kind).To(Equal("EntityDefinition"))
	Expect(*entityDefinitionOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/entity_definitions/%s", *entityDefinitionOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/entity_definitions/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestEntityDefinitionPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newEntityDefinitionList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting entityDefinition list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting entityDefinition list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestEntityDefinitionListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	entityDefinitions, err := newEntityDefinitionList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", entityDefinitions[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexAiV1EntityDefinitionsGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting entityDefinition list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(entityDefinitions[0].ID))
}
