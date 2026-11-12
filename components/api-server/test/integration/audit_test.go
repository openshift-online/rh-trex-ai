package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/resty.v1"
	"gorm.io/gorm"

	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/test"
)

type auditRow struct {
	ChainSeq      *int64
	EventID       string
	ActorType     string
	ActorUsername string
	Action        string
	Outcome       string
	Transport     string
	ResourceKind  string
	ResourceID    string
	HTTPMethod    string `gorm:"column:http_method"`
	HTTPStatus    int    `gorm:"column:http_status"`
	GRPCMethod    string `gorm:"column:grpc_method"`
	GRPCCode      string `gorm:"column:grpc_code"`
	OperationID   string
}

func auditRows(sf db.SessionFactory) []auditRow {
	var rows []auditRow
	Expect(sf.New(context.Background()).Table(audit.TableName).Order("ingest_seq").Find(&rows).Error).To(Succeed())
	return rows
}

func newAuditService(sf db.SessionFactory, key []byte, sink *bytes.Buffer) *audit.Service {
	cfg := config.NewAuditConfig()
	cfg.SetHMACKey(key)
	var opts []audit.Option
	if sink != nil {
		opts = append(opts, audit.WithSink(audit.NewWriterSink(sink)))
	} else {
		cfg.Sink = config.AuditSinkNone
	}
	s, err := audit.New(sf, cfg, opts...)
	Expect(err).NotTo(HaveOccurred())
	return s
}

// asSuperuser runs fn with triggers disabled, as a database owner trying to
// hide their tracks would.
func asSuperuser(sf db.SessionFactory, fn func(tx *gorm.DB) error) {
	err := sf.New(context.Background()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL session_replication_role = replica").Error; err != nil {
			return err
		}
		return fn(tx)
	})
	Expect(err).NotTo(HaveOccurred())
}

func TestAuditRESTRequests(t *testing.T) {
	h, client := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)
	username := strings.ToLower(account.Username)

	created, resp, err := client.DefaultAPI.CreateDinosaur(ctx).Dinosaur(openapi.Dinosaur{Species: "audited"}).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))

	// An allowed read is not recorded by default.
	_, _, err = client.DefaultAPI.GetDinosaur(ctx, *created.Id).Execute()
	Expect(err).NotTo(HaveOccurred())

	// A failed mutation rolls back its success event and records a failure.
	_, resp, err = client.DefaultAPI.UpdateDinosaur(ctx, "missing").DinosaurPatchRequest(openapi.DinosaurPatchRequest{}).Execute()
	Expect(err).To(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	// An unauthenticated request is recorded.
	_, _, err = client.DefaultAPI.GetDinosaur(context.Background(), *created.Id).Execute()
	Expect(err).To(HaveOccurred())

	// The request with the credentials leaves none of them in the trail.
	jwtToken := ctx.Value(openapi.ContextAccessToken).(string)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+jwtToken).
		SetBody(`{"species":"secret-species"}`).
		Post(h.RestURL("/dinosaurs?apikey=query-secret"))
	Expect(err).NotTo(HaveOccurred())
	Expect(restyResp.StatusCode()).To(Equal(http.StatusCreated))

	rows := auditRows(sf)
	Expect(rows).To(HaveLen(4), "events: %+v", rows)

	Expect(rows[0].Outcome).To(Equal(audit.OutcomeSuccess))
	Expect(rows[0].Action).To(Equal("create"))
	Expect(rows[0].ActorType).To(Equal(audit.ActorUser))
	Expect(rows[0].ActorUsername).To(Equal(username))
	Expect(rows[0].ResourceKind).To(Equal("dinosaurs"))
	Expect(rows[0].Transport).To(Equal(audit.TransportHTTP))
	Expect(rows[0].OperationID).NotTo(BeEmpty())

	Expect(rows[1].Outcome).To(Equal(audit.OutcomeFailure))
	Expect(rows[1].Action).To(Equal("patch"))
	Expect(rows[1].ResourceID).To(Equal("missing"))
	Expect(rows[1].HTTPStatus).To(Equal(http.StatusNotFound))

	Expect(rows[2].Outcome).To(Equal(audit.OutcomeUnauthenticated))
	Expect(rows[2].ActorType).To(Equal(audit.ActorAnonymous))
	Expect(rows[2].HTTPStatus).To(Equal(http.StatusUnauthorized))

	Expect(rows[3].Outcome).To(Equal(audit.OutcomeSuccess))
	var raw []map[string]any
	Expect(sf.New(context.Background()).Table(audit.TableName).Find(&raw).Error).To(Succeed())
	dump, _ := json.Marshal(raw)
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
	sf := h.Env().Database.SessionFactory
	account := h.NewRandAccount()

	conn, err := grpc.NewClient(h.GRPCAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(bearer(h.CreateJWTString(account))))
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	client := pb.NewDinosaurServiceClient(conn)

	created, err := client.CreateDinosaur(context.Background(), &pb.CreateDinosaurRequest{Species: "grpc-audited"})
	Expect(err).NotTo(HaveOccurred())
	_, err = client.UpdateDinosaur(context.Background(), &pb.UpdateDinosaurRequest{Id: "missing"})
	Expect(err).To(HaveOccurred())

	rows := auditRows(sf)
	Expect(rows).To(HaveLen(2), "events: %+v", rows)
	Expect(rows[0].Transport).To(Equal(audit.TransportGRPC))
	Expect(rows[0].Outcome).To(Equal(audit.OutcomeSuccess))
	Expect(rows[0].Action).To(Equal("create"))
	Expect(rows[0].GRPCMethod).To(HaveSuffix("/CreateDinosaur"))
	Expect(rows[0].ActorUsername).To(Equal(strings.ToLower(account.Username)))
	Expect(created.GetMetadata().GetId()).NotTo(BeEmpty())
	Expect(rows[1].Outcome).To(Equal(audit.OutcomeFailure))
	Expect(rows[1].GRPCCode).NotTo(BeEmpty())
}

