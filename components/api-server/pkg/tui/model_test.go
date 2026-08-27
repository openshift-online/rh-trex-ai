package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestGeneratedRuntimeNavigationWithTeatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.EscapedPath() {
		case "/parents":
			_, _ = io.WriteString(writer, `{"items":[{"id":"parent/7","name":"Alpha\u001b]52;c;b3duZWQ=\u0007"},{"id":"parent-8","name":"Other"}]}`)
		case "/parents/parent%2F7":
			assertBearer(t, request)
			_, _ = io.WriteString(writer, `{"id":"parent/7","name":"Alpha","description":"Parent restored \u001b[31mred\u001b[0m"}`)
		case "/parents/parent%2F7/children":
			assertBearer(t, request)
			_, _ = io.WriteString(writer, `{"items":[{"id":"child/1","name":"Scoped Kid"}]}`)
		case "/parents/parent%2F7/children/child%2F1":
			assertBearer(t, request)
			_, _ = io.WriteString(writer, `{"id":"child/1","name":"Scoped Kid","description":"Child detail"}`)
		case "/accounts":
			_, _ = io.WriteString(writer, `{"items":[{"id":"account-9","name":"Account Nine"}]}`)
		case "/accounts/account-9":
			assertBearer(t, request)
			_, _ = io.WriteString(writer, `{"id":"account-9","name":"Account Nine","description":"Account restored"}`)
		case "/parents/account-9/children":
			assertBearer(t, request)
			_, _ = io.WriteString(writer, `{"items":[{"id":"account-child","name":"Account Kid"}]}`)
		case "/children":
			if request.Header.Get("Authorization") != "" {
				t.Errorf("public operation received authorization")
			}
			_, _ = io.WriteString(writer, `{"items":[{"id":"public-child","name":"Public Kid"}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	model, err := NewModel(runtimeTestDescriptor(server.URL), ClientConfig{BaseURL: server.URL, Token: "token"})
	if err != nil {
		t.Fatal(err)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(110, 32))
	t.Cleanup(func() { _ = testModel.Quit() })

	waitForTexts(t, testModel, "Resources", "Accounts", "Parents", "Public Children")
	testModel.Type(":parents")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Alpha")
	output := readOutput(t, testModel.Output())
	if bytes.Contains(output, []byte("b3duZWQ=")) || bytes.Contains(output, []byte("\x1b]52")) {
		t.Fatalf("terminal injection reached output: %q", output)
	}
	testModel.Type("/other")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Other")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForText(t, testModel, "Alpha")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "description  Parent restored red", "<parents>", "<parent[parent/7]>")

	// The item has two outgoing relationships, so Enter must open a stable chooser.
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "RELATIONSHIP", "Children", "Public Children")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Scoped Kid", "<parents>", "<parent[parent/7]>", "<children[parent/7]>")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "description  Child detail", "<child[child/1]>")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForText(t, testModel, "Scoped Kid")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForTexts(t, testModel, "description  Parent restored red", "<parents>", "<parent[parent/7]>")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForText(t, testModel, "Other")

	// Reach the same scoped child view through a different parent and prove Esc
	// restores that distinct history rather than a canonical parent.
	testModel.Type(":ac")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Account Nine")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "description  Account restored", "<accounts>", "<account[account-9]>")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Account Kid", "<accounts>", "<account[account-9]>", "<children[account-9]>")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForTexts(t, testModel, "description  Account restored", "<accounts>", "<account[account-9]>")

	if err := testModel.Quit(); err != nil {
		t.Fatal(err)
	}
	final, ok := testModel.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)
	if !ok || len(final.frames) != 3 || !final.frames[0].Catalog || final.frames[1].TargetViewID != "accounts" || final.frames[2].TargetViewID != "account" {
		t.Fatalf("multi-parent history = %#v", final)
	}
}

func TestResourceCatalogShowsOnlyTopLevelResourcesInOneColumn(t *testing.T) {
	requests := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request.URL.EscapedPath()
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[{"id":"parent-1","name":"Alpha"}]}`)
	}))
	defer server.Close()

	model, err := NewModel(runtimeTestDescriptor(server.URL), ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	catalog := resourceCatalogView()
	if len(catalog.Columns) != 1 || catalog.Columns[0].Property != "resource" {
		t.Fatalf("catalog columns = %#v", catalog.Columns)
	}
	if !catalog.FillWidth {
		t.Fatal("catalog resource column does not fill the table width")
	}
	if len(model.rows) != 3 || model.rows[0].Raw["resource"] != "Accounts" || model.rows[1].Raw["resource"] != "Parents" || model.rows[2].Raw["resource"] != "Public Children" {
		t.Fatalf("top-level catalog rows = %#v", model.rows)
	}
	initial := model.View()
	if strings.Contains(initial, "Resources(all)") || strings.Contains(initial, "SCOPE") || strings.Contains(initial, "STATUS") || strings.Contains(initial, "requires context") {
		t.Fatalf("catalog leaked API debugging metadata:\n%s", initial)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() { _ = testModel.Quit() })

	waitForTexts(t, testModel, "Resources", "Accounts", "Parents", "Public Children")
	select {
	case path := <-requests:
		t.Fatalf("catalog startup made request %q", path)
	default:
	}

	// Catalog rows sort by resource name: Accounts, Parents, Public Children.
	testModel.Send(tea.KeyMsg{Type: tea.KeyDown})
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Alpha", "<resources>", "<parents>")
	select {
	case path := <-requests:
		if path != "/parents" {
			t.Fatalf("catalog selection requested %q, want /parents", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("catalog selection made no request")
	}

	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForTexts(t, testModel, "Resources", "Accounts", "Parents", "Public Children")
	select {
	case path := <-requests:
		t.Fatalf("returning to catalog made request %q", path)
	default:
	}
}

func TestResourceCatalogIsEmptyWhenNoTopLevelCollectionExists(t *testing.T) {
	descriptor := Descriptor{
		Title: "Scoped only",
		Views: []View{{
			ID: "children", Kind: "collection", Label: "Children", ScopeParameters: []string{"parent_id"},
			OperationIDs: []string{"listChildren"}, ListOperationID: "listChildren",
		}},
		Operations: []Operation{{
			ID: "listChildren", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/parents/"}, {Parameter: "parent_id"}, {Literal: "/children"}},
			Parameters: []Parameter{{Name: "parent_id", In: "path", Required: true}}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true},
		}},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: "http://localhost:8000"})
	if err != nil {
		t.Fatal(err)
	}
	model.shell.Theme = PlainTheme()
	if !model.onCatalog() || len(model.rows) != 0 || model.Init() == nil || !strings.Contains(model.View(), "Resources") || strings.Contains(model.View(), "Resources(all)") {
		t.Fatalf("scoped-only catalog state = catalog %v, rows %#v", model.onCatalog(), model.rows)
	}
	if command := model.loadCurrent(); command != nil {
		t.Fatal("scoped-only catalog created a read command")
	}
}

