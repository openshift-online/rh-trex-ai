package fieldDefinitions_test

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

func TestFieldDefinitionGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	fieldDefinitionModel, err := newFieldDefinition(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	fieldDefinitionOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsIdGet(ctx, fieldDefinitionModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*fieldDefinitionOutput.Id).To(Equal(fieldDefinitionModel.ID), "found object does not match test object")
	Expect(*fieldDefinitionOutput.Kind).To(Equal("FieldDefinition"))
	Expect(*fieldDefinitionOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/field_definitions/%s", fieldDefinitionModel.ID)))
	Expect(*fieldDefinitionOutput.CreatedAt).To(BeTemporally("~", fieldDefinitionModel.CreatedAt))
	Expect(*fieldDefinitionOutput.UpdatedAt).To(BeTemporally("~", fieldDefinitionModel.UpdatedAt))
}

func TestFieldDefinitionPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fieldDefinitionInput := openapi.FieldDefinition{
		EntityDefinitionId: "test-entity_definition_id",
		FieldName:          "test-field_name",
		FieldType:          "test-field_type",
		IsRequired:         openapi.PtrBool(true),
	}

	fieldDefinitionOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsPost(ctx).FieldDefinition(fieldDefinitionInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*fieldDefinitionOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*fieldDefinitionOutput.Kind).To(Equal("FieldDefinition"))
	Expect(*fieldDefinitionOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/field_definitions/%s", *fieldDefinitionOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/field_definitions"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestFieldDefinitionPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fieldDefinitionModel, err := newFieldDefinition(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	fieldDefinitionOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsIdPatch(ctx, fieldDefinitionModel.ID).FieldDefinitionPatchRequest(openapi.FieldDefinitionPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*fieldDefinitionOutput.Id).To(Equal(fieldDefinitionModel.ID))
	Expect(*fieldDefinitionOutput.CreatedAt).To(BeTemporally("~", fieldDefinitionModel.CreatedAt))
	Expect(*fieldDefinitionOutput.Kind).To(Equal("FieldDefinition"))
	Expect(*fieldDefinitionOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/field_definitions/%s", *fieldDefinitionOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/field_definitions/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestFieldDefinitionPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newFieldDefinitionList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting fieldDefinition list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting fieldDefinition list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestFieldDefinitionListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fieldDefinitions, err := newFieldDefinitionList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", fieldDefinitions[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting fieldDefinition list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(fieldDefinitions[0].ID))
}