func TestAuditRecordFollowsRequestTransaction(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	s := newAuditService(sf, nil, nil)

	ctx, err := db.NewContext(context.Background(), sf)
	Expect(err).NotTo(HaveOccurred())
	Expect(s.Record(ctx, audit.Event{Action: "approve", Outcome: audit.OutcomeSuccess})).To(Succeed())
	Expect(auditRows(sf)).To(BeEmpty(), "event must not be visible before commit")
	db.MarkForRollback(ctx, fmt.Errorf("test"))
	db.Resolve(ctx)
	Expect(auditRows(sf)).To(BeEmpty(), "event must roll back with the request")

	ctx, err = db.NewContext(context.Background(), sf)
	Expect(err).NotTo(HaveOccurred())
	Expect(s.Record(ctx, audit.Event{Action: "approve", Outcome: audit.OutcomeSuccess})).To(Succeed())
	db.Resolve(ctx)
	rows := auditRows(sf)
	Expect(rows).To(HaveLen(1))
	Expect(rows[0].ActorType).To(Equal(audit.ActorSystem))
	Expect(rows[0].Transport).To(Equal(audit.TransportInternal))
}

func TestAuditTableIsAppendOnly(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	s := newAuditService(sf, nil, nil)
	Expect(s.Record(context.Background(), audit.Event{Action: "delete", Outcome: audit.OutcomeSuccess})).To(Succeed())

	g := sf.New(context.Background())
	Expect(g.Exec("UPDATE trex_audit_events SET outcome = 'failure'").Error).To(MatchError(ContainSubstring("only be updated to seal")))
	Expect(g.Exec("DELETE FROM trex_audit_events").Error).To(MatchError(ContainSubstring("append-only")))
	Expect(g.Exec("TRUNCATE trex_audit_events").Error).To(MatchError(ContainSubstring("append-only")))

	// Sealing may not smuggle a content change in with the seal columns.
	Expect(g.Exec(`UPDATE trex_audit_events SET chain_seq = 1, prev_hash = repeat('0', 64), hash = 'h', key_id = 'none',
		sealed_at = now(), outcome = 'failure'`).Error).To(MatchError(ContainSubstring("immutable")))

	_, err := s.SealOnce(context.Background())
	Expect(err).NotTo(HaveOccurred())
	Expect(g.Exec("UPDATE trex_audit_events SET hash = 'forged'").Error).To(MatchError(ContainSubstring("sealed audit events are immutable")))
	Expect(auditRows(sf)).To(HaveLen(1))
}