func TestSelectedResourceRawJSONWithTeatest(t *testing.T) {
	requests := make(chan string, 4)
	item := map[string]any{
		"id": "dino-1", "species": "Tyrannosaurus",
		"traits": map[string]any{"active": true, "periods": []any{"Cretaceous", nil}},
		"note":   "safe\x1b]52;c;owned\a",
	}
	for index := 0; index < 12; index++ {
		item[fmt.Sprintf("field_%02d", index)] = strings.Repeat(fmt.Sprintf("value-%02d-", index), 5)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request.URL.EscapedPath()
		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode(map[string]any{"items": []any{item}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	descriptor := highlightedItemActionDescriptor(server.URL)
	for index := 0; index < 12; index++ {
		descriptor.Views[0].Columns = append(descriptor.Views[0].Columns, Column{
			Property: fmt.Sprintf("field_%02d", index), Label: fmt.Sprintf("FIELD %02d", index), Type: "string",
		})
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	model.shell.Theme = PlainTheme()
	if output := model.View(); strings.Contains(output, "<r>") {
		t.Fatalf("catalog advertised raw without an API resource:\n%s", output)
	}

	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(120, 32))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForText(t, testModel, "Resources")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Tyrannos", "<r>", "raw")
	select {
	case path := <-requests:
		if path != "/dinosaurs" {
			t.Fatalf("initial resource request = %q", path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resource selection made no request")
	}

	testModel.Type("/tyr")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "</tyr>")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForText(t, testModel, "◀ 1")
	testModel.Type("O")
	testModel.Type("r")
	waitForTexts(t, testModel, "Raw Dinosaur", `"traits": {`, `"active": true`, `"periods": [`)
	rawOutput := readOutput(t, testModel.Output())
	if bytes.Contains(rawOutput, []byte("owned")) || bytes.Contains(rawOutput, []byte("\x1b]52")) {
		t.Fatalf("terminal injection reached raw output: %q", rawOutput)
	}
	select {
	case path := <-requests:
		t.Fatalf("raw resource view made request %q", path)
	default:
	}

	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForTexts(t, testModel, "din", "</tyr>", "◀ 1")
	if err := testModel.Quit(); err != nil {
		t.Fatal(err)
	}
	final := testModel.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)
	selected := final.selectedRow()
	if final.mode != modeBrowse || final.filter != "tyr" || !final.sortDescending || selected == nil || selected.Identity != "dino-1" || len(final.frames) != 2 || final.frames[1].ColumnOffset != 1 {
		t.Fatalf("raw return changed table state: mode=%v filter=%q descending=%v selected=%#v frames=%d offset=%d", final.mode, final.filter, final.sortDescending, selected, len(final.frames), final.frames[1].ColumnOffset)
	}
}

func TestRenderRawPreservesJSONTypesAndSanitizesStrings(t *testing.T) {
	value := map[string]any{
		"name":   "safe\x1b]52;c;owned\a",
		"active": true,
		"count":  float64(7),
		"nested": []any{"value", nil, map[string]any{"state": "ready\u009bunsafe"}},
	}
	raw, err := renderRaw(value)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(raw)) || !strings.Contains(raw, "\n  \"active\": true") {
		t.Fatalf("raw output is not indented JSON:\n%s", raw)
	}
	if strings.Contains(raw, "owned") || strings.Contains(raw, "\x1b") || strings.Contains(raw, "\u009b") {
		t.Fatalf("raw output retained terminal controls: %q", raw)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["active"] != true || decoded["count"] != float64(7) {
		t.Fatalf("raw output changed scalar types: %#v", decoded)
	}
	nested, ok := decoded["nested"].([]any)
	if !ok || len(nested) != 3 || nested[1] != nil {
		t.Fatalf("raw output changed array/null structure: %#v", decoded["nested"])
	}
}

func TestStreamingIsIncrementalBoundedSanitizedAndCancelable(t *testing.T) {
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/resources":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"items":[{"id":"one","name":"One"}]}`)
		case "/events":
			writer.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := writer.(http.Flusher)
			if !ok {
				t.Error("test writer cannot flush")
				return
			}
			_, _ = io.WriteString(writer, "data: safe\x1b]52;c;bad\x07 event\n\n")
			flusher.Flush()
			<-request.Context().Done()
			close(canceled)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	descriptor := Descriptor{
		Title: "Stream test", Servers: []Server{{URL: server.URL}},
		Views: []View{{ID: "resources", Kind: "collection", Label: "Resources", IdentityProperty: "id", Columns: []Column{{Property: "name", Label: "NAME"}}, OperationIDs: []string{"listResources", "streamEvents"}, ListOperationID: "listResources"}},
		Operations: []Operation{
			{ID: "listResources", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/resources"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{ID: "streamEvents", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/events"}}, Response: ResponseShape{ContentType: "text/event-stream", Stream: true}, SuccessStatuses: []string{"200"}, Capabilities: []string{"stream"}, Security: EffectiveSecurity{None: true}},
		},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForText(t, testModel, "↑ RESOURCE")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "One")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	waitForText(t, testModel, "streamEvents")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "safe event")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("Esc did not cancel the stream request")
	}
	if err := testModel.Quit(); err != nil {
		t.Fatal(err)
	}
	final := testModel.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)
	if final.mode != modeBrowse || final.streamCancel != nil || final.streamEvents != nil {
		t.Fatalf("stream state was not cleared: %#v", final)
	}

	bounded := &Model{DetailStreamComponent: DetailStreamComponent{detail: viewport.New(80, 20), autoscroll: true}}
	for index := 0; index < maxVisibleStreamEvents+25; index++ {
		bounded.appendStreamEvent(fmt.Sprintf("event-%03d", index))
	}
	if len(bounded.streamLines) != maxVisibleStreamEvents || bounded.streamLines[0] != "event-025" {
		t.Fatalf("bounded stream window = %d, first %q", len(bounded.streamLines), bounded.streamLines[0])
	}
}

