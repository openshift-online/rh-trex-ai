package scientists_test

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

func TestGRPCScientistCRUD(t *testing.T) {
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

	grpcClient := pb.NewScientistServiceClient(conn)

	createReq := &pb.CreateScientistRequest{
		Name:  "Jack Horner",
		Field: "Paleontology",
	}
	created, err := grpcClient.CreateScientist(ctx, createReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.Metadata.Id).NotTo(BeEmpty())
	Expect(created.Name).To(Equal("Jack Horner"))
	Expect(created.Field).To(Equal("Paleontology"))

	scientistID := created.Metadata.Id

	getReq := &pb.GetScientistRequest{Id: scientistID}
	retrieved, err := grpcClient.GetScientist(ctx, getReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(retrieved.Metadata.Id).To(Equal(scientistID))
	Expect(retrieved.Name).To(Equal("Jack Horner"))

	updateReq := &pb.UpdateScientistRequest{
		Id:    scientistID,
		Name:  func() *string { s := "Mary Anning"; return &s }(),
		Field: func() *string { s := "Fossil Hunting"; return &s }(),
	}
	updated, err := grpcClient.UpdateScientist(ctx, updateReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(updated.Metadata.Id).To(Equal(scientistID))
	Expect(updated.Name).To(Equal("Mary Anning"))
	Expect(updated.Field).To(Equal("Fossil Hunting"))

	listReq := &pb.ListScientistsRequest{
		Page: 1,
		Size: 10,
	}
	listResp, err := grpcClient.ListScientists(ctx, listReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(listResp.Metadata.Total).To(BeNumerically(">=", 1))

	deleteReq := &pb.DeleteScientistRequest{Id: scientistID}
	_, err = grpcClient.DeleteScientist(ctx, deleteReq)
	Expect(err).NotTo(HaveOccurred())

	_, err = grpcClient.GetScientist(ctx, getReq)
	Expect(err).To(HaveOccurred())
}

func TestGRPCWatchScientists(t *testing.T) {
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

	grpcClient := pb.NewScientistServiceClient(conn)

	scientistNames := make(map[string]bool, totalItems)
	for i := 0; i < totalItems; i++ {
		scientistNames[fmt.Sprintf("Scientist_%d", i)] = true
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

		for name := range scientistNames {
			scientistInput := openapi.Scientist{
				Name:  name,
				Field: "Paleontology",
			}
			_, resp, postErr := client.DefaultAPI.CreateScientist(ctx).Scientist(scientistInput).Execute()
			if postErr != nil {
				sourceErr = fmt.Errorf("REST POST failed for %s: %v", name, postErr)
				return
			}
			if resp.StatusCode != 201 {
				sourceErr = fmt.Errorf("REST POST unexpected status %d for %s", resp.StatusCode, name)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()

		watchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		stream, streamErr := grpcClient.WatchScientists(watchCtx, &pb.WatchScientistsRequest{})
		if streamErr != nil {
			sinkErr = fmt.Errorf("WatchScientists failed: %v", streamErr)
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

	listResp, listErr := grpcClient.ListScientists(context.Background(), &pb.ListScientistsRequest{
		Page: 1,
		Size: 100,
	})
	Expect(listErr).NotTo(HaveOccurred())
	Expect(int(listResp.Metadata.Total)).To(BeNumerically(">=", totalItems))
}

func TestGRPCScientistErrorHandling(t *testing.T) {
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

	grpcClient := pb.NewScientistServiceClient(conn)

	getReq := &pb.GetScientistRequest{Id: "nonexistent"}
	_, err = grpcClient.GetScientist(context.Background(), getReq)
	Expect(err).To(HaveOccurred())

	deleteReq := &pb.DeleteScientistRequest{Id: "nonexistent"}
	_, err = grpcClient.DeleteScientist(context.Background(), deleteReq)
}
