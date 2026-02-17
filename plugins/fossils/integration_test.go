package fossils_test

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

func TestFossilGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexAiV1FossilsIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexAiV1FossilsIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	fossilModel, err := newFossil(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	fossilOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1FossilsIdGet(ctx, fossilModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*fossilOutput.Id).To(Equal(fossilModel.ID), "found object does not match test object")
	Expect(*fossilOutput.Kind).To(Equal("Fossil"))
	Expect(*fossilOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/fossils/%s", fossilModel.ID)))
	Expect(*fossilOutput.CreatedAt).To(BeTemporally("~", fossilModel.CreatedAt))
	Expect(*fossilOutput.UpdatedAt).To(BeTemporally("~", fossilModel.UpdatedAt))
}

func TestFossilPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fossilInput := openapi.Fossil{
		DiscoveryLocation: "test-discovery_location",
		EstimatedAge:      openapi.PtrInt32(42),
		FossilType:        openapi.PtrString("test-fossil_type"),
		ExcavatorName:     openapi.PtrString("test-excavator_name"),
	}

	fossilOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1FossilsPost(ctx).Fossil(fossilInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*fossilOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*fossilOutput.Kind).To(Equal("Fossil"))
	Expect(*fossilOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/fossils/%s", *fossilOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/fossils"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestFossilPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fossilModel, err := newFossil(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	fossilOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1FossilsIdPatch(ctx, fossilModel.ID).FossilPatchRequest(openapi.FossilPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*fossilOutput.Id).To(Equal(fossilModel.ID))
	Expect(*fossilOutput.CreatedAt).To(BeTemporally("~", fossilModel.CreatedAt))
	Expect(*fossilOutput.Kind).To(Equal("Fossil"))
	Expect(*fossilOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/fossils/%s", *fossilOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/fossils/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestFossilPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newFossilList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexAiV1FossilsGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting fossil list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexAiV1FossilsGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting fossil list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestFossilListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fossils, err := newFossilList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", fossils[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexAiV1FossilsGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting fossil list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(fossils[0].ID))
}
