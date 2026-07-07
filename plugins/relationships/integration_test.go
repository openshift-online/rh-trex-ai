package relationships_test

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

func TestRelationshipGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	relationshipModel, err := newRelationship(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	relationshipOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsIdGet(ctx, relationshipModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*relationshipOutput.Id).To(Equal(relationshipModel.ID), "found object does not match test object")
	Expect(*relationshipOutput.Kind).To(Equal("Relationship"))
	Expect(*relationshipOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/relationships/%s", relationshipModel.ID)))
	Expect(*relationshipOutput.CreatedAt).To(BeTemporally("~", relationshipModel.CreatedAt))
	Expect(*relationshipOutput.UpdatedAt).To(BeTemporally("~", relationshipModel.UpdatedAt))
}

func TestRelationshipPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	relationshipInput := openapi.Relationship{
		ProjectId:        "test-project_id",
		SourceEntityId:   "test-source_entity_id",
		TargetEntityId:   "test-target_entity_id",
		RelationshipType: "test-relationship_type",
		ForeignKey:       openapi.PtrString("test-foreign_key"),
	}

	relationshipOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsPost(ctx).Relationship(relationshipInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*relationshipOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*relationshipOutput.Kind).To(Equal("Relationship"))
	Expect(*relationshipOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/relationships/%s", *relationshipOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/relationships"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestRelationshipPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	relationshipModel, err := newRelationship(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	relationshipOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsIdPatch(ctx, relationshipModel.ID).RelationshipPatchRequest(openapi.RelationshipPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*relationshipOutput.Id).To(Equal(relationshipModel.ID))
	Expect(*relationshipOutput.CreatedAt).To(BeTemporally("~", relationshipModel.CreatedAt))
	Expect(*relationshipOutput.Kind).To(Equal("Relationship"))
	Expect(*relationshipOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/relationships/%s", *relationshipOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/relationships/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestRelationshipPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newRelationshipList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting relationship list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexAiV1RelationshipsGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting relationship list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestRelationshipListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	relationships, err := newRelationshipList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", relationships[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexAiV1RelationshipsGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting relationship list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(relationships[0].ID))
}
