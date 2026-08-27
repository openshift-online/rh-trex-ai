package tui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExactRequestAndSecurity(t *testing.T) {
	requests := make(chan *http.Request, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request.Clone(request.Context())
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"ok":true}`)
	}))
	defer server.Close()
	descriptor := Descriptor{
		Servers:         []Server{{URL: server.URL}},
		SecuritySchemes: []SecurityScheme{{Name: "Bearer", Type: "http", Scheme: "bearer"}},
	}
	client, err := NewClient(descriptor, ClientConfig{BaseURL: server.URL, Token: "top-secret"})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "archiveChild", Method: http.MethodPost,
		PathParts: []PathPart{{Literal: "/parents/"}, {Parameter: "parent_id"}, {Literal: "/children/"}, {Parameter: "child_id"}, {Literal: ":archive"}},
		Parameters: []Parameter{
			{Name: "parent_id", In: "path", Required: true, Style: "simple", Type: "string"},
			{Name: "child_id", In: "path", Required: true, Style: "simple", Type: "string", Pattern: `^[A-Za-z0-9/ -]+$`},
			{Name: "notify", In: "query", Required: true, Style: "form", Explode: true, Type: "boolean"},
			{Name: "X-Reason", In: "header", Required: true, Style: "simple", Type: "string"},
		},
		RequestBody: &RequestBody{Required: true, ContentType: "application/json"},
		Response:    ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"},
		Security: EffectiveSecurity{Requirements: []SecurityAlternative{{Schemes: []string{"Bearer"}}}},
	}
	result, err := client.Execute(context.Background(), operation, RequestInput{
		Values: map[string]any{"parent_id": "parent one", "child_id": "child/7", "notify": true, "X-Reason": "test"},
		Body:   []byte(`{"name":"updated"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := <-requests
	if request.Method != http.MethodPost || request.URL.EscapedPath() != "/parents/parent%20one/children/child%2F7:archive" {
		t.Fatalf("request = %s %s", request.Method, request.URL.EscapedPath())
	}
	if request.URL.Query().Get("notify") != "true" || request.Header.Get("X-Reason") != "test" {
		t.Fatalf("query/header lost: %s %#v", request.URL.RawQuery, request.Header)
	}
	if request.Header.Get("Authorization") != "Bearer top-secret" || request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("request headers = %#v", request.Header)
	}
	if result.RequestURL != server.URL+request.URL.RequestURI() || result.RequestMethod != http.MethodPost || result.Status != http.StatusOK {
		t.Fatalf("captured exchange = URL %q, method %q, status %d", result.RequestURL, result.RequestMethod, result.Status)
	}

	operation.ID = "publicArchive"
	operation.Security = EffectiveSecurity{None: true}
	_, err = client.Execute(context.Background(), operation, RequestInput{
		Values: map[string]any{"parent_id": "p", "child_id": "c", "notify": false, "X-Reason": "public"}, Body: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if authorization := (<-requests).Header.Get("Authorization"); authorization != "" {
		t.Fatalf("explicitly public request received authorization %q", authorization)
	}
}

func TestRequestValidationMakesNoRequest(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{ID: "get", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/items/"}, {Parameter: "id"}}, Parameters: []Parameter{{Name: "id", In: "path", Required: true, Type: "integer"}}, SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{None: true}}
	if _, err := client.Execute(context.Background(), operation, RequestInput{Values: map[string]any{"id": "not-an-integer"}}); err == nil || !strings.Contains(err.Error(), "must be an integer") {
		t.Fatalf("validation error = %v", err)
	}
	if called {
		t.Fatal("invalid input reached the server")
	}
}

func TestCrossOriginCredentialRefused(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer origin.Close()
	override := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("credential was transmitted") }))
	defer override.Close()
	descriptor := Descriptor{Servers: []Server{{URL: origin.URL}}}
	client, err := NewClient(descriptor, ClientConfig{BaseURL: origin.URL, Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{ID: "crossOrigin", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/private"}}, Servers: []Server{{URL: override.URL}}, SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{Requirements: []SecurityAlternative{{Schemes: []string{"Bearer"}}}}}
	if _, err := client.Execute(context.Background(), operation, RequestInput{}); err == nil || !strings.Contains(err.Error(), "untrusted credential origin") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("cross-origin error = %v", err)
	}
}

func TestOptionalBearerSecurityAllowsAnonymousRequest(t *testing.T) {
	authorizations := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorizations <- request.Header.Get("Authorization")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()
	operation := Operation{
		ID: "optional", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/optional"}},
		Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"},
		Security: EffectiveSecurity{Requirements: []SecurityAlternative{{Schemes: []string{}}, {Schemes: []string{"Bearer"}}}},
	}
	for _, testCase := range []struct {
		name, token, wantAuthorization string
	}{
		{name: "anonymous"},
		{name: "authenticated", token: "secret", wantAuthorization: "Bearer secret"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL, Token: testCase.token})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Execute(context.Background(), operation, RequestInput{}); err != nil {
				t.Fatal(err)
			}
			if authorization := <-authorizations; authorization != testCase.wantAuthorization {
				t.Fatalf("authorization = %q, want %q", authorization, testCase.wantAuthorization)
			}
		})
	}
}

