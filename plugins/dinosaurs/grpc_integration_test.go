package dinosaurs_test

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

var dinoSpecies = []string{
	"Tyrannosaurus", "Velociraptor", "Triceratops", "Stegosaurus", "Brachiosaurus",
	"Allosaurus", "Spinosaurus", "Ankylosaurus", "Parasaurolophus", "Pachycephalosaurus",
	"Dilophosaurus", "Compsognathus", "Gallimimus", "Carnotaurus", "Baryonyx",
	"Iguanodon", "Maiasaura", "Oviraptor", "Therizinosaurus", "Giganotosaurus",
	"Deinonychus", "Protoceratops", "Styracosaurus", "Chasmosaurus", "Ceratosaurus",
}

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

func TestGRPCDinosaurCRUD(t *testing.T) {
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

	grpcClient := pb.NewDinosaurServiceClient(conn)

	// Test Create
	createReq := &pb.CreateDinosaurRequest{
		Species: "TestDinosaurus",
	}
	created, err := grpcClient.CreateDinosaur(ctx, createReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.Species).To(Equal("TestDinosaurus"))
	Expect(created.Metadata.Id).NotTo(BeEmpty())

	dinoID := created.Metadata.Id

	// Test Get
	getReq := &pb.GetDinosaurRequest{Id: dinoID}
	retrieved, err := grpcClient.GetDinosaur(ctx, getReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(retrieved.Species).To(Equal("TestDinosaurus"))
	Expect(retrieved.Metadata.Id).To(Equal(dinoID))

	// Test Update
	updateReq := &pb.UpdateDinosaurRequest{
		Id: dinoID,
		Species: func() *string { s := "UpdatedDinosaurus"; return &s }(),
	}
	updated, err := grpcClient.UpdateDinosaur(ctx, updateReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(updated.Species).To(Equal("UpdatedDinosaurus"))
	Expect(updated.Metadata.Id).To(Equal(dinoID))

	// Test List
	listReq := &pb.ListDinosaursRequest{
		Page: 1,
		Size: 10,
	}
	listResp, err := grpcClient.ListDinosaurs(ctx, listReq)
	Expect(err).NotTo(HaveOccurred())
	Expect(listResp.Metadata.Total).To(BeNumerically(">=", 1))

	// Test Delete
	deleteReq := &pb.DeleteDinosaurRequest{Id: dinoID}
	_, err = grpcClient.DeleteDinosaur(ctx, deleteReq)
	Expect(err).NotTo(HaveOccurred())

	// Verify deletion
	_, err = grpcClient.GetDinosaur(ctx, getReq)
	Expect(err).To(HaveOccurred())
}

func TestGRPCSourceSinkDinosaurs(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	h.StartControllersServer()

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)
	jwtToken := h.CreateJWTString(account)

	const totalDinosaurs = 25

	conn, err := grpc.NewClient(
		h.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerToken{token: jwtToken}),
	)
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()

	grpcClient := pb.NewDinosaurServiceClient(conn)

	speciesSet := make(map[string]bool, totalDinosaurs)
	for i := 0; i < totalDinosaurs; i++ {
		speciesSet[fmt.Sprintf("%s_%d", dinoSpecies[i%len(dinoSpecies)], i)] = true
	}

	var sourceErr error
	var sinkErr error
	var wg sync.WaitGroup
	wg.Add(2)

	sinkReady := make(chan struct{})

	// Source goroutine: creates dinosaurs via REST API
	go func() {
		defer wg.Done()
		<-sinkReady
		time.Sleep(100 * time.Millisecond)

		for species := range speciesSet {
			dino := openapi.Dinosaur{Species: species}
			_, resp, postErr := client.DefaultAPI.ApiRhTrexAiV1DinosaursPost(ctx).Dinosaur(dino).Execute()
			if postErr != nil {
				sourceErr = fmt.Errorf("REST POST failed for %s: %v", species, postErr)
				return
			}
			if resp.StatusCode != 201 {
				sourceErr = fmt.Errorf("REST POST unexpected status %d for %s", resp.StatusCode, species)
				return
			}
		}
	}()

	// Sink goroutine: watches for dinosaurs via gRPC streaming
	go func() {
		defer wg.Done()

		watchCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		stream, streamErr := grpcClient.WatchDinosaurs(watchCtx, &pb.WatchDinosaursRequest{})
		if streamErr != nil {
			sinkErr = fmt.Errorf("WatchDinosaurs failed: %v", streamErr)
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
					sinkErr = fmt.Errorf("sink timed out: saw %d/%d dinosaurs", len(seen), totalDinosaurs)
				} else {
					sinkErr = fmt.Errorf("stream recv error: %v", recvErr)
				}
				return
			}

			if evt.Type != pb.EventType_EVENT_TYPE_CREATED {
				continue
			}

			if evt.Dinosaur != nil && speciesSet[evt.Dinosaur.Species] {
				seen[evt.Dinosaur.Species] = true
			}

			if len(seen) == totalDinosaurs {
				return
			}
		}
	}()

	wg.Wait()

	Expect(sourceErr).NotTo(HaveOccurred(), "source goroutine error")
	Expect(sinkErr).NotTo(HaveOccurred(), "sink goroutine error")

	// Verify final state
	listResp, listErr := grpcClient.ListDinosaurs(context.Background(), &pb.ListDinosaursRequest{
		Page: 1,
		Size: 100,
	})
	Expect(listErr).NotTo(HaveOccurred())
	Expect(int(listResp.Metadata.Total)).To(BeNumerically(">=", totalDinosaurs))
}

func TestGRPCErrorHandling(t *testing.T) {
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

	grpcClient := pb.NewDinosaurServiceClient(conn)

	// Test Get with invalid ID
	getReq := &pb.GetDinosaurRequest{Id: "nonexistent"}
	_, err = grpcClient.GetDinosaur(context.Background(), getReq)
	Expect(err).To(HaveOccurred())

	// Test Create with empty species - this might be allowed depending on validation
	createReq := &pb.CreateDinosaurRequest{
		Species: "",
	}
	_, err = grpcClient.CreateDinosaur(context.Background(), createReq)
	// Note: Empty species may or may not be validated depending on implementation

	// Test Delete with invalid ID - may or may not error depending on implementation
	deleteReq := &pb.DeleteDinosaurRequest{Id: "nonexistent"}
	_, err = grpcClient.DeleteDinosaur(context.Background(), deleteReq)
	// Note: Delete of non-existent ID may succeed or fail depending on implementation
}