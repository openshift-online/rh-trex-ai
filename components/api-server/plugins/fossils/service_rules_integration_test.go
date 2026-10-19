package fossils_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

// The service, not the handlers, owns validation, so REST rejects the same
// input that gRPC does.
func TestFossilRESTValidation(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, resp, err := client.DefaultAPI.CreateFossil(ctx).Fossil(openapi.Fossil{DiscoveryLocation: ""}).Execute()
	Expect(err).To(HaveOccurred(), "Expected empty discovery_location to be rejected")
	Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

	fossilModel, err := newFossil(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	_, resp, err = client.DefaultAPI.UpdateFossil(ctx, fossilModel.ID).
		FossilPatchRequest(openapi.FossilPatchRequest{DiscoveryLocation: openapi.PtrString("")}).Execute()
	Expect(err).To(HaveOccurred(), "Expected a patch to be unable to clear a required field")
	Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

	_, resp, err = client.DefaultAPI.UpdateFossil(ctx, fossilModel.ID).
		FossilPatchRequest(openapi.FossilPatchRequest{ExcavatorName: openapi.PtrString(strings.Repeat("x", 256))}).Execute()
	Expect(err).To(HaveOccurred(), "Expected an overlong field to be rejected")
	Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

	stored, _, err := client.DefaultAPI.GetFossil(ctx, fossilModel.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(stored.DiscoveryLocation).To(Equal(fossilModel.DiscoveryLocation))
	Expect(*stored.ExcavatorName).To(Equal(*fossilModel.ExcavatorName))
}

// Two clients patching different fields at the same time must both see their
// change stored: the read-modify-write in Patch holds a row lock.
func TestFossilConcurrentPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	fossilModel, err := newFossil(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	for round := 0; round < 10; round++ {
		location := openapi.PtrString("location-" + strings.Repeat("l", round+1))
		excavator := openapi.PtrString("excavator-" + strings.Repeat("e", round+1))

		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, patch := range []openapi.FossilPatchRequest{
			{DiscoveryLocation: location},
			{ExcavatorName: excavator},
		} {
			wg.Add(1)
			go func(patch openapi.FossilPatchRequest) {
				defer wg.Done()
				_, _, err := client.DefaultAPI.UpdateFossil(ctx, fossilModel.ID).FossilPatchRequest(patch).Execute()
				errs <- err
			}(patch)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			Expect(err).NotTo(HaveOccurred())
		}

		stored, _, err := client.DefaultAPI.GetFossil(ctx, fossilModel.ID).Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.DiscoveryLocation).To(Equal(*location), "round %d lost the discovery_location patch", round)
		Expect(*stored.ExcavatorName).To(Equal(*excavator), "round %d lost the excavator_name patch", round)
	}
}

func TestGRPCFossilValidation(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)
	jwtToken := h.CreateJWTString(account)

	conn, err := grpc.NewClient(
		h.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerToken{token: jwtToken}),
	)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()

	grpcClient := pb.NewFossilServiceClient(conn)

	_, err = grpcClient.CreateFossil(ctx, &pb.CreateFossilRequest{DiscoveryLocation: ""})
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument))

	fossilModel, err := newFossil(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	empty := ""
	_, err = grpcClient.UpdateFossil(ctx, &pb.UpdateFossilRequest{Id: fossilModel.ID, DiscoveryLocation: &empty})
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument))

	overlong := strings.Repeat("x", 256)
	_, err = grpcClient.UpdateFossil(ctx, &pb.UpdateFossilRequest{Id: fossilModel.ID, ExcavatorName: &overlong})
	Expect(status.Code(err)).To(Equal(codes.InvalidArgument))
	Expect(status.Convert(err).Message()).To(ContainSubstring("excavator_name exceeds maximum length of 255"))
}
