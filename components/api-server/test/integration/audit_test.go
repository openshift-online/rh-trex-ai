package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/resty.v1"

	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

// auditRecorder returns the integration environment's in-memory recorder, emptied.
func auditRecorder(h *test.Helper) *audit.MemoryRecorder {
	rec, ok := h.Env().Audit.Recorder().(*audit.MemoryRecorder)
	Expect(ok).To(BeTrue(), "the integration environment must use audit.MemoryRecorder")
	rec.Reset()
	return rec
}

func TestAuditRESTRequests(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	rec := auditRecorder(h)
	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)
	username := strings.ToLower(account.Username)

	created, resp, err := client.DefaultAPI.CreateDinosaur(ctx).Dinosaur(openapi.Dinosaur{Species: "audited"}).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))

	// An allowed read is not recorded by default.
	_, _, err = client.DefaultAPI.GetDinosaur(ctx, *created.Id).Execute()
	Expect(err).NotTo(HaveOccurred())

	// A failed mutation is recorded as a failure with its status.
	_, resp, err = client.DefaultAPI.UpdateDinosaur(ctx, "missing").DinosaurPatchRequest(openapi.DinosaurPatchRequest{}).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	// A request the JWT handler refuses before routing is recorded.
	_, _, err = client.DefaultAPI.GetDinosaur(context.Background(), *created.Id).Execute()
	Expect(err).To(HaveOccurred())

	// Credentials, the query string, and the body stay out of the event.
	jwtToken := ctx.Value(openapi.ContextAccessToken).(string)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+jwtToken).
		SetBody(`{"species":"secret-species"}`).
		Post(h.RestURL("/dinosaurs?apikey=query-secret"))
	Expect(err).NotTo(HaveOccurred())
	Expect(restyResp.StatusCode()).To(Equal(http.StatusCreated))

	events := rec.Events()
	Expect(events).To(HaveLen(4), "events: %+v", events)

	Expect(events[0].Outcome).To(Equal(audit.OutcomeSuccess))
	Expect(events[0].Action).To(Equal("create"))
	Expect(events[0].Actor.Type).To(Equal(audit.ActorUser))
	Expect(events[0].Actor.Username).To(Equal(username))
	Expect(events[0].Resource.Kind).To(Equal("dinosaurs"))
	Expect(events[0].Resource.Surface).To(Equal("v1"))
	Expect(events[0].HTTP.Status).To(Equal(http.StatusCreated))
	Expect(events[0].OperationID).NotTo(BeEmpty())

	Expect(events[1].Outcome).To(Equal(audit.OutcomeFailure))
	Expect(events[1].Action).To(Equal("patch"))
	Expect(events[1].Resource.ID).To(Equal("missing"))
	Expect(events[1].HTTP.Status).To(Equal(http.StatusNotFound))

	Expect(events[2].Outcome).To(Equal(audit.OutcomeUnauthenticated))
	Expect(events[2].Actor.Type).To(Equal(audit.ActorAnonymous))
	Expect(events[2].HTTP.Status).To(Equal(http.StatusUnauthorized))

	Expect(events[3].Outcome).To(Equal(audit.OutcomeSuccess))
	dump, _ := json.Marshal(events)
	Expect(string(dump)).NotTo(ContainSubstring("query-secret"))
	Expect(string(dump)).NotTo(ContainSubstring("secret-species"))
	Expect(string(dump)).NotTo(ContainSubstring(jwtToken))
}

type bearer string

func (b bearer) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(b)}, nil
}
func (b bearer) RequireTransportSecurity() bool { return false }

func TestAuditGRPCRequests(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	rec := auditRecorder(h)
	account := h.NewRandAccount()

	conn, err := grpc.NewClient(h.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(bearer(h.CreateJWTString(account))))
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	client := pb.NewDinosaurServiceClient(conn)

	created, err := client.CreateDinosaur(context.Background(), &pb.CreateDinosaurRequest{Species: "grpc-audited"})
	Expect(err).NotTo(HaveOccurred())
	Expect(created.GetMetadata().GetId()).NotTo(BeEmpty())
	_, err = client.UpdateDinosaur(context.Background(), &pb.UpdateDinosaurRequest{Id: "missing"})
	Expect(err).To(HaveOccurred())

	anonymous, err := grpc.NewClient(h.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(err).NotTo(HaveOccurred())
	defer anonymous.Close()
	_, err = pb.NewDinosaurServiceClient(anonymous).CreateDinosaur(context.Background(), &pb.CreateDinosaurRequest{Species: "x"})
	Expect(err).To(HaveOccurred())

	events := rec.Events()
	Expect(events).To(HaveLen(3), "events: %+v", events)
	Expect(events[0].Transport).To(Equal(audit.TransportGRPC))
	Expect(events[0].Outcome).To(Equal(audit.OutcomeSuccess))
	Expect(events[0].Action).To(Equal("create"))
	Expect(events[0].GRPC.Method).To(HaveSuffix("/CreateDinosaur"))
	Expect(events[0].Actor.Username).To(Equal(strings.ToLower(account.Username)))
	Expect(events[1].Outcome).To(Equal(audit.OutcomeFailure))
	Expect(events[1].GRPC.Code).NotTo(Equal("OK"))
	Expect(events[2].Outcome).To(Equal(audit.OutcomeUnauthenticated))
	Expect(events[2].GRPC.Code).To(Equal("Unauthenticated"))
}
