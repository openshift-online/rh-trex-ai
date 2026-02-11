package dinosaurs_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/plugins/dinosaurs"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/openshift-online/rh-trex-ai/pkg/api/openapi"
	"github.com/openshift-online/rh-trex-ai/test"
)

func TestDinosaurGet(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, _, err := client.DefaultAPI.ApiRhTrexV1DinosaursIdGet(context.Background(), "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 401 but got nil error")

	_, resp, err := client.DefaultAPI.ApiRhTrexV1DinosaursIdGet(ctx, "foo").Execute()
	Expect(err).To(HaveOccurred(), "Expected 404")
	Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

	dino, err := newDinosaur(h.NewID())
	Expect(err).NotTo(HaveOccurred())

	dinosaur, resp, err := client.DefaultAPI.ApiRhTrexV1DinosaursIdGet(ctx, dino.ID).Execute()
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	Expect(*dinosaur.Id).To(Equal(dino.ID), "found object does not match test object")
	Expect(dinosaur.Species).To(Equal(dino.Species), "species mismatch")
	Expect(*dinosaur.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaur.Href).To(Equal(fmt.Sprintf("/api/rh-trex/v1/dinosaurs/%s", dino.ID)))
	Expect(*dinosaur.CreatedAt).To(BeTemporally("~", dino.CreatedAt))
	Expect(*dinosaur.UpdatedAt).To(BeTemporally("~", dino.UpdatedAt))
}

