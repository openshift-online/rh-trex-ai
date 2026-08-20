package fossils_test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/openshift-online/rh-trex-ai/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/test"
)

type bearerToken struct {
	token string
}

func (b *bearerToken) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + b.token,
	}, nil
}

func (b *bearerToken) RequireTransportSecurity() bool {
	return false
}

func TestGRPCFossilCRUD(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	h.StartControllersServer()

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

	// Test Create
	estimatedAge := int32(65000000)
	createReq := &pb.CreateFossilRequest{
		DiscoveryLocation: "Montana, USA",
		EstimatedAge:      &estimatedAge,
		FossilType:        func() *string { s := "Dinosaur Bone"; return &s }(),
		ExcavatorName:     func() *string { s := "Dr. Paleontologist"; return &s }(),
	}
	created, err := grpcClient.CreateFossil(ctx, createReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.DiscoveryLocation).To(Equal("Montana, USA"))
	Expect(created.EstimatedAge).To(Equal(&estimatedAge))
	Expect(created.Metadata.Id).NotTo(BeEmpty())

	fossilID := created.Metadata.Id

	// Test Get
	getReq := &pb.GetFossilRequest{Id: fossilID}
	retrieved, err := grpcClient.GetFossil(ctx, getReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(retrieved.DiscoveryLocation).To(Equal("Montana, USA"))
	Expect(retrieved.Metadata.Id).To(Equal(fossilID))

	// Test Update
	newAge := int32(70000000)
	updateReq := &pb.UpdateFossilRequest{
		Id:                fossilID,
		DiscoveryLocation: func() *string { s := "Colorado, USA"; return &s }(),
		EstimatedAge:      &newAge,
		FossilType:        func() *string { s := "Updated Fossil Type"; return &s }(),
		ExcavatorName:     func() *string { s := "Dr. Updated Paleontologist"; return &s }(),
	}
	updated, err := grpcClient.UpdateFossil(ctx, updateReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(updated.DiscoveryLocation).To(Equal("Colorado, USA"))
	Expect(updated.EstimatedAge).To(Equal(&newAge))
	Expect(updated.Metadata.Id).To(Equal(fossilID))

	// Test List
	listReq := &pb.ListFossilsRequest{
		Page: 1,
		Size: 10,
	}
	listResp, err := grpcClient.ListFossils(ctx, listReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(listResp.Metadata.Total).To(BeNumerically(">=", 1))

	// Test Delete
	deleteReq := &pb.DeleteFossilRequest{Id: fossilID}
	_, err = grpcClient.DeleteFossil(ctx, deleteReq)
	Expect(err).NotTo(HaveOccurred())

	// Verify deletion
	_, err = grpcClient.GetFossil(ctx, getReq)
	Expect(err).To(HaveOccurred())
}

func TestGRPCWatchFossils(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	h.StartControllersServer()

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)
	jwtToken := h.CreateJWTString(account)

	const totalItems = 25

	conn, err := grpc.NewClient(
		h.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerToken{token: jwtToken}),
	)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()

	grpcClient := pb.NewFossilServiceClient(conn)

	locationNames := make(map[string]bool, totalItems)
	for i := 0; i < totalItems; i++ {
		locationNames[fmt.Sprintf("Site_%d", i)] = true
	}

	var sourceErr error
	var sinkErr error
	var wg sync.WaitGroup
	wg.Add(2)

	sinkReady := make(chan struct{})

	go func() {
		defer wg.Done()
		<-sinkReady
		time.Sleep(100 * time.Millisecond)

		for location := range locationNames {
			fossilInput := openapi.Fossil{
				DiscoveryLocation: location,
			}
			_, resp, postErr := client.DefaultAPI.CreateFossil(ctx).Fossil(fossilInput).Execute()
			if postErr != nil {
				sourceErr = fmt.Errorf("REST POST failed for %s: %v", location, postErr)
				return
			}
			if resp.StatusCode != 201 {
				sourceErr = fmt.Errorf("REST POST unexpected status %d for %s", resp.StatusCode, location)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()

		watchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		stream, streamErr := grpcClient.WatchFossils(watchCtx, &pb.WatchFossilsRequest{})
		if streamErr != nil {
			sinkErr = fmt.Errorf("WatchFossils failed: %v", streamErr)
			close(sinkReady)
			return
		}

		close(sinkReady)

		seen := make(map[string]bool)
		for {
			evt, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				if watchCtx.Err() != nil {
					sinkErr = fmt.Errorf("sink timed out: saw %d/%d items", len(seen), totalItems)
				} else {
					sinkErr = fmt.Errorf("stream recv error: %v", recvErr)
				}
				return
			}

			if evt.Type != pb.EventType_EVENT_TYPE_CREATED {
				continue
			}

			if evt.ResourceId != "" {
				seen[evt.ResourceId] = true
			}

			if len(seen) == totalItems {
				return
			}
		}
	}()

	wg.Wait()

	Expect(sourceErr).NotTo(HaveOccurred(), "source goroutine error")
	Expect(sinkErr).NotTo(HaveOccurred(), "sink goroutine error")

	listResp, listErr := grpcClient.ListFossils(context.Background(), &pb.ListFossilsRequest{
		Page: 1,
		Size: 100,
	})
	Expect(listErr).NotTo(HaveOccurred())
	Expect(int(listResp.Metadata.Total)).To(BeNumerically(">=", totalItems))
}

func TestGRPCFossilErrorHandling(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	h.StartControllersServer()

	account := h.NewRandAccount()
	jwtToken := h.CreateJWTString(account)

	conn, err := grpc.NewClient(
		h.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerToken{token: jwtToken}),
	)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()

	grpcClient := pb.NewFossilServiceClient(conn)

	getReq := &pb.GetFossilRequest{Id: "nonexistent"}
	_, err = grpcClient.GetFossil(context.Background(), getReq)
	Expect(err).To(HaveOccurred())

	deleteReq := &pb.DeleteFossilRequest{Id: "nonexistent"}
	_, err = grpcClient.DeleteFossil(context.Background(), deleteReq)
}
