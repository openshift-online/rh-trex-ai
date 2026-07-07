package builds_test

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

func TestBuildGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexAiV1BuildsIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexAiV1BuildsIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	buildModel, err := newBuild(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	buildOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1BuildsIdGet(ctx, buildModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*buildOutput.Id).To(Equal(buildModel.ID), "found object does not match test object")
	Expect(*buildOutput.Kind).To(Equal("Build"))
	Expect(*buildOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/builds/%s", buildModel.ID)))
	Expect(*buildOutput.CreatedAt).To(BeTemporally("~", buildModel.CreatedAt))
	Expect(*buildOutput.UpdatedAt).To(BeTemporally("~", buildModel.UpdatedAt))
}

func TestBuildPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	buildInput := openapi.Build{
		ProjectId:   "test-project_id",
		Status:      "test-status",
		BuildLog:    openapi.PtrString("test-build_log"),
		TriggeredBy: openapi.PtrString("test-triggered_by"),
		CompletedAt: openapi.PtrTime(time.Now()),
	}

	buildOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1BuildsPost(ctx).Build(buildInput).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*buildOutput.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(*buildOutput.Kind).To(Equal("Build"))
	Expect(*buildOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/builds/%s", *buildOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/builds"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestBuildPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	buildModel, err := newBuild(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	buildOutput, resp, err := client.DefaultAPI.ApiRhTrexAiV1BuildsIdPatch(ctx, buildModel.ID).BuildPatchRequest(openapi.BuildPatchRequest{}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*buildOutput.Id).To(Equal(buildModel.ID))
	Expect(*buildOutput.CreatedAt).To(BeTemporally("~", buildModel.CreatedAt))
	Expect(*buildOutput.Kind).To(Equal("Build"))
	Expect(*buildOutput.Href).To(Equal(fmt.Sprintf("/api/rh-trex-ai/v1/builds/%s", *buildOutput.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/builds/foo"))

	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestBuildPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newBuildList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexAiV1BuildsGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting build list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexAiV1BuildsGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting build list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestBuildListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	builds, err := newBuildList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", builds[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexAiV1BuildsGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting build list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(builds[0].ID))
}