func TestRequiredBearerWithoutTokenDefersAuthenticationToServer(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Fatalf("anonymous request authorization = %q", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"items":[]}`)
	}))
	defer server.Close()

	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "listScientists", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/scientists"}},
		Response: ResponseShape{ContentType: "application/json", ItemsPointer: "/items"}, SuccessStatuses: []string{"200"},
		Security: EffectiveSecurity{Requirements: []SecurityAlternative{{Schemes: []string{"Bearer"}}}},
	}
	if _, err := client.Execute(context.Background(), operation, RequestInput{}); err != nil {
		t.Fatalf("server-authorized anonymous request failed locally: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}

	authenticatedServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if authorization := request.Header.Get("Authorization"); authorization != "" {
			t.Fatalf("rejected anonymous request authorization = %q", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(writer, `{"kind":"Error","reason":"Authentication required","code":"test-401"}`)
	}))
	defer authenticatedServer.Close()
	authenticatedClient, err := NewClient(Descriptor{Servers: []Server{{URL: authenticatedServer.URL}}}, ClientConfig{BaseURL: authenticatedServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = authenticatedClient.Execute(context.Background(), operation, RequestInput{})
	apiError, ok := err.(*APIError)
	if !ok || apiError.Status != http.StatusUnauthorized || apiError.Reason != "Authentication required" {
		t.Fatalf("authentication server error = %#v (%T), want retained 401 API error", err, err)
	}
}

func TestStreamDoesNotInheritFiniteRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		flusher := writer.(http.Flusher)
		_, _ = io.WriteString(writer, "first\n")
		flusher.Flush()
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(writer, "second\n")
	}))
	defer server.Close()
	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.OpenStream(context.Background(), Operation{
		ID: "watch", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/watch"}},
		SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{None: true},
	}, RequestInput{})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || string(data) != "first\nsecond\n" {
		t.Fatalf("stream body = %q, error %v", data, err)
	}
}

func TestClientRejectsMissingOrInvalidRequiredBodyBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "updateThing", Method: http.MethodPatch, PathParts: []PathPart{{Literal: "/things"}},
		RequestBody: &RequestBody{Required: true, ContentType: "application/json", Fields: []InputField{{Name: "name", Type: "string", Required: true}}},
		Response:    ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{None: true},
	}
	for _, testCase := range []struct {
		name string
		body []byte
		want string
	}{
		{name: "missing body", want: "requires a request body"},
		{name: "malformed JSON", body: []byte(`{"name":`), want: "invalid JSON"},
		{name: "missing field", body: []byte(`{}`), want: "requires field name"},
		{name: "wrong field type", body: []byte(`{"name":7}`), want: "field name must be string"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, executeErr := client.Execute(context.Background(), operation, RequestInput{Body: testCase.body})
			if executeErr == nil || !strings.Contains(executeErr.Error(), testCase.want) {
				t.Fatalf("error = %v, want text %q", executeErr, testCase.want)
			}
		})
	}
	if requests != 0 {
		t.Fatalf("invalid bodies made %d HTTP requests", requests)
	}
	if _, err := client.Execute(context.Background(), operation, RequestInput{Body: []byte(`{"name":"safe"}`)}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("valid body made %d requests, want 1", requests)
	}
}

func TestExplicitAndInheritedBindingsConstructExactMultiplyScopedRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.URL.EscapedPath() != "/organizations/org%2F1/projects/project%202/agents/agent%2F7/inbox" {
			t.Errorf("request = %s %s", request.Method, request.URL.EscapedPath())
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()
	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "getInbox", Method: http.MethodGet,
		PathParts: []PathPart{
			{Literal: "/organizations/"}, {Parameter: "organization_id"},
			{Literal: "/projects/"}, {Parameter: "project_id"},
			{Literal: "/agents/"}, {Parameter: "agent_id"}, {Literal: "/inbox"},
		},
		Parameters: []Parameter{
			{Name: "organization_id", In: "path", Required: true, Type: "string"},
			{Name: "project_id", In: "path", Required: true, Type: "string"},
			{Name: "agent_id", In: "path", Required: true, Type: "string"},
		},
		Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{None: true},
	}
	frame := Frame{Bindings: map[string]any{"organization_id": "org/1", "project_id": "project 2"}}
	row := Row{Raw: map[string]any{"id": "agent/7"}}
	edge := Edge{Name: "inbox", Bindings: []Binding{
		{Target: "organization_id", SourceKind: "runtime-expression", Source: "$request.path.organization_id"},
		{Target: "project_id", SourceKind: "frame-path", Source: "project_id"},
		{Target: "agent_id", SourceKind: "runtime-expression", Source: "$response.body#/id"},
	}}
	values, err := evaluateBindings(edge, frame, row)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Execute(context.Background(), operation, RequestInput{Values: values}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}

	edge.Bindings[2].Source = "$response.body#/missing"
	if _, err := evaluateBindings(edge, frame, row); err == nil {
		t.Fatal("missing explicit binding unexpectedly resolved")
	}
	if requests != 1 {
		t.Fatalf("missing binding made an HTTP request: %d", requests)
	}
}

func TestSuccessStatusRange(t *testing.T) {
	if !acceptsStatus([]string{"2XX"}, http.StatusNoContent) || acceptsStatus([]string{"2XX"}, http.StatusBadRequest) {
		t.Fatal("OpenAPI response status range matching is incorrect")
	}
}

func TestPathStylesAndAllowReservedConstructExactRequest(t *testing.T) {
	requests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request.URL.EscapedPath() + "?" + request.URL.RawQuery
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()
	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "styledRequest", Method: http.MethodGet,
		PathParts: []PathPart{{Literal: "/things/"}, {Parameter: "colors"}, {Literal: "/"}, {Parameter: "attributes"}},
		Parameters: []Parameter{
			{Name: "colors", In: "path", Required: true, Style: "label", Explode: true, Type: "array"},
			{Name: "attributes", In: "path", Required: true, Style: "matrix", Explode: true, Type: "object"},
			{Name: "raw", In: "query", Style: "form", Type: "string", AllowReserved: true},
			{Name: "encoded", In: "query", Style: "form", Type: "string"},
		},
		Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{None: true},
	}
	value := "a/b?c=d:tag,@!$'()*+;"
	_, err = client.Execute(context.Background(), operation, RequestInput{Values: map[string]any{
		"colors":     []any{"red", "blue/green"},
		"attributes": map[string]any{"role": "admin/user", "name": "A B"},
		"raw":        value, "encoded": value,
	}})
	if err != nil {
		t.Fatal(err)
	}
	request := <-requests
	want := "/things/.red.blue%2Fgreen/;name=A%20B;role=admin%2Fuser?" +
		"raw=a/b?c=d:tag,@!$'()*+;&encoded=a%2Fb%3Fc%3Dd%3Atag%2C%40%21%24%27%28%29%2A%2B%3B"
	if request != want {
		t.Fatalf("styled request = %q, want %q", request, want)
	}
}

func TestSameNamedParametersRemainLocationAware(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.EscapedPath() != "/items/path-value" || request.URL.RawQuery != "id=query-value" || request.Header.Get("id") != "header-value" {
			t.Errorf("colliding parameters serialized as path=%q query=%q header=%q", request.URL.EscapedPath(), request.URL.RawQuery, request.Header.Get("id"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{}`)
	}))
	defer server.Close()
	client, err := NewClient(Descriptor{Servers: []Server{{URL: server.URL}}}, ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	operation := Operation{
		ID: "collision", Method: http.MethodGet, PathParts: []PathPart{{Literal: "/items/"}, {Parameter: "id"}},
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Style: "simple", Type: "string"},
			{Name: "id", In: "query", Required: true, Style: "form", Type: "string"},
			{Name: "id", In: "header", Required: true, Style: "simple", Type: "string"},
		},
		Response: ResponseShape{ContentType: "application/json"}, SuccessStatuses: []string{"200"}, Security: EffectiveSecurity{None: true},
	}
	_, err = client.Execute(context.Background(), operation, RequestInput{Values: map[string]any{
		ParameterValueKey("path", "id"):   "path-value",
		ParameterValueKey("query", "id"):  "query-value",
		ParameterValueKey("header", "id"): "header-value",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
	_, err = client.Execute(context.Background(), operation, RequestInput{Values: map[string]any{"id": "ambiguous"}})
	if err == nil || !strings.Contains(err.Error(), "requires query parameter id") {
		t.Fatalf("ambiguous bare value error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("ambiguous bare value made request; requests = %d", requests)
	}
}
