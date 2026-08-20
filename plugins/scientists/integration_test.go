package scientists_test

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

func TestScientistGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.GetScientist(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.GetScientist(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	scientistModel, err := newScientist(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	scientistOutput, resp, err := client.DefaultAPI.GetScientist(ctx, scientistModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*scientistOutput.Id).To(Equal(scientistModel.ID), "found object does not match test object")
	Expect(*scientistOutput.Kind).To(Equal("Scientist"))
	Expect(*scientistOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/scientists/%s", scientistModel.ID)))
	Expect(*scientistOutput.CreatedAt).To(BeTemporally("~", scientistModel.CreatedAt))
	Expect(*scientistOutput.UpdatedAt).To(BeTemporally("~", scientistModel.UpdatedAt))
}

func TestScientistPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	scientistInput := openapi.Scientist{
		Name:  "test-name",
		Field: "test-field",
	}

	scientistOutput, resp, err := client.DefaultAPI.CreateScientist(ctx).Scientist(scientistInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*scientistOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*scientistOutput.Kind).To(Equal("Scientist"))
	Expect(*scientistOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/scientists/%s", *scientistOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/scientists"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestScientistPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	scientistModel, err := newScientist(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	scientistOutput, resp, err := client.DefaultAPI.UpdateScientist(ctx, scientistModel.ID).ScientistPatchRequest(openapi.ScientistPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*scientistOutput.Id).To(Equal(scientistModel.ID))
	Expect(*scientistOutput.CreatedAt).To(BeTemporally("~", scientistModel.CreatedAt))
	Expect(*scientistOutput.Kind).To(Equal("Scientist"))
	Expect(*scientistOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/scientists/%s", *scientistOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/scientists/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestScientistDelete(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	scientistModel, err := newScientist(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	resp, err := client.DefaultAPI.DeleteScientist(ctx, scientistModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error deleting object: %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

	_, resp, err = client.DefaultAPI.GetScientist(ctx, scientistModel.ID).Execute()
	Expect(err).To(HaveOccurred(), "Expected deleted object to be absent")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
}

func TestScientistPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newScientistList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ListScientists(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting scientist list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ListScientists(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting scientist list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestScientistListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	scientists, err := newScientistList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", scientists[0].ID)
	list, _, err := client.DefaultAPI.ListScientists(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting scientist list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(scientists[0].ID))
}