func TestGenericActionInputsExecuteDocumentedRequest(t *testing.T) {
	actionRequests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPost {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Error(err)
			}
			actionRequests <- request.URL.EscapedPath() + "?" + request.URL.RawQuery + "|" + request.Header.Get("X-Reason") + "|" + string(body)
			writer.WriteHeader(http.StatusAccepted)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[]}`)
	}))
	defer server.Close()
	descriptor := Descriptor{
		Title: "Action test", Servers: []Server{{URL: server.URL}},
		Views: []View{{ID: "things", Kind: "collection", Label: "Things", OperationIDs: []string{"listThings", "archiveThing"}, ListOperationID: "listThings"}},
		Operations: []Operation{
			{ID: "listThings", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/things"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{
				ID: "archiveThing", Method: http.MethodPost,
				PathParts: []PathPart{{Literal: "/things/"}, {Parameter: "thing_id"}, {Literal: ":archive"}},
				Parameters: []Parameter{
					{Name: "thing_id", In: "path", Required: true, Style: "simple", Type: "string"},
					{Name: "thing_id", In: "query", Style: "form", Type: "string"},
					{Name: "X-Reason", In: "header", Required: true, Style: "simple", Type: "string"},
				},
				RequestBody:     &RequestBody{Required: true, ContentType: "application/json", Fields: []InputField{{Name: "name", Type: "string", Required: true}}},
				SuccessStatuses: []string{"202"}, Capabilities: []string{"action"}, Security: EffectiveSecurity{None: true},
			},
		},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(110, 30))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForTexts(t, testModel, "Resources", "Things")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Things(all)[0]")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	waitForTexts(t, testModel, "archiveThing", "POST · 4 input(s)")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "thing_id  string  required")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "thing_id is required")
	select {
	case request := <-actionRequests:
		t.Fatalf("empty required input made request %q", request)
	default:
	}
	testModel.Type("thing/7")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "X-Reason  string  required")
	testModel.Type("operator requested")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "name      string  required")
	testModel.Type("updated")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "thing_id  string  optional")
	testModel.Type("notify")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Operation completed")
	select {
	case request := <-actionRequests:
		if request != `/things/thing%2F7:archive?thing_id=notify|operator requested|{"name":"updated"}` {
			t.Fatalf("action request = %q", request)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("action request was not received")
	}
}

func TestCollectionActionsIncludeAndBindHighlightedItemOperations(t *testing.T) {
	descriptor := highlightedItemActionDescriptor("http://localhost:8000")
	model, err := NewModel(descriptor, ClientConfig{BaseURL: "http://localhost:8000"})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "dinosaurs")
	view := descriptor.View("dinosaurs")
	model.setRows(*view, nil)
	if got := actionOperationIDs(model.operationsForCurrentView()); !reflect.DeepEqual(got, []string{"createDinosaur"}) {
		t.Fatalf("actions without a selected row = %v", got)
	}

	model.setRows(*view, []map[string]any{{"id": "dinosaur/7", "species": "Raptor"}})
	if got := actionOperationIDs(model.operationsForCurrentView()); !reflect.DeepEqual(got, []string{"createDinosaur", "deleteDinosaur", "updateDinosaur"}) {
		t.Fatalf("actions for highlighted row = %v", got)
	}
	if actions := model.localActions(*view); len(actions) != 1 || actions[0].Hotkey != "x" || actions[0].Label != "Update a dinosaur" {
		t.Fatalf("highlighted-item local actions = %#v", actions)
	}
	_, _ = model.openActions()
	if chooser := model.chooser.View(); !strings.Contains(chooser, "Create a new dinosaur") || !strings.Contains(chooser, "Delete a dinosaur") || !strings.Contains(chooser, "Update a dinosaur") {
		t.Fatalf("highlighted-item action chooser = %q; rows %#v", chooser, model.chooser.Rows())
	}
	updateIndex := -1
	for index, operation := range model.chosenOperations {
		if operation.ID == "updateDinosaur" {
			updateIndex = index
			break
		}
	}
	if updateIndex < 0 || model.chosenValues[updateIndex]["id"] != "dinosaur/7" {
		t.Fatalf("selected-item action bindings = %#v", model.chosenValues)
	}
	_, _ = model.beginActionWithValues(model.chosenOperations[updateIndex], model.chosenValues[updateIndex])
	if model.form == nil || len(model.form.fields) != 1 || model.form.fields[0].descriptor.Name != "species" || model.form.fields[0].descriptor.Location != "body" {
		t.Fatalf("pre-bound item form fields = %#v", model.form)
	}
}

func TestHighlightedItemActionUsesExactBoundPathWithTeatest(t *testing.T) {
	actionRequests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPatch {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Error(err)
			}
			actionRequests <- request.URL.EscapedPath() + "|" + string(body)
			_, _ = io.WriteString(writer, `{"id":"dinosaur/7","species":"Raptor"}`)
			return
		}
		_, _ = io.WriteString(writer, `{"items":[{"id":"dinosaur/7","species":"Tyrannosaurus"}]}`)
	}))
	defer server.Close()

	model, err := NewModel(highlightedItemActionDescriptor(server.URL), ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(110, 30))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForTexts(t, testModel, "Resources", "Dinosaur")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Tyrannosaurus")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	waitForTexts(t, testModel, "Create a new dinosaur", "Delete a dinosaur", "Update a dinosaur")
	testModel.Send(tea.KeyMsg{Type: tea.KeyDown})
	testModel.Send(tea.KeyMsg{Type: tea.KeyDown})
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "species  string  required")
	testModel.Type("Raptor")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Operation completed")
	select {
	case request := <-actionRequests:
		if request != `/dinosaurs/dinosaur%2F7|{"species":"Raptor"}` {
			t.Fatalf("selected-item action request = %q", request)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("selected-item action request was not received")
	}
}

func TestRoutineReadsRemainSilentAndPostActionRefreshPreservesSuccess(t *testing.T) {
	descriptor := Descriptor{
		Title: "Silent reads", Servers: []Server{{URL: "http://localhost:9001"}},
		Views: []View{
			{ID: "things", Kind: "collection", Label: "Things", OperationIDs: []string{"listThings", "archiveThing"}, ListOperationID: "listThings"},
			{ID: "thing", Kind: "item", Label: "Thing", OperationIDs: []string{"getThing"}, GetOperationID: "getThing"},
		},
		Operations: []Operation{
			{ID: "listThings", Method: http.MethodGet, Capabilities: []string{"list"}, Response: ResponseShape{ItemsPointer: "/items"}, Security: EffectiveSecurity{None: true}},
			{ID: "getThing", Method: http.MethodGet, Capabilities: []string{"get"}, Security: EffectiveSecurity{None: true}},
			{ID: "archiveThing", Method: http.MethodPost, Capabilities: []string{"action"}, Security: EffectiveSecurity{None: true}},
		},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: "http://localhost:9001"})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "things")
	active := &model.frames[len(model.frames)-1]
	frameID := active.ID
	listResult := operationResultMsg{
		viewID: "things", frameID: frameID, operationID: "listThings",
		result: Result{Status: http.StatusOK, Body: map[string]any{"items": []any{map[string]any{"id": "thing-1"}}}},
	}
	_, _ = model.handleResult(listResult)
	if alert, present := model.shell.Alerts.Active(); present {
		t.Fatalf("initial list created alert %#v", alert)
	}

	active.TargetViewID = "thing"
	_, _ = model.handleResult(operationResultMsg{
		viewID: "thing", frameID: frameID, operationID: "getThing",
		result: Result{Status: http.StatusOK, Body: map[string]any{"id": "thing-1"}},
	})
	if alert, present := model.shell.Alerts.Active(); present {
		t.Fatalf("detail read created alert %#v", alert)
	}

	active.TargetViewID = "things"
	listResult.background = true
	_, _ = model.handleResult(listResult)
	if alert, present := model.shell.Alerts.Active(); present {
		t.Fatalf("background refresh created alert %#v", alert)
	}

	_, _ = model.handleResult(operationResultMsg{
		viewID: "things", frameID: frameID, operationID: "archiveThing",
		result: Result{Status: http.StatusAccepted},
	})
	alert, present := model.shell.Alerts.Active()
	if !present || alert.Severity != AlertSuccess || alert.Summary != "Operation completed" {
		t.Fatalf("action success alert = %#v, present %v", alert, present)
	}
	listResult.background = false
	_, _ = model.handleResult(listResult)
	alert, present = model.shell.Alerts.Active()
	if !present || alert.Summary != "Operation completed" || strings.Contains(alert.Summary, "Loaded") {
		t.Fatalf("post-action refresh replaced success alert = %#v, present %v", alert, present)
	}
}

func TestActionChooserUsesOnlyDocumentedCapabilities(t *testing.T) {
	descriptor := Descriptor{
		Views: []View{{
			ID: "record", Kind: "item", Label: "Record",
			OperationIDs: []string{"patchRecord", "streamRecordEvents"},
			Capabilities: []string{"stream", "update"},
		}},
		Operations: []Operation{
			{ID: "patchRecord", Method: http.MethodPatch, Capabilities: []string{"update"}},
			{ID: "streamRecordEvents", Method: http.MethodGet, Capabilities: []string{"stream"}},
		},
	}
	model := &Model{
		descriptor: descriptor,
		frames:     []Frame{{TargetViewID: "record", Bindings: map[string]any{}}},
	}
	_, _ = model.openActions()
	if model.mode != modeActions || len(model.chosenOperations) != 2 {
		t.Fatalf("action chooser state = mode %v, operations %#v", model.mode, model.chosenOperations)
	}
	if got := []string{model.chosenOperations[0].ID, model.chosenOperations[1].ID}; !reflect.DeepEqual(got, []string{"patchRecord", "streamRecordEvents"}) {
		t.Fatalf("action chooser controls = %v, want documented patch and stream only", got)
	}
}

func TestForegroundAPIErrorOpensScrollableSafeDialog(t *testing.T) {
	var bodyLines []string
	for index := 0; index < 40; index++ {
		bodyLines = append(bodyLines, fmt.Sprintf("response line %02d", index))
	}
	bodyLines = append(bodyLines, "secret-token", "final response marker")
	responseData := map[string]any{
		"kind": "Error", "reason": "Unable to load things", "code": "rh-trex-ai-1", "diagnostics": bodyLines,
	}
	responseBytes, err := json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	responseBody := string(responseBytes)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("X-Debug-Secret", "response-header-secret")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(writer, responseBody)
	}))
	defer server.Close()
	descriptor := Descriptor{
		Title: "Error test", Servers: []Server{{URL: server.URL}},
		Views:      []View{{ID: "things", Kind: "collection", Label: "Things", OperationIDs: []string{"listThings"}, ListOperationID: "listThings"}},
		Operations: []Operation{{ID: "listThings", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/things"}}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}}},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL, Token: "secret-token"})
	if err != nil {
		t.Fatal(err)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(90, 25))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForTexts(t, testModel, "Resources", "Things")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Error", "Unable to load things", "rh-trex-ai-1", "Details")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRight})
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Operation: listThings", "Status: 500 Internal Server Error", "response line 00")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnd})
	waitForText(t, testModel, "final response marker")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForTexts(t, testModel, "Unable to load things", "Details")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEsc})
	waitForText(t, testModel, "alert details")
	if err := testModel.Quit(); err != nil {
		t.Fatal(err)
	}
	final := testModel.FinalModel(t, teatest.WithFinalTimeout(5*time.Second)).(*Model)
	if final.mode != modeBrowse || final.errorDialog != nil || len(final.frames) != 2 {
		t.Fatalf("error dismissal changed source state: mode=%v dialog=%#v frames=%d", final.mode, final.errorDialog, len(final.frames))
	}
	alert, present := final.shell.Alerts.Active()
	if !present || !strings.Contains(alert.Details, "final response marker") {
		t.Fatalf("full API error details were not retained: %#v", alert)
	}
	rendered := final.View() + alert.Details
	for _, forbidden := range []string{"secret-token", "response-header-secret"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("unsafe API error content %q reached presentation: %q", forbidden, rendered)
		}
	}
}

func TestFailedActionRetainsEditableFormUntilSuccessfulRetry(t *testing.T) {
	var submittedBodies []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPatch:
			body, _ := io.ReadAll(request.Body)
			submittedBodies = append(submittedBodies, string(body))
			if strings.Contains(string(body), "first") {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = io.WriteString(writer, `{"kind":"Error","reason":"Name is already used","code":"rh-trex-ai-7"}`)
				return
			}
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"items":[]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	confirmation := &Confirmation{Title: "Confirm update", Message: "Update this thing?"}
	update := Operation{
		ID: "updateThing", Method: http.MethodPatch, PathParts: []PathPart{{Literal: "/things"}},
		RequestBody:     &RequestBody{Required: true, ContentType: "application/json", Fields: []InputField{{Name: "name", Type: "string", Required: true}}},
		SuccessStatuses: []string{"204"}, Capabilities: []string{"update"}, Security: EffectiveSecurity{None: true},
		Presentation: ActionPresentation{Confirmation: confirmation},
	}
	descriptor := Descriptor{
		Title: "Retry action", Servers: []Server{{URL: server.URL}},
		Views: []View{{ID: "things", Kind: "collection", Label: "Things", OperationIDs: []string{"listThings", update.ID}, ListOperationID: "listThings"}},
		Operations: []Operation{
			{ID: "listThings", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/things"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			update,
		},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "things")
	_, _ = model.beginActionWithValues(update, nil)
	if model.mode != modeActionInput || model.form == nil || len(model.form.fields) != 1 {
		t.Fatalf("initial action form = mode %v form %#v", model.mode, model.form)
	}
	model.form.fields[0].input.SetValue("first name")
	_, command := model.handleFormKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || model.mode != modeConfirmation || model.form == nil || !model.form.inFlight {
		t.Fatalf("confirmation did not retain submitted form: mode %v form %#v command %v", model.mode, model.form, command)
	}
	_, _ = model.handleConfirmationKey(tea.KeyMsg{Type: tea.KeyTab})
	_, command = model.handleConfirmationKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("confirmed action did not create a request")
	}
	_, _ = model.handleResult(command().(operationResultMsg))
	if model.mode != modeErrorDialog || model.previousMode != modeActionInput || model.form == nil || model.form.inFlight || model.confirmation != nil {
		t.Fatalf("failed action workflow = mode %v previous %v form %#v confirmation %#v", model.mode, model.previousMode, model.form, model.confirmation)
	}
	if model.form.focus != 0 || model.form.fields[0].input.Value() != "first name" {
		t.Fatalf("failed action changed form state: focus %d value %q", model.form.focus, model.form.fields[0].input.Value())
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if model.mode != modeActionInput || model.form == nil {
		t.Fatalf("default error close did not restore editable form: mode %v form %#v", model.mode, model.form)
	}

	model.form.fields[0].input.SetValue("second name")
	_, command = model.handleFormKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || model.mode != modeConfirmation || model.confirmation == nil {
		t.Fatalf("retry did not require confirmation again: mode %v confirmation %#v command %v", model.mode, model.confirmation, command)
	}
	_, _ = model.handleConfirmationKey(tea.KeyMsg{Type: tea.KeyTab})
	_, command = model.handleConfirmationKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("confirmed retry did not create a request")
	}
	_, refresh := model.handleResult(command().(operationResultMsg))
	if model.mode != modeBrowse || model.form != nil || model.confirmation != nil || model.actionInFlight || refresh == nil {
		t.Fatalf("successful retry did not close action workflow: mode %v form %#v confirmation %#v inFlight %v refresh %v", model.mode, model.form, model.confirmation, model.actionInFlight, refresh)
	}
	if len(submittedBodies) != 2 || !strings.Contains(submittedBodies[0], "first name") || !strings.Contains(submittedBodies[1], "second name") {
		t.Fatalf("submitted bodies = %#v", submittedBodies)
	}

	directUpdate := update
	directUpdate.Presentation.Confirmation = nil
	directDescriptor := descriptor
	directDescriptor.Operations = append([]Operation(nil), descriptor.Operations...)
	directDescriptor.Operations[1] = directUpdate
	directModel, err := NewModel(directDescriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, directModel, "things")
	_, _ = directModel.beginActionWithValues(directUpdate, nil)
	directModel.form.fields[0].input.SetValue("direct first")
	_, command = directModel.handleFormKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || directModel.mode != modeActionInput || directModel.form == nil || !directModel.form.inFlight {
		t.Fatalf("unconfirmed action did not remain visibly in flight: mode %v form %#v command %v", directModel.mode, directModel.form, command)
	}
	_, _ = directModel.handleResult(command().(operationResultMsg))
	if directModel.mode != modeErrorDialog || directModel.previousMode != modeActionInput || directModel.form == nil || directModel.form.fields[0].input.Value() != "direct first" {
		t.Fatalf("unconfirmed failure did not retain form: mode %v previous %v form %#v", directModel.mode, directModel.previousMode, directModel.form)
	}
	_, _ = directModel.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	directModel.form.fields[0].input.SetValue("direct second")
	_, command = directModel.handleFormKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("unconfirmed retry did not create a request")
	}
	_, _ = directModel.handleResult(command().(operationResultMsg))
	if directModel.mode != modeBrowse || directModel.form != nil || directModel.actionInFlight {
		t.Fatalf("successful unconfirmed retry retained action workflow: mode %v form %#v inFlight %v", directModel.mode, directModel.form, directModel.actionInFlight)
	}
	if len(submittedBodies) != 4 || !strings.Contains(submittedBodies[2], "direct first") || !strings.Contains(submittedBodies[3], "direct second") {
		t.Fatalf("all submitted bodies = %#v", submittedBodies)
	}
}

func TestBreadcrumbSanitizesAPIDerivedIdentityWithTeatest(t *testing.T) {
	const injectedID = "one\x1b]52;c;breadcrumb-owned\x07"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/items" {
			_, _ = io.WriteString(writer, `{"items":[{"id":"one\u001b]52;c;breadcrumb-owned\u0007","name":"Safe"}]}`)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/items/"+injectedID) {
			_, _ = io.WriteString(writer, `{"id":"one\u001b]52;c;breadcrumb-owned\u0007","name":"Safe detail"}`)
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()
	descriptor := Descriptor{
		Title: "Breadcrumb test", Servers: []Server{{URL: server.URL}},
		Views: []View{
			{ID: "items", Kind: "collection", Label: "Items", IdentityProperty: "id", Columns: []Column{{Property: "name", Label: "NAME"}}, OperationIDs: []string{"listItems"}, ListOperationID: "listItems"},
			{ID: "item", Kind: "item", Label: "Item", IdentityProperty: "id", Columns: []Column{{Property: "name", Label: "NAME"}}, OperationIDs: []string{"getItem"}, GetOperationID: "getItem"},
		},
		Operations: []Operation{
			{ID: "listItems", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/items"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{ID: "getItem", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/items/"}, {Parameter: "item_id"}}, Parameters: []Parameter{{Name: "item_id", In: "path", Required: true, Type: "string"}}, SuccessStatuses: []string{"200"}, Capabilities: []string{"get"}, Security: EffectiveSecurity{None: true}},
		},
		Edges: []Edge{{ID: "items-item", Name: "details", SourceViewID: "items", TargetViewID: "item", TargetOperationID: "getItem", Provenance: "collection-item", Bindings: []Binding{{Target: "item_id", SourceKind: "row-property", Source: "id"}}, Navigable: true}},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(90, 25))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForTexts(t, testModel, "Resources", "Items")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Safe")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForTexts(t, testModel, "Safe detail", "<items>", "<item[one]>")
	output := readOutput(t, testModel.Output())
	if bytes.Contains(output, []byte("breadcrumb-owned")) || bytes.Contains(output, []byte("\x1b]52")) {
		t.Fatalf("unsafe breadcrumb identity reached output: %q", output)
	}
}

func TestEvaluateExplicitRuntimeBindingsAndMissingValue(t *testing.T) {
	frame := Frame{Bindings: map[string]any{"project_id": "project/1"}}
	row := Row{Raw: map[string]any{"id": "agent/7"}}
	edge := Edge{Name: "children", Bindings: []Binding{
		{Target: "project_id", SourceKind: "runtime-expression", Source: "$request.path.project_id"},
		{Target: "agent_id", SourceKind: "runtime-expression", Source: "$response.body#/id"},
		{Target: "mode", SourceKind: "literal", Source: "active"},
	}}
	bindings, err := evaluateBindings(edge, frame, row)
	if err != nil {
		t.Fatal(err)
	}
	if bindings["project_id"] != "project/1" || bindings["agent_id"] != "agent/7" || bindings["mode"] != "active" {
		t.Fatalf("bindings = %#v", bindings)
	}
	edge.Bindings[1].Source = "$response.body#/missing"
	if _, err := evaluateBindings(edge, frame, row); err == nil || !strings.Contains(err.Error(), "cannot bind agent_id") {
		t.Fatalf("missing binding error = %v", err)
	}
}

func TestEvaluateStandardLinkRuntimeExpressions(t *testing.T) {
	operation := Operation{Parameters: []Parameter{
		{Name: "id", In: "path"},
		{Name: "id", In: "query"},
		{Name: "id", In: "header"},
	}}
	frame := Frame{
		Bindings: map[string]any{"project_id": "project/1"},
		RequestValues: captureRequestValues(operation, map[string]any{
			ParameterValueKey("path", "id"):   "path-id",
			ParameterValueKey("query", "id"):  "query-id",
			ParameterValueKey("header", "id"): "header-id",
		}),
		RequestBody:   decodeRuntimeBody([]byte(`{"parent":{"id":"parent/9"}}`)),
		RequestURL:    "https://api.example.test/parents/path-id?id=query-id",
		RequestMethod: http.MethodPost,
		ResponseHeaders: http.Header{
			"Location": []string{"/children/child-3"},
		},
		ResponseBody:   map[string]any{"id": "child-3"},
		ResponseStatus: http.StatusCreated,
	}
	row := Row{Raw: map[string]any{"id": "fallback-row"}}
	edge := Edge{Name: "standard expressions", Bindings: []Binding{
		{Target: "url", SourceKind: "runtime-expression", Source: "$url"},
		{Target: "method", SourceKind: "runtime-expression", Source: "$method"},
		{Target: "status", SourceKind: "runtime-expression", Source: "$statusCode"},
		{Target: "path", SourceKind: "runtime-expression", Source: "$request.path.id"},
		{Target: "query", SourceKind: "runtime-expression", Source: "$request.query.id"},
		{Target: "request_header", SourceKind: "runtime-expression", Source: "$request.header.ID"},
		{Target: "request_body_full", SourceKind: "runtime-expression", Source: "$request.body"},
		{Target: "request_body", SourceKind: "runtime-expression", Source: "$request.body#/parent/id"},
		{Target: "response_header", SourceKind: "runtime-expression", Source: "$response.header.location"},
		{Target: "response_body_full", SourceKind: "runtime-expression", Source: "$response.body"},
		{Target: "response_body", SourceKind: "runtime-expression", Source: "$response.body#/id"},
	}}
	bindings, err := evaluateBindings(edge, frame, row)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"project_id": "project/1", "url": frame.RequestURL, "method": http.MethodPost, "status": http.StatusCreated,
		"path": "path-id", "query": "query-id", "request_header": "header-id", "request_body": "parent/9",
		"response_header": "/children/child-3", "response_body": "child-3",
		"request_body_full": frame.RequestBody, "response_body_full": frame.ResponseBody,
	}
	if !reflect.DeepEqual(bindings, want) {
		t.Fatalf("standard runtime bindings = %#v, want %#v", bindings, want)
	}
}

func TestModelScrollsColumnsWithoutChangingRowsOrFilterWidths(t *testing.T) {
	view := View{
		ID: "records", Kind: "collection", Label: "Records", IdentityProperty: "id", DefaultSort: "a",
		Columns: []Column{
			{Property: "a", Label: "A", Priority: 100, Type: "integer"},
			{Property: "b", Label: "B", Priority: 90, Type: "integer"},
			{Property: "c", Label: "C", Priority: 80, Type: "integer"},
			{Property: "d", Label: "D", Priority: 70, Type: "integer"},
			{Property: "e", Label: "E", Priority: 60, Type: "integer"},
			{Property: "f", Label: "F", Priority: 50, Type: "integer"},
		},
	}
	model := &Model{
		descriptor: Descriptor{Views: []View{view}}, width: 24, height: 20,
		frames: []Frame{{TargetViewID: view.ID, Label: view.Label, Bindings: map[string]any{}}},
	}
	model.rebuildTable(view)
	items := []map[string]any{
		{"id": "record-1", "a": 1, "b": 2, "c": 3, "d": 4, "e": 5, "f": "needle-one"},
		{"id": "record-2", "a": 2, "b": 3, "c": 4, "d": 5, "e": 6, "f": "needle-two"},
	}
	model.setRows(view, items)
	if !reflect.DeepEqual(model.displayColumns, []int{0, 1, 2}) || model.leftOverflow != 0 || model.rightOverflow != 3 {
		t.Fatalf("initial horizontal state = columns %v, left %d, right %d", model.displayColumns, model.leftOverflow, model.rightOverflow)
	}
	if output := model.tableView(); strings.Contains(output, "◀") || !strings.Contains(output, "3 ▶") {
		t.Fatalf("left-edge affordance = %q", output)
	}
	model.table.SetCursor(1)
	selected := model.selectedRow()
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	if model.frames[0].ColumnOffset != 1 || !reflect.DeepEqual(model.displayColumns, []int{1, 2, 3}) {
		t.Fatalf("scrolled state = frame %#v, columns %v", model.frames[0], model.displayColumns)
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	if model.frames[0].ColumnOffset != 2 || !reflect.DeepEqual(model.displayColumns, []int{2, 3, 4}) {
		t.Fatalf("second scrolled state = frame %#v, columns %v", model.frames[0], model.displayColumns)
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRight})
	if model.frames[0].ColumnOffset != 3 || !reflect.DeepEqual(model.displayColumns, []int{3, 4, 5}) {
		t.Fatalf("right-edge state = frame %#v, columns %v", model.frames[0], model.displayColumns)
	}
	if output := model.tableView(); !strings.Contains(output, "◀ 3") || strings.Contains(output, "▶") {
		t.Fatalf("right-edge affordance = %q", output)
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
	if model.frames[0].ColumnOffset != 2 || !reflect.DeepEqual(model.displayColumns, []int{2, 3, 4}) {
		t.Fatalf("left-scrolled state = frame %#v, columns %v", model.frames[0], model.displayColumns)
	}
	if after := model.selectedRow(); selected == nil || after == nil || after.Identity != selected.Identity {
		t.Fatalf("row selection changed during horizontal scroll: before %#v, after %#v", selected, after)
	}

	widths := append([]int(nil), model.columnWidths...)
	model.filter = "needle"
	model.applyFilter()
	if len(model.visible) != 2 || !reflect.DeepEqual(widths, model.columnWidths) {
		t.Fatalf("off-screen filter changed rows or widths: rows %d, widths %v -> %v", len(model.visible), widths, model.columnWidths)
	}
	if output := model.View(); !strings.Contains(output, "◀ 2") || !strings.Contains(output, "1 ▶") || !strings.Contains(output, columnScrollHint()) {
		t.Fatalf("overflow affordance absent from view: %q", output)
	}
	model.setRows(view, items)
	if model.frames[0].ColumnOffset != 2 || model.leftOverflow != 2 {
		t.Fatalf("refresh lost horizontal offset: frame %#v, left %d", model.frames[0], model.leftOverflow)
	}

	model.width = 50
	model.resize()
	if model.frames[0].ColumnOffset != 0 || model.leftOverflow != 0 || model.rightOverflow != 0 || len(model.displayColumns) != len(view.Columns) {
		t.Fatalf("wide resize did not clamp offset and reveal all columns: frame %#v, columns %v, left %d, right %d", model.frames[0], model.displayColumns, model.leftOverflow, model.rightOverflow)
	}
	model.table.SetCursor(1)
	selected = model.selectedRow()
	model.width = 24
	model.resize()
	if !reflect.DeepEqual(model.displayColumns, []int{0, 1, 2}) || model.rightOverflow != 3 {
		t.Fatalf("narrow resize did not restore overflow: columns %v, right %d", model.displayColumns, model.rightOverflow)
	}
	if after := model.selectedRow(); selected == nil || after == nil || after.Identity != selected.Identity {
		t.Fatalf("row selection changed during narrow resize: before %#v, after %#v", selected, after)
	}
}

func TestPopRestoresCollectionSelectionByIdentity(t *testing.T) {
	descriptor := Descriptor{
		Views: []View{
			{ID: "parents", Kind: "collection", Label: "Parents", IdentityProperty: "id", DefaultSort: "id", Columns: []Column{{Property: "id", Label: "ID"}}, ListOperationID: "listParents"},
			{ID: "children", Kind: "collection", Label: "Children", IdentityProperty: "id", Columns: []Column{{Property: "id", Label: "ID"}}},
		},
		Operations: []Operation{{ID: "listParents", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/parents"}}, Capabilities: []string{"list"}}},
	}
	model := &Model{
		descriptor: descriptor, width: 80, height: 25,
		frames: []Frame{
			{TargetViewID: "parents", Label: "Parents", Bindings: map[string]any{}},
			{TargetViewID: "children", Label: "Children", SelectedIdentity: "parent-2", Bindings: map[string]any{}},
		},
	}
	model.rebuildTable(*descriptor.View("children"))
	_, _ = model.popFrame()
	model.setRows(*descriptor.View("parents"), []map[string]any{{"id": "parent-1"}, {"id": "parent-2"}})
	selected := model.selectedRow()
	if selected == nil || selected.Identity != "parent-2" {
		t.Fatalf("restored selection = %#v", selected)
	}
}

func TestRefreshFailurePreservesContentAndLaterSuccessRestoresSelection(t *testing.T) {
	fail := false
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		if fail {
			http.Error(writer, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[{"id":"one","name":"One"},{"id":"two","name":"Two"}]}`)
	}))
	defer server.Close()
	descriptor := Descriptor{
		Title: "Refresh test", Servers: []Server{{URL: server.URL}},
		Views:      []View{{ID: "things", Kind: "collection", Label: "Things", IdentityProperty: "id", DefaultSort: "name", Columns: []Column{{Property: "name", Label: "NAME"}}, OperationIDs: []string{"listThings"}, ListOperationID: "listThings"}},
		Operations: []Operation{{ID: "listThings", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/things"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}}},
	}
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL, RefreshInterval: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "things")
	active := &model.frames[len(model.frames)-1]
	initial := model.loadCurrent()
	if initial == nil {
		t.Fatal("initial load command is nil")
	}
	_, _ = model.handleResult(initial().(operationResultMsg))
	model.table.SetCursor(1)
	selected := model.selectedRow()
	if selected == nil || selected.Identity != "two" {
		t.Fatalf("initial selection = %#v", selected)
	}

	fail = true
	refresh := model.refreshCurrent()
	if refresh == nil || !active.InFlight || !active.Refreshing {
		t.Fatalf("refresh did not enter in-flight state: %#v", active)
	}
	if duplicate := model.refreshCurrent(); duplicate != nil {
		t.Fatal("overlapping refresh command was created")
	}
	_, _ = model.handleResult(refresh().(operationResultMsg))
	if len(model.rows) != 2 || !active.Stale || active.InFlight || active.Refreshing {
		t.Fatalf("failed refresh state = frame %#v, rows %d", active, len(model.rows))
	}
	alert, present := model.shell.Alerts.Active()
	if !present || alert.Severity != AlertError || !strings.Contains(alert.Summary, "HTTP 503") {
		t.Fatalf("refresh alert = %#v, present %v", alert, present)
	}

	fail = false
	refresh = model.refreshCurrent()
	_, _ = model.handleResult(refresh().(operationResultMsg))
	selected = model.selectedRow()
	if selected == nil || selected.Identity != "two" || active.Stale || active.LastSuccess.IsZero() {
		t.Fatalf("successful refresh state = frame %#v, selection %#v", active, selected)
	}
	for _, candidate := range model.shell.Alerts.alerts {
		if candidate.Key == model.refreshAlertKey(active.ID) {
			t.Fatalf("successful retry retained refresh error %#v", candidate)
		}
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want initial plus two refreshes", requests)
	}
}