func TestAuditSealExportVerify(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	key := []byte(strings.Repeat("s", 32))
	var sink bytes.Buffer
	s := newAuditService(sf, key, &sink)

	for i := 0; i < 5; i++ {
		Expect(s.Record(context.Background(), audit.Event{
			Action: "delete", Outcome: audit.OutcomeSuccess,
			Resource: audit.Resource{Kind: "dinosaurs", ID: fmt.Sprintf("d-%d", i)},
		})).To(Succeed())
	}

	res, err := s.SealOnce(context.Background())
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Acquired).To(BeTrue())
	Expect(res.Sealed).To(Equal(5))
	Expect(res.Exported).To(Equal(5))
	Expect(res.Unsealed).To(BeZero())

	// Export: one JSON line per event, in chain order, with the seal.
	var lines []audit.Line
	scanner := bufio.NewScanner(&sink)
	for scanner.Scan() {
		var l audit.Line
		Expect(json.Unmarshal(scanner.Bytes(), &l)).To(Succeed())
		lines = append(lines, l)
	}
	Expect(lines).To(HaveLen(5))
	prev := audit.GenesisHash
	for i, l := range lines {
		Expect(l.SchemaVersion).To(Equal(audit.SchemaVersion))
		Expect(l.ChainSeq).To(Equal(int64(i + 1)))
		Expect(l.PrevHash).To(Equal(prev))
		Expect(l.KeyID).To(Equal(audit.KeyID(key)))
		want, err := audit.ComputeHash(key, prev, &l)
		Expect(err).NotTo(HaveOccurred())
		Expect(l.Hash).To(Equal(want), "a SIEM must be able to verify the chain from the lines alone")
		prev = l.Hash
	}

	// A second pass exports nothing new.
	res, err = s.SealOnce(context.Background())
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Sealed).To(BeZero())
	Expect(res.Exported).To(BeZero())

	ctx := context.Background()
	ok, err := s.Verify(ctx, audit.VerifyOptions{ExpectHead: fmt.Sprintf("5:%s", lines[4].Hash)})
	Expect(err).NotTo(HaveOccurred())
	Expect(ok.Problems).To(BeEmpty())
	Expect(ok.Checked).To(Equal(int64(5)))
	Expect(ok.HeadSeq).To(Equal(int64(5)))

	// Wrong key.
	wrong, err := newAuditService(sf, []byte(strings.Repeat("w", 32)), nil).Verify(ctx, audit.VerifyOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.Join(wrong.Problems, "\n")).To(ContainSubstring("key_id"))

	// Truncated tail: the SIEM holds an event the database no longer has.
	asSuperuser(sf, func(tx *gorm.DB) error {
		return tx.Exec("DELETE FROM trex_audit_events WHERE chain_seq = 5").Error
	})
	tail, err := s.Verify(ctx, audit.VerifyOptions{ExpectHead: fmt.Sprintf("5:%s", lines[4].Hash)})
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.Join(tail.Problems, "\n")).To(ContainSubstring("truncated tail"))

	// An edited event.
	asSuperuser(sf, func(tx *gorm.DB) error {
		return tx.Exec("UPDATE trex_audit_events SET actor_username = 'mallory' WHERE chain_seq = 2").Error
	})
	edited, err := s.Verify(ctx, audit.VerifyOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.Join(edited.Problems, "\n")).To(ContainSubstring("chain_seq 2: hash mismatch"))

	// A deleted event in the middle.
	asSuperuser(sf, func(tx *gorm.DB) error {
		return tx.Exec("DELETE FROM trex_audit_events WHERE chain_seq = 3").Error
	})
	gap, err := s.Verify(ctx, audit.VerifyOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.Join(gap.Problems, "\n")).To(ContainSubstring("gap: chain_seq 3"))
}

