package dinosaurs_test

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

func TestDinosaurGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	dinosaurModel, err := newDinosaur(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	dinosaurOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursIdGet(ctx, dinosaurModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*dinosaurOutput.Id).To(Equal(dinosaurModel.ID), "found object does not match test object")
	Expect(*dinosaurOutput.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaurOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/dinosaurs/%s", dinosaurModel.ID)))
	Expect(*dinosaurOutput.CreatedAt).To(BeTemporally("~", dinosaurModel.CreatedAt))
	Expect(*dinosaurOutput.UpdatedAt).To(BeTemporally("~", dinosaurModel.UpdatedAt))
}

func TestDinosaurPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dinosaurInput := openapi.Dinosaur{
		Species: "test-species",
	}

	dinosaurOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursPost(ctx).Dinosaur(dinosaurInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*dinosaurOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*dinosaurOutput.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaurOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/dinosaurs/%s", *dinosaurOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/dinosaurs"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestDinosaurPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dinosaurModel, err := newDinosaur(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	dinosaurOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursIdPatch(ctx, dinosaurModel.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*dinosaurOutput.Id).To(Equal(dinosaurModel.ID))
	Expect(*dinosaurOutput.CreatedAt).To(BeTemporally("~", dinosaurModel.CreatedAt))
	Expect(*dinosaurOutput.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaurOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/dinosaurs/%s", *dinosaurOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/dinosaurs/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestDinosaurPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newDinosaurList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexAiV1DinosaursGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestDinosaurListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dinosaurs, err := newDinosaurList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", dinosaurs[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexAiV1DinosaursGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(dinosaurs[0].ID))
}