func TestInvalidSuccessfulReadShapeDoesNotRecordRefreshSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"unexpected":[]}`)
	}))
	defer server.Close()
	model, err := NewModel(runtimeTestDescriptor(server.URL), ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "parents")
	active := &model.frames[len(model.frames)-1]
	request := model.loadCurrent()
	_, _ = model.handleResult(request().(operationResultMsg))
	if !active.LoadFailed || !active.LastSuccess.IsZero() {
		t.Fatalf("invalid response shape recorded success: %#v", active)
	}
	alert, present := model.shell.Alerts.Active()
	if !present || alert.Severity != AlertError || !strings.Contains(alert.Summary, "items") {
		t.Fatalf("invalid response alert = %#v, present %v", alert, present)
	}
}

func TestPollingCanBeDisabledAndStalenessUsesConfiguredThreshold(t *testing.T) {
	descriptor := runtimeTestDescriptor("http://localhost:8000")
	model, err := NewModel(descriptor, ClientConfig{BaseURL: "http://localhost:8000", RefreshInterval: 0})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "parents")
	active := &model.frames[len(model.frames)-1]
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	active.LastSuccess = now
	model.nextRefresh = now
	_, _ = model.Update(presentationPulseMsg{now: now.Add(20 * time.Second)})
	if active.InFlight {
		t.Fatal("disabled polling started a request")
	}
	if !active.Stale {
		t.Fatal("page did not become stale after the fifteen-second floor")
	}

	model.refreshInterval = 10 * time.Second
	active.Stale = false
	model.updateStaleness(now.Add(29 * time.Second))
	if active.Stale {
		t.Fatal("page became stale before three configured intervals")
	}
	model.updateStaleness(now.Add(31 * time.Second))
	if !active.Stale {
		t.Fatal("page did not become stale after three configured intervals")
	}
}

func TestHiddenFrameResultIsIgnoredWithoutLeavingRequestStuck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[{"id":"one","name":"One"}]}`)
	}))
	defer server.Close()
	descriptor := runtimeTestDescriptor(server.URL)
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	activateResource(t, model, "parents")
	initial := model.loadCurrent()
	_, _ = model.handleResult(initial().(operationResultMsg))
	refresh := model.refreshCurrent()
	parentIndex := len(model.frames) - 1
	parentID := model.frames[parentIndex].ID
	model.frames = append(model.frames, Frame{ID: model.newFrameID(), TargetViewID: "accounts", Label: "Accounts", Bindings: map[string]any{}})
	before := len(model.rows)
	_, _ = model.handleResult(refresh().(operationResultMsg))
	if model.frames[parentIndex].InFlight || model.frames[parentIndex].Refreshing {
		t.Fatalf("hidden frame %d remained in flight: %#v", parentID, model.frames[parentIndex])
	}
	if len(model.rows) != before {
		t.Fatalf("hidden result changed active content: %d -> %d", before, len(model.rows))
	}
}

