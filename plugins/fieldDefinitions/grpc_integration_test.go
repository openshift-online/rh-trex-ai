package fieldDefinitions_test

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

func TestGRPCFieldDefinitionCRUD(t *testing.T) {
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

	grpcClient := pb.NewFieldDefinitionServiceClient(conn)

	createReq := &pb.CreateFieldDefinitionRequest{
		EntityDefinitionId: "TestEntityDefinitionId",
		FieldName:          "TestFieldName",
		FieldType:          "TestFieldType",
		IsRequired:         func() *bool { v := true; return &v }(),
	}
	created, err := grpcClient.CreateFieldDefinition(ctx, createReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.Metadata.Id).NotTo(BeEmpty())

	fieldDefinitionID := created.Metadata.Id

	getReq := &pb.GetFieldDefinitionRequest{Id: fieldDefinitionID}
	retrieved, err := grpcClient.GetFieldDefinition(ctx, getReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(retrieved.Metadata.Id).To(Equal(fieldDefinitionID))

	updateReq := &pb.UpdateFieldDefinitionRequest{
		Id:                 fieldDefinitionID,
		EntityDefinitionId: func() *string { s := "UpdatedEntityDefinitionId"; return &s }(),
		FieldName:          func() *string { s := "UpdatedFieldName"; return &s }(),
		FieldType:          func() *string { s := "UpdatedFieldType"; return &s }(),
		IsRequired:         func() *bool { v := false; return &v }(),
	}
	updated, err := grpcClient.UpdateFieldDefinition(ctx, updateReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(updated.Metadata.Id).To(Equal(fieldDefinitionID))

	listReq := &pb.ListFieldDefinitionsRequest{
		Page: 1,
		Size: 10,
	}
	listResp, err := grpcClient.ListFieldDefinitions(ctx, listReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(listResp.Metadata.Total).To(BeNumerically(">=", 1))

	deleteReq := &pb.DeleteFieldDefinitionRequest{Id: fieldDefinitionID}
	_, err = grpcClient.DeleteFieldDefinition(ctx, deleteReq)
	Expect(err).NotTo(HaveOccurred())

	_, err = grpcClient.GetFieldDefinition(ctx, getReq)
	Expect(err).To(HaveOccurred())
}

func TestGRPCWatchFieldDefinitions(t *testing.T) {
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

	grpcClient := pb.NewFieldDefinitionServiceClient(conn)

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
			fieldDefinitionInput := openapi.FieldDefinition{}
			_, resp, postErr := client.DefaultAPI.ApiRhTrexAiV1FieldDefinitionsPost(ctx).FieldDefinition(fieldDefinitionInput).Execute()
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

		stream, streamErr := grpcClient.WatchFieldDefinitions(watchCtx, &pb.WatchFieldDefinitionsRequest{})
		if streamErr != nil {
			sinkErr = fmt.Errorf("WatchFieldDefinitions failed: %v", streamErr)
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

	listResp, listErr := grpcClient.ListFieldDefinitions(context.Background(), &pb.ListFieldDefinitionsRequest{
		Page: 1,
		Size: 100,
	})
	Expect(listErr).NotTo(HaveOccurred())
	Expect(int(listResp.Metadata.Total)).To(BeNumerically(">=", totalItems))
}

func TestGRPCFieldDefinitionErrorHandling(t *testing.T) {
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

	grpcClient := pb.NewFieldDefinitionServiceClient(conn)

	getReq := &pb.GetFieldDefinitionRequest{Id: "nonexistent"}
	_, err = grpcClient.GetFieldDefinition(context.Background(), getReq)
	Expect(err).To(HaveOccurred())

	deleteReq := &pb.DeleteFieldDefinitionRequest{Id: "nonexistent"}
	_, err = grpcClient.DeleteFieldDefinition(context.Background(), deleteReq)
}
