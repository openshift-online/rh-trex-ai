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
	"Diplodocus", "Apatosaurus", "Camarasaurus", "Titanosaurus", "Argentinosaurus",
	"Saltasaurus", "Amargasaurus", "Nigersaurus", "Suchomimus", "Acrocanthosaurus",
	"Carcharodontosaurus", "Mapusaurus", "Concavenator", "Megalosaurus", "Torvosaurus",
	"Coelophysis", "Herrerasaurus", "Plateosaurus", "Massospondylus", "Lufengosaurus",
	"Lesothosaurus", "Heterodontosaurus", "Pisanosaurus", "Eoraptor", "Panphagia",
	"Kentrosaurus", "Tuojiangosaurus", "Huayangosaurus", "Gigantspinosaurus", "Dacentrurus",
	"Polacanthus", "Nodosaurus", "Edmontonia", "Euoplocephalus", "Saichania",
	"Tarchia", "Pinacosaurus", "Minmi", "Gastonia", "Sauropelta",
	"Corythosaurus", "Lambeosaurus", "Edmontosaurus", "Hadrosaurus", "Saurolophus",
	"Tsintaosaurus", "Ouranosaurus", "Tenontosaurus", "Dryosaurus", "Camptosaurus",
	"Hypsilophodon", "Leaellynasaura", "Muttaburrasaurus", "Rhabdodon", "Zalmoxes",
	"Microraptor", "Sinornithosaurus", "Bambiraptor", "Utahraptor", "Austroraptor",
	"Buitreraptor", "Troodon", "Saurornithoides", "Byronosaurus", "Mei",
	"Citipati", "Khaan", "Rinchenia", "Avimimus", "Caudipteryx",
	"Nomingia", "Chirostenotes", "Elmisaurus", "Struthiomimus", "Ornithomimus",
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

func TestGRPCSourceSinkDinosaurs(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	h.StartControllersServer()

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)
	jwtToken := h.CreateJWTString(account)

	const totalDinosaurs = 100

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
		speciesSet[fmt.Sprintf("%s_%d", dinoSpecies[i], i)] = true
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

		for species := range speciesSet {
			dino := openapi.Dinosaur{Species: species}
			_, resp, postErr := client.DefaultAPI.ApiRhTrexV1DinosaursPost(ctx).Dinosaur(dino).Execute()
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

	listResp, listErr := grpcClient.ListDinosaurs(context.Background(), &pb.ListDinosaursRequest{
		Page: 1,
		Size: 500,
	})
	Expect(listErr).NotTo(HaveOccurred())
	Expect(int(listResp.Metadata.Total)).To(BeNumerically(">=", totalDinosaurs))
}