func TestModelRejectsNegativeRefreshInterval(t *testing.T) {
	_, err := NewModel(runtimeTestDescriptor("http://localhost:8000"), ClientConfig{BaseURL: "http://localhost:8000", RefreshInterval: -time.Second})
	if err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("negative refresh interval error = %v", err)
	}
}

func TestOperationHotkeyUsesSafeConfirmationAndSubmitsOnce(t *testing.T) {
	actionRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			actionRequests++
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[]}`)
	}))
	defer server.Close()
	descriptor := confirmationTestDescriptor(server.URL)
	model, err := NewModel(descriptor, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	model.shell.Theme = PlainTheme()
	activateResource(t, model, "things")
	_, command := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if command != nil || model.mode != modeConfirmation || model.confirmation == nil || model.confirmation.confirmFocus {
		t.Fatalf("hotkey confirmation state = mode %v, dialog %#v, command %v", model.mode, model.confirmation, command)
	}
	rendered := model.View()
	if !strings.Contains(rendered, "<Confirm delete>") || !strings.Contains(rendered, "[ Cancel ]") || !strings.Contains(rendered, "Delete all things?") || strings.Contains(rendered, "DESTRUCTIVE") {
		t.Fatalf("rendered confirmation is not compact and safe:\n%s", rendered)
	}
	_, command = model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || model.mode != modeBrowse || actionRequests != 0 {
		t.Fatalf("safe cancel issued request: mode %v, command %v, requests %d", model.mode, command, actionRequests)
	}

	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	_, command = model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("explicit confirmation did not create request")
	}
	_, duplicate := model.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if duplicate != nil {
		t.Fatal("repeated confirmation created a second request")
	}
	_, refresh := model.handleResult(command().(operationResultMsg))
	if refresh == nil || actionRequests != 1 {
		t.Fatalf("confirmed action = requests %d, refresh %v", actionRequests, refresh)
	}
	_, _ = model.handleResult(refresh().(operationResultMsg))
	if actionRequests != 1 || model.mode != modeBrowse {
		t.Fatalf("post-action state = requests %d, mode %v", actionRequests, model.mode)
	}
}

func TestCompactDestructiveConfirmationWithTeatest(t *testing.T) {
	requests := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodDelete {
			requests <- struct{}{}
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[]}`)
	}))
	defer server.Close()

	model, err := NewModel(confirmationTestDescriptor(server.URL), ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	model.shell.Theme = PlainTheme()
	testModel := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 30))
	t.Cleanup(func() { _ = testModel.Quit() })
	waitForTexts(t, testModel, "Resources", "Things")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Things(all)[0]")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	waitForTexts(t, testModel, "<Confirm delete>", "Delete all things?", "[ Cancel ]")
	select {
	case <-requests:
		t.Fatal("opening confirmation sent the destructive request")
	default:
	}
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Action canceled")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	waitForText(t, testModel, "[ Cancel ]")
	testModel.Send(tea.KeyMsg{Type: tea.KeyRight})
	waitForText(t, testModel, "[ Delete ]")
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	testModel.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText(t, testModel, "Operation completed")
	select {
	case <-requests:
	case <-time.After(2 * time.Second):
		t.Fatal("confirmed delete request was not received")
	}
	select {
	case <-requests:
		t.Fatal("duplicate submit sent a second delete request")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestResourcePromptCompletesOnlyCurrentlyAddressableViews(t *testing.T) {
	model, err := NewModel(runtimeTestDescriptor("http://localhost:8000"), ClientConfig{BaseURL: "http://localhost:8000"})
	if err != nil {
		t.Fatal(err)
	}
	candidates := model.resourceCommandCandidates()
	joined := strings.Join(candidates, "|")
	if !strings.Contains(joined, "Accounts") || !strings.Contains(joined, "ac") || !strings.Contains(joined, "Public Children") {
		t.Fatalf("addressable resource candidates = %q", joined)
	}
	if containsString(candidates, "Children") || containsString(candidates, "ch") {
		t.Fatalf("unbound scoped view leaked into candidates: %q", joined)
	}

	for _, keyType := range []tea.KeyType{tea.KeyTab, tea.KeyRight, tea.KeyCtrlF} {
		model.mode = modeSwitch
		model.CommandBar.Begin(CommandResource, "")
		model.CommandBar.SetSuggestions(candidates)
		_, _ = model.handleInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		if model.CommandBar.CurrentSuggestion() != "ac" {
			t.Fatalf("initial inline suggestion for %v = %q", keyType, model.CommandBar.CurrentSuggestion())
		}
		if rendered := model.CommandBar.View(PlainTheme(), 40).Input; !strings.Contains(rendered, "ac") {
			t.Fatalf("inline completion suffix absent for %v: %q", keyType, rendered)
		}
		_, _ = model.handleInputKey(tea.KeyMsg{Type: keyType})
		if model.CommandBar.Value() != "ac" {
			t.Fatalf("acceptance key %v completed %q", keyType, model.CommandBar.Value())
		}
	}

	model.mode = modeSwitch
	model.CommandBar.Begin(CommandResource, "")
	model.CommandBar.SetSuggestions(candidates)
	_, _ = model.handleInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	_, _ = model.handleInputKey(tea.KeyMsg{Type: tea.KeyUp})
	if model.CommandBar.CurrentSuggestion() != "Accounts" {
		t.Fatalf("Up did not cycle to the next deterministic suggestion: %q", model.CommandBar.CurrentSuggestion())
	}
	_, _ = model.handleInputKey(tea.KeyMsg{Type: tea.KeyDown})
	if model.CommandBar.CurrentSuggestion() != "ac" {
		t.Fatalf("Down did not cycle to the previous deterministic suggestion: %q", model.CommandBar.CurrentSuggestion())
	}
}

func TestLiveFilterPersistsInFrameTitleUntilCleared(t *testing.T) {
	model, err := NewModel(runtimeTestDescriptor("http://localhost:8000"), ClientConfig{BaseURL: "http://localhost:8000"})
	if err != nil {
		t.Fatal(err)
	}
	view := model.currentView()
	if view == nil {
		t.Fatal("missing root view")
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	_, _ = model.handleInputKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alpha")})
	live := pageFrameTitle(model.semanticPage(*view))
	if live.Filter != "alpha" {
		t.Fatalf("live filter title = %#v", live)
	}
	_, _ = model.handleInputKey(tea.KeyMsg{Type: tea.KeyEsc})
	persisted := pageFrameTitle(model.semanticPage(*view))
	if model.mode != modeCatalog || persisted.Filter != "alpha" {
		t.Fatalf("closed prompt lost active filter: mode %v, title %#v", model.mode, persisted)
	}
	_, _ = model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	cleared := pageFrameTitle(model.semanticPage(*view))
	if cleared.Filter != "" {
		t.Fatalf("cleared filter retained frame badge: %#v", cleared)
	}
}

func activateResource(t *testing.T, model *Model, viewID string) {
	t.Helper()
	view := model.descriptor.View(viewID)
	if view == nil {
		t.Fatalf("missing test resource %q", viewID)
	}
	catalog := model.catalogFrame()
	catalog.CatalogSelection = view.ID
	model.frames = []Frame{
		catalog,
		{ID: model.newFrameID(), TargetViewID: view.ID, Label: view.Label, Bindings: map[string]any{}},
	}
	model.mode = modeBrowse
	model.filter = ""
	model.rebuildTable(*view)
}

func actionOperationIDs(operations []Operation) []string {
	result := make([]string, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operation.ID)
	}
	return result
}

func confirmationTestDescriptor(server string) Descriptor {
	return Descriptor{
		Title: "Action confirmation", Servers: []Server{{URL: server}},
		Views: []View{{ID: "things", Kind: "collection", Label: "Things", OperationIDs: []string{"listThings", "deleteThings"}, ListOperationID: "listThings"}},
		Operations: []Operation{
			{ID: "listThings", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/things"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{ID: "deleteThings", Method: http.MethodDelete, PathParts: []PathPart{{Literal: "/things"}}, SuccessStatuses: []string{"204"}, Capabilities: []string{"delete"}, Security: EffectiveSecurity{None: true}, Presentation: ActionPresentation{Label: "Delete all", Hotkey: "x", Confirmation: &Confirmation{Title: "Confirm delete", Message: "Delete all things?", Destructive: true}}},
		},
	}
}

func highlightedItemActionDescriptor(server string) Descriptor {
	itemParameter := Parameter{Name: "id", In: "path", Required: true, Style: "simple", Type: "string"}
	body := &RequestBody{
		Required: true, ContentType: "application/json",
		Fields: []InputField{{Name: "species", Type: "string", Required: true}},
	}
	return Descriptor{
		Title: "Highlighted item actions", Servers: []Server{{URL: server}},
		Views: []View{
			{
				ID: "dinosaurs", Kind: "collection", Label: "Dinosaur", IdentityProperty: "id", DefaultSort: "species",
				Columns:      []Column{{Property: "species", Label: "SPECIES"}, {Property: "id", Label: "ID"}},
				OperationIDs: []string{"createDinosaur", "listDinosaurs"}, ListOperationID: "listDinosaurs",
			},
			{
				ID: "dinosaur", Kind: "item", Label: "Dinosaur", IdentityProperty: "id",
				OperationIDs: []string{"deleteDinosaur", "getDinosaur", "updateDinosaur"}, GetOperationID: "getDinosaur",
			},
		},
		Operations: []Operation{
			{ID: "createDinosaur", Method: http.MethodPost, PathParts: []PathPart{{Literal: "/dinosaurs"}}, RequestBody: body, SuccessStatuses: []string{"201"}, Capabilities: []string{"create"}, Summary: "Create a new dinosaur", Security: EffectiveSecurity{None: true}},
			{ID: "listDinosaurs", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/dinosaurs"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{ID: "getDinosaur", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/dinosaurs/"}, {Parameter: "id"}}, Parameters: []Parameter{itemParameter}, SuccessStatuses: []string{"200"}, Capabilities: []string{"get"}, Security: EffectiveSecurity{None: true}},
			{ID: "updateDinosaur", Method: http.MethodPatch, PathParts: []PathPart{{Literal: "/dinosaurs/"}, {Parameter: "id"}}, Parameters: []Parameter{itemParameter}, RequestBody: body, SuccessStatuses: []string{"200"}, Capabilities: []string{"update"}, Summary: "Update a dinosaur", Security: EffectiveSecurity{None: true}, Presentation: ActionPresentation{Hotkey: "x"}},
			{ID: "deleteDinosaur", Method: http.MethodDelete, PathParts: []PathPart{{Literal: "/dinosaurs/"}, {Parameter: "id"}}, Parameters: []Parameter{itemParameter}, SuccessStatuses: []string{"204"}, Capabilities: []string{"delete"}, Summary: "Delete a dinosaur", Security: EffectiveSecurity{None: true}, Presentation: ActionPresentation{Confirmation: &Confirmation{Title: "Confirm delete", Message: "Delete the dinosaur?", Destructive: true}}},
		},
		Edges: []Edge{{
			ID: "dinosaurs-dinosaur", Name: "details", SourceViewID: "dinosaurs", TargetViewID: "dinosaur",
			TargetOperationID: "getDinosaur", Provenance: "collection-item", Navigable: true,
			Bindings: []Binding{{Target: "id", SourceKind: "row-property", Source: "id"}},
		}},
	}
}

