package builds_test

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

func TestGRPCBuildCRUD(t *testing.T) {
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

	grpcClient := pb.NewBuildServiceClient(conn)

	createReq := &pb.CreateBuildRequest{
		ProjectId:   "TestProjectId",
		Status:      "TestStatus",
		BuildLog:    func() *string { s := "TestBuildLog"; return &s }(),
		TriggeredBy: func() *string { s := "TestTriggeredBy"; return &s }(),
	}
	created, err := grpcClient.CreateBuild(ctx, createReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.Metadata.Id).NotTo(BeEmpty())

	buildID := created.Metadata.Id

	getReq := &pb.GetBuildRequest{Id: buildID}
	retrieved, err := grpcClient.GetBuild(ctx, getReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(retrieved.Metadata.Id).To(Equal(buildID))

	updateReq := &pb.UpdateBuildRequest{
		Id:          buildID,
		ProjectId:   func() *string { s := "UpdatedProjectId"; return &s }(),
		Status:      func() *string { s := "UpdatedStatus"; return &s }(),
		BuildLog:    func() *string { s := "UpdatedBuildLog"; return &s }(),
		TriggeredBy: func() *string { s := "UpdatedTriggeredBy"; return &s }(),
	}
	updated, err := grpcClient.UpdateBuild(ctx, updateReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(updated.Metadata.Id).To(Equal(buildID))

	listReq := &pb.ListBuildsRequest{
		Page: 1,
		Size: 10,
	}
	listResp, err := grpcClient.ListBuilds(ctx, listReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(listResp.Metadata.Total).To(BeNumerically(">=", 1))

	deleteReq := &pb.DeleteBuildRequest{Id: buildID}
	_, err = grpcClient.DeleteBuild(ctx, deleteReq)
	Expect(err).NotTo(HaveOccurred())

	_, err = grpcClient.GetBuild(ctx, getReq)
	Expect(err).To(HaveOccurred())
}

func TestGRPCWatchBuilds(t *testing.T) {
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

	grpcClient := pb.NewBuildServiceClient(conn)

	itemNames := make(map[string]bool, totalItems)
	for i := 0; i < totalItems; i++ {
		itemNames[fmt.Sprintf("watch_test_%d", i)] = true
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

		for name := range itemNames {
			buildInput := openapi.Build{}
			_, resp, postErr := client.DefaultAPI.ApiRhTrexAiV1BuildsPost(ctx).Build(buildInput).Execute()
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

		stream, streamErr := grpcClient.WatchBuilds(watchCtx, &pb.WatchBuildsRequest{})
		if streamErr != nil {
			sinkErr = fmt.Errorf("WatchBuilds failed: %v", streamErr)
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

	listResp, listErr := grpcClient.ListBuilds(context.Background(), &pb.ListBuildsRequest{
		Page: 1,
		Size: 100,
	})
	Expect(listErr).NotTo(HaveOccurred())
	Expect(int(listResp.Metadata.Total)).To(BeNumerically(">=", totalItems))
}

func TestGRPCBuildErrorHandling(t *testing.T) {
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

	grpcClient := pb.NewBuildServiceClient(conn)

	getReq := &pb.GetBuildRequest{Id: "nonexistent"}
	_, err = grpcClient.GetBuild(context.Background(), getReq)
	Expect(err).To(HaveOccurred())

	deleteReq := &pb.DeleteBuildRequest{Id: "nonexistent"}
	_, err = grpcClient.DeleteBuild(context.Background(), deleteReq)
}