func TestAuditConcurrentSealing(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	replicas := []*audit.Service{newAuditService(sf, nil, nil), newAuditService(sf, nil, nil)}

	const writers, perWriter = 4, 25
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				Expect(replicas[0].Record(context.Background(), audit.Event{Action: "create", Outcome: audit.OutcomeSuccess})).To(Succeed())
				_, err := replicas[(w+i)%2].SealOnce(context.Background())
				Expect(err).NotTo(HaveOccurred())
			}
		}(w)
	}
	wg.Wait()
	for {
		res, err := replicas[0].SealOnce(context.Background())
		Expect(err).NotTo(HaveOccurred())
		if res.Acquired && res.Sealed == 0 {
			break
		}
	}

	res, err := replicas[1].Verify(context.Background(), audit.VerifyOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Problems).To(BeEmpty())
	Expect(res.Checked).To(Equal(int64(writers * perWriter)))
	Expect(res.Unsealed).To(BeZero())
}

func TestAuditForgedSealRejected(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	s := newAuditService(sf, nil, nil)
	Expect(s.Record(context.Background(), audit.Event{Action: "delete", Outcome: audit.OutcomeSuccess})).To(Succeed())

	g := sf.New(context.Background())
	// Hiding an event by giving it a seal outside the chain is refused.
	Expect(g.Exec(`UPDATE trex_audit_events SET chain_seq = -1, prev_hash = 'x', hash = 'x', key_id = 'x', sealed_at = now()`).Error).
		To(MatchError(ContainSubstring("next chain position")))
	Expect(g.Exec(`UPDATE trex_audit_events SET chain_seq = 1, prev_hash = 'x', hash = 'x', key_id = 'x', sealed_at = now()`).Error).
		To(MatchError(ContainSubstring("previous chain hash")))

	// A superuser who bypasses the trigger is caught by verify.
	asSuperuser(sf, func(tx *gorm.DB) error {
		return tx.Exec(`UPDATE trex_audit_events SET chain_seq = -1, prev_hash = 'x', hash = 'x', key_id = 'x', sealed_at = now()`).Error
	})
	res, err := s.Verify(context.Background(), audit.VerifyOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(strings.Join(res.Problems, "\n")).To(ContainSubstring("hidden from the chain"))
}

type failingSink struct{ fail bool }

func (f *failingSink) WriteLines([][]byte) error {
	if f.fail {
		return fmt.Errorf("sink down")
	}
	return nil
}
func (f *failingSink) Close() error { return nil }

func TestAuditSinkFailureDoesNotStopSealing(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	sink := &failingSink{fail: true}
	cfg := config.NewAuditConfig()
	s, err := audit.New(sf, cfg, audit.WithSink(sink))
	Expect(err).NotTo(HaveOccurred())
	Expect(s.Record(context.Background(), audit.Event{Action: "delete", Outcome: audit.OutcomeSuccess})).To(Succeed())

	res, err := s.SealOnce(context.Background())
	Expect(err).To(HaveOccurred(), "the export failure must be reported")
	Expect(res.Sealed).To(Equal(1), "sealing must commit even when export fails")
	rows := auditRows(sf)
	Expect(rows[0].ChainSeq).NotTo(BeNil())

	sink.fail = false
	res, err = s.SealOnce(context.Background())
	Expect(err).NotTo(HaveOccurred())
	Expect(res.Exported).To(Equal(1), "the committed seal must be exported once the sink recovers")
}

func TestAuditHostileBytesStillRecorded(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	req, err := http.NewRequest(http.MethodDelete, h.RestURL("/dinosaurs/%ff"), nil)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("User-Agent", "probe\xff")
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	rows := auditRows(sf)
	Expect(rows).To(HaveLen(1))
	Expect(rows[0].Outcome).To(Equal(audit.OutcomeUnauthenticated))
}

func TestAuditUnauthenticatedGRPCBurstDoesNotExhaustPool(t *testing.T) {
	h, _ := test.RegisterIntegration(t)
	sf := h.Env().Database.SessionFactory
	conn, err := grpc.NewClient(h.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	Expect(err).NotTo(HaveOccurred())
	defer conn.Close()
	client := pb.NewDinosaurServiceClient(conn)

	calls := h.Env().Config.Database.MaxOpenConnections*2 + 10
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.CreateDinosaur(ctx, &pb.CreateDinosaurRequest{Species: "x"})
		}()
	}
	wg.Wait()
	Expect(ctx.Err()).NotTo(HaveOccurred(), "calls must not hang waiting for database connections")
	Expect(auditRows(sf)).To(HaveLen(calls))
}