func runtimeTestDescriptor(server string) Descriptor {
	return Descriptor{
		Title: "Runtime test", Servers: []Server{{URL: server}},
		Views: []View{
			{ID: "parents", Kind: "collection", Label: "Parents", IdentityProperty: "id", DefaultSort: "name", Columns: []Column{{Property: "name", Label: "NAME"}, {Property: "id", Label: "ID"}}, OperationIDs: []string{"listParents"}, Capabilities: []string{"list"}, ListOperationID: "listParents"},
			{ID: "parent", Kind: "item", Label: "Parent", IdentityProperty: "id", Columns: []Column{{Property: "id", Label: "ID"}}, OperationIDs: []string{"getParent"}, Capabilities: []string{"get"}, GetOperationID: "getParent"},
			{ID: "accounts", Kind: "collection", Label: "Accounts", Aliases: []string{"ac"}, IdentityProperty: "id", DefaultSort: "name", Columns: []Column{{Property: "name", Label: "NAME"}}, OperationIDs: []string{"listAccounts"}, Capabilities: []string{"list"}, ListOperationID: "listAccounts"},
			{ID: "account", Kind: "item", Label: "Account", IdentityProperty: "id", Columns: []Column{{Property: "id", Label: "ID"}}, OperationIDs: []string{"getAccount"}, Capabilities: []string{"get"}, GetOperationID: "getAccount"},
			{ID: "children", Kind: "collection", Label: "Children", Aliases: []string{"ch"}, IdentityProperty: "id", DefaultSort: "name", Columns: []Column{{Property: "name", Label: "NAME"}}, ScopeParameters: []string{"parent_id"}, OperationIDs: []string{"listChildren"}, Capabilities: []string{"list"}, ListOperationID: "listChildren"},
			{ID: "child", Kind: "item", Label: "Child", IdentityProperty: "id", Columns: []Column{{Property: "id", Label: "ID"}}, ScopeParameters: []string{"parent_id"}, OperationIDs: []string{"getChild"}, Capabilities: []string{"get"}, GetOperationID: "getChild"},
			{ID: "public-children", Kind: "collection", Label: "Public Children", IdentityProperty: "id", Columns: []Column{{Property: "name", Label: "NAME"}}, OperationIDs: []string{"listPublicChildren"}, Capabilities: []string{"list"}, ListOperationID: "listPublicChildren"},
		},
		Operations: []Operation{
			{ID: "listParents", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/parents"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{ID: "getParent", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/parents/"}, {Parameter: "parent_id"}}, Parameters: []Parameter{{Name: "parent_id", In: "path", Required: true, Type: "string"}}, Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"get"}, Security: bearerSecurity()},
			{ID: "listAccounts", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/accounts"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
			{ID: "getAccount", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/accounts/"}, {Parameter: "account_id"}}, Parameters: []Parameter{{Name: "account_id", In: "path", Required: true, Type: "string"}}, Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"get"}, Security: bearerSecurity()},
			{ID: "listChildren", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/parents/"}, {Parameter: "parent_id"}, {Literal: "/children"}}, Parameters: []Parameter{{Name: "parent_id", In: "path", Required: true, Type: "string"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: bearerSecurity()},
			{ID: "getChild", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/parents/"}, {Parameter: "parent_id"}, {Literal: "/children/"}, {Parameter: "child_id"}}, Parameters: []Parameter{{Name: "parent_id", In: "path", Required: true, Type: "string"}, {Name: "child_id", In: "path", Required: true, Type: "string"}}, Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"get"}, Security: bearerSecurity()},
			{ID: "listPublicChildren", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/children"}}, Response: ResponseShape{ItemsPointer: "/items"}, SuccessStatuses: []string{"200"}, Capabilities: []string{"list"}, Security: EffectiveSecurity{None: true}},
		},
		Edges: []Edge{
			{ID: "parents-item", Name: "details", SourceViewID: "parents", TargetViewID: "parent", TargetOperationID: "getParent", Provenance: "collection-item", Bindings: []Binding{{Target: "parent_id", SourceKind: "row-property", Source: "id"}}, Navigable: true},
			{ID: "parent-children", Name: "children", SourceViewID: "parent", TargetViewID: "children", TargetOperationID: "listChildren", Provenance: "explicit-link", Bindings: []Binding{{Target: "parent_id", SourceKind: "runtime-expression", Source: "$response.body#/id"}}, Navigable: true},
			{ID: "parent-public", Name: "publicChildren", SourceViewID: "parent", TargetViewID: "public-children", TargetOperationID: "listPublicChildren", Provenance: "explicit-link", Navigable: true},
			{ID: "children-item", Name: "details", SourceViewID: "children", TargetViewID: "child", TargetOperationID: "getChild", Provenance: "collection-item", Bindings: []Binding{{Target: "parent_id", SourceKind: "frame-path", Source: "parent_id"}, {Target: "child_id", SourceKind: "row-property", Source: "id"}}, Navigable: true},
			{ID: "accounts-item", Name: "details", SourceViewID: "accounts", TargetViewID: "account", TargetOperationID: "getAccount", Provenance: "collection-item", Bindings: []Binding{{Target: "account_id", SourceKind: "row-property", Source: "id"}}, Navigable: true},
			{ID: "account-children", Name: "children", SourceViewID: "account", TargetViewID: "children", TargetOperationID: "listChildren", Provenance: "explicit-link", Bindings: []Binding{{Target: "parent_id", SourceKind: "runtime-expression", Source: "$response.body#/id"}}, Navigable: true},
		},
	}
}

func bearerSecurity() EffectiveSecurity {
	return EffectiveSecurity{Requirements: []SecurityAlternative{{Schemes: []string{"Bearer"}}}}
}

func assertBearer(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("missing authorization: %#v", request.Header)
	}
}

func waitForText(t *testing.T, model *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, model.Output(), func(output []byte) bool { return bytes.Contains(output, []byte(text)) }, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

func waitForTexts(t *testing.T, model *teatest.TestModel, texts ...string) {
	t.Helper()
	teatest.WaitFor(t, model.Output(), func(output []byte) bool {
		for _, text := range texts {
			if !bytes.Contains(output, []byte(text)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

func readOutput(t *testing.T, reader io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