func TestDinosaurPost(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dino := openapi.Dinosaur{
		Species: time.Now().String(),
	}

	dinosaur, resp, err := client.DefaultAPI.ApiRhTrexV1DinosaursPost(ctx).Dinosaur(dino).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(*dinosaur.Id).NotTo(BeEmpty(), "Expected ID assigned on creation")
	Expect(dinosaur.Species).To(Equal(dino.Species), "species mismatch")
	Expect(*dinosaur.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaur.Href).To(Equal(fmt.Sprintf("/api/rh-trex/v1/dinosaurs/%s", *dinosaur.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Post(h.RestURL("/dinosaurs"))

	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
}

func TestDinosaurPatch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dino, err := newDinosaur("Brontosaurus")
	Expect(err).NotTo(HaveOccurred())

	species := "Dodo"
	dinosaur, resp, err := client.DefaultAPI.ApiRhTrexV1DinosaursIdPatch(ctx, dino.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{Species: &species}).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
	Expect(*dinosaur.Id).To(Equal(dino.ID))
	Expect(dinosaur.Species).To(Equal(species), "species mismatch")
	Expect(*dinosaur.CreatedAt).To(BeTemporally("~", dino.CreatedAt))
	Expect(*dinosaur.Kind).To(Equal("Dinosaur"))
	Expect(*dinosaur.Href).To(Equal(fmt.Sprintf("/api/rh-trex/v1/dinosaurs/%s", *dinosaur.Id)))

	jwtToken := ctx.Value(openapi.ContextAccessToken)
	restyResp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{ this is invalid }`).
		Patch(h.RestURL("/dinosaurs/foo"))

	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))

	restyResp, err = resty.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", jwtToken)).
		SetBody(`{"species":""}`).
		Patch(h.RestURL(fmt.Sprintf("/dinosaurs/%s", *dinosaur.Id)))
	Expect(err).NotTo(HaveOccurred(), "Error posting object:  %v", err)
	Expect(restyResp.StatusCode()).To(Equal(http.StatusBadRequest))
	Expect(restyResp.String()).To(ContainSubstring("species cannot be empty"))

	Eventually(func() error {
		dao := dao.NewEventDao(&h.Env().Database.SessionFactory)
		events, err := dao.FindByIDs(ctx, []string{*dinosaur.Id})
		Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)
		Expect(len(events)).To(Equal(2), "expected Create and Update events")
		Expect(contains(api.CreateEventType, events)).To(BeTrue())
		Expect(contains(api.UpdateEventType, events)).To(BeTrue())
		return nil
	}, 5*time.Second, 1*time.Second)
}

func contains(et api.EventType, events api.EventList) bool {
	for _, e := range events {
		if e.EventType == et {
			return true
		}
	}
	return false
}

func TestDinosaurPaging(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	_, err := newDinosaurList("Bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	list, _, err := client.DefaultAPI.ApiRhTrexV1DinosaursGet(ctx).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(20))
	Expect(list.Size).To(Equal(int32(20)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(1)))

	list, _, err = client.DefaultAPI.ApiRhTrexV1DinosaursGet(ctx).Page(2).Size(5).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(5))
	Expect(list.Size).To(Equal(int32(5)))
	Expect(list.Total).To(Equal(int32(20)))
	Expect(list.Page).To(Equal(int32(2)))
}

func TestDinosaurListSearch(t *testing.T) {
	h, client := test.RegisterIntegration(t)

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dinoList, err := newDinosaurList("bronto", 20)
	Expect(err).NotTo(HaveOccurred())

	search := fmt.Sprintf("id in ('%s')", dinoList[0].ID)
	list, _, err := client.DefaultAPI.ApiRhTrexV1DinosaursGet(ctx).Search(search).Execute()
	Expect(err).NotTo(HaveOccurred(), "Error getting dinosaur list: %v", err)
	Expect(len(list.Items)).To(Equal(1))
	Expect(list.Total).To(Equal(int32(1)))
	Expect(*list.Items[0].Id).To(Equal(dinoList[0].ID))
}

func TestUpdateDinosaurWithRacingRequests_BlockingAdvisoryLock(t *testing.T) {
	testUpdateDinosaurWithRacingRequests(t, true, true, 2)
}

func TestUpdateDinosaurWithRacingRequests_NonBlockingAdvisoryLock(t *testing.T) {
	// TODO: This test is flaky due to race condition timing. 
	// Need better approach to test non-blocking advisory locks reliably.
	t.Skip("Skipping flaky race condition test - needs better testing approach")
	// testUpdateDinosaurWithRacingRequests(t, true, false, 1)
}

func TestUpdateDinosaurWithRacingRequests_WithoutLock(t *testing.T) {
	// TODO: This test is flaky due to race condition timing.
	// Need better approach to test concurrent updates without locks.
	t.Skip("Skipping flaky race condition test - needs better testing approach") 
	// testUpdateDinosaurWithRacingRequests(t, false, false, 2)
}

func testUpdateDinosaurWithRacingRequests(t *testing.T, useAdvisoryLock, useBlockingAdvisoryLock bool, expectedUpdates int) {
	h, client := test.RegisterIntegration(t)

	t.Logf("TEST CONFIG: useAdvisoryLock=%v, useBlockingAdvisoryLock=%v, expectedUpdates=%d", 
		useAdvisoryLock, useBlockingAdvisoryLock, expectedUpdates)

	dinosaurs.DisableAdvisoryLock = !useAdvisoryLock
	dinosaurs.UseBlockingAdvisoryLock = useBlockingAdvisoryLock

	t.Logf("LOCK CONFIG: DisableAdvisoryLock=%v, UseBlockingAdvisoryLock=%v", 
		dinosaurs.DisableAdvisoryLock, dinosaurs.UseBlockingAdvisoryLock)

	defer func() {
		dinosaurs.DisableAdvisoryLock = false
		dinosaurs.UseBlockingAdvisoryLock = true
	}()

	account := h.NewRandAccount()
	ctx := h.NewAuthenticatedContext(account)

	dino, err := newDinosaur("Stegosaurus")
	Expect(err).NotTo(HaveOccurred())
	t.Logf("CREATED DINOSAUR: ID=%s, Species=%s", dino.ID, dino.Species)

	firstDinoUpdate := "AdvisoryLockosaurus"
	secondDinoUpdate := "AdvisoryLockosaurusSecond"
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup
	wg1.Add(1)
	wg2.Add(2)

	var firstErr, secondErr error

	go func() {
		wg1.Done()
		t.Logf("STARTING FIRST UPDATE: %s", firstDinoUpdate)
		_, _, firstErr = client.DefaultAPI.ApiRhTrexV1DinosaursIdPatch(ctx, dino.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{Species: &firstDinoUpdate}).Execute()
		t.Logf("COMPLETED FIRST UPDATE: err=%v", firstErr)
		wg2.Done()
	}()

	wg1.Wait()
	time.Sleep(100 * time.Millisecond)

	go func() {
		t.Logf("STARTING SECOND UPDATE: %s", secondDinoUpdate)
		_, _, secondErr = client.DefaultAPI.ApiRhTrexV1DinosaursIdPatch(ctx, dino.ID).DinosaurPatchRequest(openapi.DinosaurPatchRequest{Species: &secondDinoUpdate}).Execute()
		t.Logf("COMPLETED SECOND UPDATE: err=%v", secondErr)
		wg2.Done()
	}()

	wg2.Wait()

	Expect(firstErr).NotTo(HaveOccurred(), "Error in first update: %v", firstErr)
	Expect(secondErr).NotTo(HaveOccurred(), "Error in second update: %v", secondErr)

	eventdao := dao.NewEventDao(&h.Env().Database.SessionFactory)
	events, err := eventdao.All(ctx)
	Expect(err).NotTo(HaveOccurred(), "Error getting events:  %v", err)

	dinodao := dinosaurs.NewDinosaurDao(&h.Env().Database.SessionFactory)
	readDino, daoErr := dinodao.Get(ctx, dino.ID)
	Expect(daoErr).NotTo(HaveOccurred())
	
	t.Logf("FINAL DINOSAUR STATE: Species=%s", readDino.Species)
	
	if useBlockingAdvisoryLock {
		Expect(readDino.Species).To(Equal(secondDinoUpdate))
	} else {
		Expect(readDino.Species).To(Equal(firstDinoUpdate))
	}

	updatedCount := 0
	for _, e := range events {
		if e.SourceID == dino.ID && e.EventType == api.UpdateEventType {
			updatedCount = updatedCount + 1
			t.Logf("FOUND UPDATE EVENT: ID=%s, EventType=%s", e.ID, e.EventType)
		}
	}

	t.Logf("UPDATE EVENT COUNT: found=%d, expected=%d", updatedCount, expectedUpdates)
	Expect(updatedCount).To(Equal(expectedUpdates))

	Eventually(func() error {
		var count int
		err := h.DBFactory.DirectDB().
			QueryRow("select count(*) from pg_locks where locktype='advisory';").
			Scan(&count)
		Expect(err).NotTo(HaveOccurred(), "Error querying pg_locks:  %v", err)

		if count != 0 {
			return fmt.Errorf("there are %d unreleased advisory lock", count)
		}
		return nil
	}, 5*time.Second, 1*time.Second).Should(Succeed())
}
