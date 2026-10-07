package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryCharacterization(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "..", "components", "api-server", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := resourceSummary(resources), []string{
		"Dinosaur:dinosaurs:species:string",
		"Fossil:fossils:discovery_location:string,estimated_age:int,excavator_name:string,fossil_type:string",
		"Scientist:scientists:field:string,name:string",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repository projection = %#v, want %#v", got, want)
	}
	if resources[0].DefaultColumns != "id, species, created_at" || resources[1].DefaultColumns != "id, discovery_location, estimated_age, excavator_name, fossil_type, created_at" {
		t.Fatalf("legacy columns changed: %#v", resources)
	}
}

func TestSharedFixtureConformance(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "openapi-ir", "testdata", "conformance", "openapi.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Name != "Widget" || resources[0].PathSegment != "widgets" {
		t.Fatalf("shared fixture projection = %#v", resources)
	}
	if !containsCLIField(resources[0].WritableFields, "name") || containsCLIField(resources[0].WritableFields, "id") {
		t.Fatalf("schema-derived writable fields are wrong: %#v", resources[0].WritableFields)
	}
}

func TestDeleteAndUpdateFollowTheSpec(t *testing.T) {
	repo, err := parseResources(filepath.Join("..", "..", "components", "api-server", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range repo {
		if !resource.DeleteEnabled || !resource.UpdateEnabled || resource.UpdateMethod != "PATCH" || len(resource.UpdateFields) == 0 {
			t.Fatalf("%s should have delete and PATCH update from the spec: %#v", resource.Name, resource)
		}
	}
	readOnly, err := parseResources(filepath.Join("testdata", "read-only.yaml"), "")
	if err != nil || len(readOnly) != 1 {
		t.Fatalf("read-only fixture: %v %#v", err, readOnly)
	}
	if readOnly[0].DeleteEnabled || readOnly[0].UpdateEnabled {
		t.Fatalf("a spec without delete or update operations must not enable them: %#v", readOnly[0])
	}
	putOnly, err := parseResources(filepath.Join("testdata", "put-only.yaml"), "")
	if err != nil || len(putOnly) != 1 {
		t.Fatalf("put-only fixture: %v %#v", err, putOnly)
	}
	if putOnly[0].DeleteEnabled || !putOnly[0].UpdateEnabled || putOnly[0].UpdateMethod != "PUT" {
		t.Fatalf("a PUT-only update must be generated as PUT without delete: %#v", putOnly[0])
	}

	output := t.TempDir()
	data := cliData{Binary: "ro-cli", Project: "ro", Module: "example.test/ro-cli", Resources: readOnly}
	if err := generateCLI(data, output); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{filepath.Join("cmd", "ro-cli", "delete"), filepath.Join("cmd", "ro-cli", "update"), filepath.Join("pkg", "confirm")} {
		if _, err := os.Stat(filepath.Join(output, absent)); !os.IsNotExist(err) {
			t.Fatalf("%s must not be generated for a read-only spec (err=%v)", absent, err)
		}
	}
	registrations := readTestFile(t, filepath.Join(output, "cmd", "ro-cli", "generated_commands.go"))
	if strings.Contains(registrations, "/delete\"") || strings.Contains(registrations, "/update\"") {
		t.Fatalf("delete or update is registered for a read-only spec:\n%s", registrations)
	}
}

func TestSingletonEndpointIsNotAResource(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "openapi-ir", "testdata", "conformance", "singleton.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Name != "Thing" {
		t.Fatalf("a GET that returns one object on a path without a parameter must not become a resource: %#v", resources)
	}
}

func TestRegenerationRegistersNewCommandsWithoutTouchingMain(t *testing.T) {
	readOnly, err := parseResources(filepath.Join("testdata", "read-only.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	var warnings bytes.Buffer
	previous := warningOutput
	warningOutput = &warnings
	defer func() { warningOutput = previous }()

	output := t.TempDir()
	data := cliData{Binary: "ro-cli", Project: "ro", Module: "example.test/ro-cli", Resources: readOnly}
	if err := generateCLI(data, output); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(output, "cmd", "ro-cli", "main.go")
	registrationsPath := filepath.Join(output, "cmd", "ro-cli", "generated_commands.go")
	handMaintained := readTestFile(t, mainPath) + "\n// hand-maintained\n"
	if !strings.Contains(handMaintained, "addGeneratedCommands(root)") {
		t.Fatalf("a fresh main.go must call addGeneratedCommands:\n%s", handMaintained)
	}
	writeTestFile(t, mainPath, handMaintained)

	data.Resources[0].DeleteEnabled = true
	if err := generateCLI(data, output); err != nil {
		t.Fatal(err)
	}
	if readTestFile(t, mainPath) != handMaintained {
		t.Fatal("regeneration changed the hand-maintained main.go")
	}
	if !strings.Contains(readTestFile(t, registrationsPath), "root.AddCommand(delete.Cmd)") {
		t.Fatal("regeneration did not register the new delete command")
	}
	if warnings.Len() != 0 {
		t.Fatalf("unexpected warning for a main.go that calls addGeneratedCommands: %s", warnings.String())
	}

	// A main.go from before generated_commands.go existed registers commands itself.
	writeTestFile(t, mainPath, "package main\n\nfunc main() {}\n")
	if err := generateCLI(data, output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(warnings.String(), "does not call addGeneratedCommands(root)") {
		t.Fatalf("no warning for a main.go that leaves generated commands unregistered: %q", warnings.String())
	}
}

func TestOIDCClientIDOption(t *testing.T) {
	resources, err := parseResources(filepath.Join("testdata", "read-only.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	loginSource := func(clientID string) string {
		output := t.TempDir()
		data := cliData{Binary: "ro-cli", Project: "ro", Module: "example.test/ro-cli", Resources: resources, OIDCClientID: clientID}
		if err := generateCLI(data, output); err != nil {
			t.Fatal(err)
		}
		return readTestFile(t, filepath.Join(output, "cmd", "ro-cli", "login", "cmd.go"))
	}
	if source := loginSource(""); !strings.Contains(source, `"client-id", "ro-cli"`) {
		t.Fatalf("the default client ID must be the binary name:\n%s", source)
	}
	if source := loginSource("custom-client"); !strings.Contains(source, `"client-id", "custom-client"`) || strings.Contains(source, `"client-id", "ro-cli"`) {
		t.Fatalf("--oidc-client-id must set the login default:\n%s", source)
	}
	for _, invalid := range []string{`bad"id`, "has space", "new\nline", "{{x}}"} {
		data := cliData{Binary: "ro-cli", Project: "ro", Module: "example.test/ro-cli", Resources: resources, OIDCClientID: invalid}
		if err := generateCLI(data, t.TempDir()); err == nil {
			t.Fatalf("client ID %q must be rejected because it is written into generated source", invalid)
		}
	}
}

func TestConfigNameOption(t *testing.T) {
	resources, err := parseResources(filepath.Join("testdata", "read-only.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct{ name, configName, fileName, envVar string }{
		{"default is the binary name", "", "ro-cli", "RO_CLI_CONFIG"},
		{"hyphen and dot in the name", "my-app.v2", "my-app.v2", "MY_APP_V2_CONFIG"},
		{"plain name", "hypershell", "hypershell", "HYPERSHELL_CONFIG"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			output := t.TempDir()
			data := cliData{Binary: "ro-cli", Project: "ro", Module: "example.test/ro-cli", Resources: resources, ConfigName: testCase.configName}
			if err := generateCLI(data, output); err != nil {
				t.Fatal(err)
			}
			check := strings.NewReplacer("__FILE__", testCase.fileName, "__ENV__", testCase.envVar).Replace(configLocationTest)
			writeTestFile(t, filepath.Join(output, "pkg", "config", "location_test.go"), check)
			runCommand(t, output, "go", "mod", "tidy")
			runCommand(t, output, "go", "test", "./pkg/config/")
		})
	}
	for _, invalid := range []string{"has space", "a/b", "../x", "-leading", "dot..dot", `q"uote`, "{{x}}"} {
		data := cliData{Binary: "ro-cli", Project: "ro", Module: "example.test/ro-cli", Resources: resources, ConfigName: invalid}
		if err := generateCLI(data, t.TempDir()); err == nil {
			t.Fatalf("config name %q must be rejected because it becomes a file name and an environment variable", invalid)
		}
	}
}

func TestGeneratedCLIAcceptance(t *testing.T) {
	resources, err := parseResources(filepath.Join("..", "..", "components", "api-server", "openapi", "openapi.yaml"), "/api/rh-trex-ai/v1")
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	data := cliData{
		Binary: "trex-cli", Project: "rh-trex-ai", APIPrefix: "/api/rh-trex-ai/v1",
		Module: "github.com/openshift-online/rh-trex-ai-cli", Resources: resources,
	}
	if err := generateCLI(data, output); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"list/dinosaurs", "get/dinosaur", "create/dinosaur"} {
		source := readTestFile(t, filepath.Join(output, "cmd", "trex-cli", filepath.FromSlash(command), "cmd.go"))
		if !strings.Contains(source, "connection.NewConnectionBuilder()") {
			t.Fatalf("generated %s command bypasses the connection builder, so registered factories would not apply", command)
		}
	}
	writeTestFile(t, filepath.Join(output, "pkg", "connection", "factory_test.go"), connectionFactoryTest)
	writeTestFile(t, filepath.Join(output, "pkg", "config", "store_test.go"), configStoreTest)
	writeTestFile(t, filepath.Join(output, "pkg", "connection", "proxy_test.go"), connectionProxyTest)
	writeTestFile(t, filepath.Join(output, "pkg", "confirm", "confirm_test.go"), confirmTest)
	writeTestFile(t, filepath.Join(output, "pkg", "oidc", "oidc_test.go"), oidcPackageTest)
	writeTestFile(t, filepath.Join(output, "pkg", "connection", "provider_test.go"), configTokenProviderTest)
	runCommand(t, output, "go", "mod", "tidy")
	runCommand(t, output, "go", "test", "./...")
	binary := filepath.Join(output, "trex-cli")
	runCommand(t, output, "go", "build", "-o", binary, "./cmd/trex-cli")
	help := runCommand(t, output, binary, "list", "dinosaurs", "--help")
	if !strings.Contains(help, "List dinosaurs") || !strings.Contains(help, "--columns") {
		t.Fatalf("generated command behavior changed:\n%s", help)
	}
	assertDeleteAndUpdateBehavior(t, output, binary)
	assertOIDCBehavior(t, output, binary)
	generated := readTestFile(t, filepath.Join(output, "pkg", "urls", "urls.go"))
	if !strings.Contains(generated, `DinosaursPath = APIPrefix + "/dinosaurs"`) {
		t.Fatalf("generated list route is not exact:\n%s", generated)
	}
}

type recordedRequest struct {
	Method, Path, Body string
}

// assertDeleteAndUpdateBehavior runs the built CLI against a mock server and
// checks the generated delete confirmation and update body behavior.
func assertDeleteAndUpdateBehavior(t *testing.T, output, binary string) {
	t.Helper()
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, recordedRequest{r.Method, r.URL.Path, string(body)})
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"abc","kind":"Dinosaur","species":"Foo"}`)
	}))
	defer server.Close()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".trex-cli.json"), `{"access_token":"token","url":"`+server.URL+`"}`)
	env := []string{"HOME=" + home}
	const item = "/api/rh-trex-ai/v1/dinosaurs/abc"

	// Without a terminal and without --yes, delete must refuse and send nothing.
	_, stderr, err := runCommandResult(output, env, binary, "delete", "dinosaur", "abc")
	if err == nil || !strings.Contains(stderr, "--yes") || len(requests) != 0 {
		t.Fatalf("delete without a terminal or --yes must fail before any request: err=%v stderr=%q requests=%v", err, stderr, requests)
	}

	stdout, stderr, err := runCommandResult(output, env, binary, "delete", "dinosaur", "abc", "--yes")
	if err != nil || strings.TrimSpace(stdout) != "Dinosaur abc deleted." {
		t.Fatalf("delete --yes: err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if len(requests) != 1 || requests[0].Method != http.MethodDelete || requests[0].Path != item {
		t.Fatalf("delete request = %#v", requests)
	}

	requests = nil
	_, stderr, err = runCommandResult(output, env, binary, "update", "dinosaur", "abc")
	if err == nil || !strings.Contains(stderr, "nothing to update") || len(requests) != 0 {
		t.Fatalf("update without fields must fail before any request: err=%v stderr=%q requests=%v", err, stderr, requests)
	}

	stdout, stderr, err = runCommandResult(output, env, binary, "update", "dinosaur", "abc", "--species", "Foo")
	if err != nil || !strings.Contains(stdout, "Foo") {
		t.Fatalf("update: err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if len(requests) != 1 || requests[0].Method != http.MethodPatch || requests[0].Path != item || requests[0].Body != `{"species":"Foo"}` {
		t.Fatalf("update must PATCH only the flags that were set: %#v", requests)
	}
}

func resourceSummary(resources []cliResource) []string {
	result := make([]string, 0, len(resources))
	for _, resource := range resources {
		fields := make([]string, 0, len(resource.WritableFields))
		for _, field := range resource.WritableFields {
			fields = append(fields, field.Name+":"+field.GoType)
		}
		result = append(result, resource.Name+":"+resource.PathSegment+":"+strings.Join(fields, ","))
	}
	return result
}

func containsCLIField(fields []cliField, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

// connectionFactoryTest runs inside the generated module and verifies that
// registered factories replace the default client and token provider.
const connectionFactoryTest = `package connection

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openshift-online/rh-trex-ai-cli/pkg/config"
)

type recordingClient struct{ calls int }

func (c *recordingClient) Do(req *http.Request) (*http.Response, error) {
	c.calls++
	return http.DefaultClient.Do(req)
}

type fixedToken struct{ token string }

func (p fixedToken) GetToken() (string, error) { return p.token, nil }

func TestRegisteredFactoriesApplyToBuilder(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := &config.Config{URL: server.URL, AccessToken: "static"}

	client := &recordingClient{}
	SetHTTPClientFactory(func(*config.Config) HTTPClient { return client })
	SetTokenProviderFactory(func(*config.Config) TokenProvider { return fixedToken{token: "custom"} })
	defer SetHTTPClientFactory(nil)
	defer SetTokenProviderFactory(nil)

	conn, err := NewConnectionBuilder().Config(cfg).Build()
	if err != nil {
		t.Fatal(err)
	}
	response, err := conn.Get("/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if client.calls != 1 || authorization != "Bearer custom" {
		t.Fatalf("custom factories not used: calls=%d authorization=%q", client.calls, authorization)
	}

	// NewConnection keeps its original builder signature for hand-written commands.
	if conn, err = NewConnection().Config(cfg).Build(); err != nil || conn == nil {
		t.Fatalf("NewConnection().Config(cfg).Build() = %v, %v", conn, err)
	}

	SetHTTPClientFactory(nil)
	SetTokenProviderFactory(nil)
	conn, err = NewConnectionBuilder().Config(cfg).Build()
	if err != nil {
		t.Fatal(err)
	}
	response, err = conn.Get("/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if client.calls != 1 || authorization != "Bearer static" {
		t.Fatalf("defaults not restored: calls=%d authorization=%q", client.calls, authorization)
	}
}
`

// configStoreTest runs inside the generated module and verifies that a
// registered Store replaces the default file-backed Load and Save.
const configStoreTest = `package config

import (
	"os"
	"testing"
)

type memoryStore struct {
	saved *Config
	loads int
}

func (s *memoryStore) Load() (*Config, error) {
	s.loads++
	if s.saved == nil {
		return &Config{URL: "memory"}, nil
	}
	copied := *s.saved
	return &copied, nil
}

func (s *memoryStore) Save(cfg *Config) error {
	copied := *cfg
	s.saved = &copied
	return nil
}

func TestRegisteredStoreReplacesFileBackedLoadAndSave(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	location, err := Location()
	if err != nil {
		t.Fatal(err)
	}

	store := &memoryStore{}
	SetStore(store)
	defer SetStore(nil)

	cfg, err := Load()
	if err != nil || cfg.URL != "memory" || store.loads != 1 {
		t.Fatalf("Load did not use the registered store: cfg=%#v err=%v loads=%d", cfg, err, store.loads)
	}
	cfg.AccessToken = "token"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	if store.saved == nil || store.saved.AccessToken != "token" {
		t.Fatalf("Save did not use the registered store: %#v", store.saved)
	}
	if _, err := os.Stat(location); !os.IsNotExist(err) {
		t.Fatalf("default config file was written while a store was registered: %v", err)
	}

	SetStore(nil)
	if err := Save(&Config{URL: "file", AccessToken: "file-token"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(location); err != nil {
		t.Fatalf("default file-backed Save did not write the config file: %v", err)
	}
	loaded, err := Load()
	if err != nil || loaded.URL != "file" || loaded.AccessToken != "file-token" || store.loads != 1 {
		t.Fatalf("nil did not restore file-backed Load: cfg=%#v err=%v loads=%d", loaded, err, store.loads)
	}
}
`

// connectionProxyTest runs inside the generated module. http.ProxyFromEnvironment
// reads the environment once per process, so the assertions run in a re-executed
// test binary that starts with the proxy variables already set.
const connectionProxyTest = `package connection

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const proxyHelperEnv = "GENERATED_CLI_PROXY_HELPER"

func TestDefaultHTTPClientHonorsProxyEnvironment(t *testing.T) {
	if os.Getenv(proxyHelperEnv) == "1" {
		assertDefaultClientProxy(t)
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestDefaultHTTPClientHonorsProxyEnvironment$", "-test.v")
	command.Env = []string{proxyHelperEnv + "=1", "HTTPS_PROXY=http://proxy.example.test:3128"}
	for _, entry := range os.Environ() {
		name := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
		if name == "HTTP_PROXY" || name == "HTTPS_PROXY" || name == "NO_PROXY" || name == "REQUEST_METHOD" || strings.HasPrefix(name, proxyHelperEnv) {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("proxy helper failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "--- PASS: TestDefaultHTTPClientHonorsProxyEnvironment") {
		t.Fatalf("proxy helper did not run the assertions:\n%s", output)
	}
}

func assertDefaultClientProxy(t *testing.T) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "https://api.example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, insecure := range []bool{false, true} {
		client, ok := newDefaultHTTPClient(insecure).(*defaultHTTPClient)
		if !ok {
			t.Fatal("default client has an unexpected type")
		}
		transport, ok := client.client.Transport.(*http.Transport)
		if !ok || transport.Proxy == nil {
			t.Fatalf("insecure=%v: default transport has no proxy function", insecure)
		}
		proxy, err := transport.Proxy(request)
		if err != nil || proxy == nil || proxy.String() != "http://proxy.example.test:3128" {
			t.Fatalf("insecure=%v: proxy = %v, err = %v, want http://proxy.example.test:3128", insecure, proxy, err)
		}
	}
}
`

// confirmTest runs inside the generated module and checks the delete
// confirmation rules without needing a real terminal.
const confirmTest = `package confirm

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestAskRules(t *testing.T) {
	var out bytes.Buffer
	if err := ask("Delete x?", true, strings.NewReader(""), &out, false); err != nil || out.Len() != 0 {
		t.Fatalf("--yes must proceed silently: err=%v out=%q", err, out.String())
	}
	if err := ask("Delete x?", false, strings.NewReader("y\n"), &out, false); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("a non-terminal without --yes must fail: %v", err)
	}
	for _, answer := range []string{"y\n", "Y\n", "yes\n", "YES\n"} {
		out.Reset()
		if err := ask("Delete x?", false, strings.NewReader(answer), &out, true); err != nil {
			t.Fatalf("%q must confirm: %v", answer, err)
		}
		if out.String() != "Delete x? [y/N]: " {
			t.Fatalf("prompt = %q", out.String())
		}
	}
	for _, answer := range []string{"n\n", "\n", "", "no\n", "yep\n", "maybe\n"} {
		if err := ask("Delete x?", false, strings.NewReader(answer), &out, true); !errors.Is(err, ErrCancelled) {
			t.Fatalf("%q must cancel with ErrCancelled, got %v", answer, err)
		}
	}
}
`

// configTokenProviderTest runs inside the generated module and checks the
// exported default provider that other clients, such as a TUI, can reuse.
const configTokenProviderTest = `package connection

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openshift-online/rh-trex-ai-cli/pkg/config"
)

func TestNewConfigTokenProviderRenewsAnExpiringToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var refreshes int32
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": issuer.URL, "token_endpoint": issuer.URL + "/token"})
		case "/token":
			_ = r.ParseForm()
			atomic.AddInt32(&refreshes, 1)
			if r.PostForm.Get("refresh_token") != "refresh-1" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "renewed", "refresh_token": "refresh-2", "expires_in": 3600})
		}
	}))
	defer issuer.Close()

	cfg := &config.Config{
		URL: "http://api.example.test", AccessToken: "opaque-expired", RefreshToken: "refresh-1",
		IssuerURL: issuer.URL, ClientID: "test-client", ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}
	provider := NewConfigTokenProvider(cfg)
	token, err := provider.GetToken()
	if err != nil || token != "renewed" || atomic.LoadInt32(&refreshes) != 1 {
		t.Fatalf("an expiring token must be renewed once: token=%q err=%v refreshes=%d", token, err, refreshes)
	}
	saved, err := config.Load()
	if err != nil || saved.RefreshToken != "refresh-2" || saved.AccessToken != "renewed" {
		t.Fatalf("the rotated session must be saved: %#v %v", saved, err)
	}
	if again, err := provider.GetToken(); err != nil || again != "renewed" || atomic.LoadInt32(&refreshes) != 1 {
		t.Fatalf("a fresh token must not be renewed again: token=%q err=%v refreshes=%d", again, err, refreshes)
	}

	static := NewConfigTokenProvider(&config.Config{AccessToken: "static"})
	if token, err := static.GetToken(); err != nil || token != "static" {
		t.Fatalf("without a refresh token the saved token is returned unchanged: %q %v", token, err)
	}
}
`

// configLocationTest runs inside a generated module; __FILE__ and __ENV__ are
// replaced with the expected config name and environment variable.
const configLocationTest = `package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocationUsesTheConfiguredName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".xdg"))
	t.Setenv("__ENV__", "")
	os.Unsetenv("__ENV__")

	path, err := Location()
	if err != nil || !strings.HasSuffix(path, filepath.Join("__FILE__", "config.json")) {
		t.Fatalf("default location = %q, %v, want a path ending in __FILE__/config.json", path, err)
	}

	legacy := filepath.Join(home, ".__FILE__.json")
	if err := os.WriteFile(legacy, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if path, err := Location(); err != nil || path != legacy {
		t.Fatalf("an existing home-directory file must be used: %q, %v, want %q", path, err, legacy)
	}

	t.Setenv("__ENV__", "/tmp/override.json")
	if path, err := Location(); err != nil || path != "/tmp/override.json" {
		t.Fatalf("the environment override must win: %q, %v", path, err)
	}
}
`
