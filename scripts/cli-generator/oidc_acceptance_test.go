package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeIssuer is a minimal OpenID Connect issuer for black-box CLI tests. It
// serves discovery, the device grant, refresh with rotation and revocation.
type fakeIssuer struct {
	*httptest.Server
	mu            sync.Mutex
	calls         []string
	revoked       []string
	currentToken  string
	issued        int
	rejectRefresh bool
	failRevoke    bool
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	issuer := &fakeIssuer{currentToken: "refresh-1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		issuer.record("discovery")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                        issuer.URL,
			"authorization_endpoint":        issuer.URL + "/authorize",
			"token_endpoint":                issuer.URL + "/token",
			"device_authorization_endpoint": issuer.URL + "/device",
			"revocation_endpoint":           issuer.URL + "/revoke",
		})
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		issuer.record("device")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "ABCD-EFGH", "verification_uri": issuer.URL + "/activate",
			"expires_in": 60, "interval": 1,
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		grant := r.PostForm.Get("grant_type")
		issuer.record("token:" + grant)
		issuer.mu.Lock()
		defer issuer.mu.Unlock()
		switch {
		case strings.HasSuffix(grant, "device_code"):
			issuer.respondTokens(w)
		case grant == "refresh_token":
			if issuer.rejectRefresh || r.PostForm.Get("refresh_token") != issuer.currentToken {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "refresh token rejected"})
				return
			}
			issuer.respondTokens(w)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unsupported_grant_type"})
		}
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		issuer.record("revoke")
		issuer.mu.Lock()
		issuer.revoked = append(issuer.revoked, r.PostForm.Get("token"))
		failing := issuer.failRevoke
		issuer.mu.Unlock()
		if failing {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	issuer.Server = httptest.NewServer(mux)
	t.Cleanup(issuer.Close)
	return issuer
}

func (issuer *fakeIssuer) record(call string) {
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	issuer.calls = append(issuer.calls, call)
}

func (issuer *fakeIssuer) count(call string) int {
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	total := 0
	for _, recorded := range issuer.calls {
		if recorded == call {
			total++
		}
	}
	return total
}

// respondTokens issues a JWT access token and rotates the refresh token. The
// caller holds issuer.mu.
func (issuer *fakeIssuer) respondTokens(w http.ResponseWriter) {
	issuer.issued++
	issuer.currentToken = fmt.Sprintf("refresh-%d", issuer.issued+1)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  testJWT(time.Now().Add(time.Hour), fmt.Sprintf("access-%d", issuer.issued)),
		"refresh_token": issuer.currentToken,
		"expires_in":    3600,
	})
}

// testJWT builds an unsigned-in-practice JWT; the CLI only decodes it.
func testJWT(expiry time.Time, jti string) string {
	encode := func(value any) string {
		data, _ := json.Marshal(value)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	return encode(map[string]string{"alg": "HS256", "typ": "JWT"}) + "." + encode(map[string]any{
		"exp": expiry.Unix(), "jti": jti, "preferred_username": "alice", "email": "alice@example.test",
		"iss": "https://sso.example.test/realms/example", "sub": "user-1",
	}) + ".c2lnbmF0dXJl"
}

func readConfigFile(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".trex-cli.json"))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	return values
}

func writeConfigFile(t *testing.T, home string, values map[string]any) {
	t.Helper()
	data, _ := json.Marshal(values)
	writeTestFile(t, filepath.Join(home, ".trex-cli.json"), string(data))
}

// assertOIDCBehavior drives the built CLI against a fake issuer and API server.
func assertOIDCBehavior(t *testing.T, output, binary string) {
	t.Helper()
	var apiMu sync.Mutex
	var authorizations []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiMu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		apiMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc","kind":"Dinosaur"}`))
	}))
	defer api.Close()
	apiRequests := func() []string {
		apiMu.Lock()
		defer apiMu.Unlock()
		return append([]string(nil), authorizations...)
	}
	run := func(home string, arguments ...string) (string, string, error) {
		return runCommandResult(output, []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".xdg")}, binary, arguments...)
	}

	t.Run("static token login keeps working and clears OIDC fields", func(t *testing.T) {
		home := t.TempDir()
		writeConfigFile(t, home, map[string]any{"access_token": "old", "refresh_token": "r", "issuer_url": "https://x", "client_id": "c", "expires_at": 1})
		tokenFile := filepath.Join(home, "token")
		writeTestFile(t, tokenFile, "static-token\n")
		if _, stderr, err := run(home, "login", "--token-file", tokenFile, "--url", api.URL); err != nil {
			t.Fatalf("static login failed: %v %s", err, stderr)
		}
		values := readConfigFile(t, home)
		if values["access_token"] != "static-token" || values["url"] != api.URL {
			t.Fatalf("static login config = %#v", values)
		}
		for _, field := range []string{"refresh_token", "issuer_url", "client_id", "expires_at"} {
			if _, present := values[field]; present {
				t.Fatalf("static login must clear %s: %#v", field, values)
			}
		}
		if _, stderr, err := run(home, "login", "--url", api.URL); err == nil || !strings.Contains(stderr, "token is required") {
			t.Fatalf("login without a token must fail with an error: err=%v stderr=%q", err, stderr)
		}
	})

	t.Run("issuer URL selects the OIDC device login", func(t *testing.T) {
		issuer := newFakeIssuer(t)
		home := t.TempDir()
		writeConfigFile(t, home, map[string]any{}) // pins the config location to the home directory
		stdout, stderr, err := run(home, "login", "--issuer-url", issuer.URL, "--no-browser", "--url", api.URL)
		if err != nil {
			t.Fatalf("OIDC login failed: %v %s", err, stderr)
		}
		if !strings.Contains(stderr, "ABCD-EFGH") || !strings.Contains(stderr, "/activate") {
			t.Fatalf("device instructions missing: stdout=%q stderr=%q", stdout, stderr)
		}
		if info, err := os.Stat(filepath.Join(home, ".trex-cli.json")); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("the config file holds credentials and must be mode 0600 even when it already existed: %v %v", info, err)
		}
		values := readConfigFile(t, home)
		if values["refresh_token"] != "refresh-2" || values["issuer_url"] != issuer.URL || values["client_id"] != "trex-cli" || values["url"] != api.URL {
			t.Fatalf("OIDC login config = %#v", values)
		}
		if values["expires_at"] == nil || values["access_token"] == "" {
			t.Fatalf("OIDC login must store the access token and its expiry: %#v", values)
		}
		if _, stderr, err := run(home, "login", "--issuer-url", issuer.URL, "--token-file", "/dev/null"); err == nil || !strings.Contains(stderr, "can't be combined") {
			t.Fatalf("--issuer-url with --token-file must be rejected: err=%v stderr=%q", err, stderr)
		}
	})

	t.Run("whoami shows claims and tolerates an opaque token", func(t *testing.T) {
		home := t.TempDir()
		writeConfigFile(t, home, map[string]any{"access_token": testJWT(time.Now().Add(time.Hour), "j"), "url": api.URL})
		stdout, stderr, err := run(home, "whoami")
		if err != nil || !strings.Contains(stdout, "User: alice") || !strings.Contains(stdout, "Email: alice@example.test") || !strings.Contains(stdout, "Issuer: https://sso.example.test/realms/example") || !strings.Contains(stdout, "API URL: "+api.URL) || !strings.Contains(stdout, "expires in") {
			t.Fatalf("whoami: err=%v stdout=%q stderr=%q", err, stdout, stderr)
		}
		if strings.Contains(stdout, "Access token:") || strings.Contains(stdout, "Claims:") {
			t.Fatalf("whoami must not print the token unless asked: %q", stdout)
		}
		stdout, _, err = run(home, "whoami", "--show-token", "--show-token-decoded")
		if err != nil || !strings.Contains(stdout, "Access token: ") || !strings.Contains(stdout, `"preferred_username": "alice"`) {
			t.Fatalf("whoami with token flags: err=%v stdout=%q", err, stdout)
		}
		writeConfigFile(t, home, map[string]any{"access_token": "opaque-token", "url": api.URL})
		stdout, stderr, err = run(home, "whoami")
		if err != nil || !strings.Contains(stdout, "opaque") || !strings.Contains(stdout, "API URL: "+api.URL) {
			t.Fatalf("an opaque token must not be an error: err=%v stdout=%q stderr=%q", err, stdout, stderr)
		}
		writeConfigFile(t, home, map[string]any{})
		if _, stderr, err := run(home, "whoami"); err == nil || !strings.Contains(stderr, "not logged in") {
			t.Fatalf("whoami without credentials must fail: err=%v stderr=%q", err, stderr)
		}
	})

	t.Run("an expired token is refreshed once and the rotation is saved", func(t *testing.T) {
		issuer := newFakeIssuer(t)
		home := t.TempDir()
		before := len(apiRequests())
		writeConfigFile(t, home, map[string]any{
			"access_token": testJWT(time.Now().Add(-time.Hour), "expired"), "refresh_token": "refresh-1",
			"issuer_url": issuer.URL, "client_id": "trex-cli", "url": api.URL,
		})
		if _, stderr, err := run(home, "get", "dinosaur", "abc"); err != nil {
			t.Fatalf("get failed: %v %s", err, stderr)
		}
		if issuer.count("token:refresh_token") != 1 {
			t.Fatalf("expected exactly one refresh, calls = %v", issuer.calls)
		}
		values := readConfigFile(t, home)
		if values["refresh_token"] != "refresh-2" {
			t.Fatalf("the rotated refresh token must be saved: %#v", values)
		}
		sent := apiRequests()[before:]
		if len(sent) != 1 || sent[0] != "Bearer "+values["access_token"].(string) || strings.Contains(sent[0], "expired") {
			t.Fatalf("the request must carry the refreshed token: sent=%v config=%#v", sent, values)
		}
		if _, stderr, err := run(home, "get", "dinosaur", "abc"); err != nil {
			t.Fatalf("second get failed: %v %s", err, stderr)
		}
		if issuer.count("token:refresh_token") != 1 {
			t.Fatalf("a fresh token must not be refreshed again, calls = %v", issuer.calls)
		}
	})

	t.Run("a fresh token causes no refresh", func(t *testing.T) {
		issuer := newFakeIssuer(t)
		home := t.TempDir()
		fresh := testJWT(time.Now().Add(time.Hour), "fresh")
		writeConfigFile(t, home, map[string]any{"access_token": fresh, "refresh_token": "refresh-1", "issuer_url": issuer.URL, "client_id": "trex-cli", "url": api.URL})
		before := len(apiRequests())
		if _, stderr, err := run(home, "get", "dinosaur", "abc"); err != nil {
			t.Fatalf("get failed: %v %s", err, stderr)
		}
		if issuer.count("token:refresh_token") != 0 || apiRequests()[before:][0] != "Bearer "+fresh {
			t.Fatalf("fresh token handling wrong: calls=%v", issuer.calls)
		}
	})

	t.Run("a rejected refresh sends no API request and keeps the config", func(t *testing.T) {
		issuer := newFakeIssuer(t)
		issuer.rejectRefresh = true
		home := t.TempDir()
		writeConfigFile(t, home, map[string]any{
			"access_token": testJWT(time.Now().Add(-time.Hour), "expired"), "refresh_token": "refresh-1",
			"issuer_url": issuer.URL, "client_id": "trex-cli", "url": api.URL,
		})
		original := readConfigFile(t, home)
		before := len(apiRequests())
		_, stderr, err := run(home, "get", "dinosaur", "abc")
		if err == nil || !strings.Contains(stderr, "session expired") {
			t.Fatalf("a rejected refresh must report an expired session: err=%v stderr=%q", err, stderr)
		}
		if len(apiRequests()) != before {
			t.Fatal("no API request may be sent after a rejected refresh")
		}
		if after := readConfigFile(t, home); fmt.Sprint(after) != fmt.Sprint(original) {
			t.Fatalf("a rejected refresh must keep the config: before=%v after=%v", original, after)
		}
	})

	t.Run("logout revokes the latest refresh token and still clears on failure", func(t *testing.T) {
		issuer := newFakeIssuer(t)
		home := t.TempDir()
		writeConfigFile(t, home, map[string]any{"access_token": "a", "refresh_token": "refresh-latest", "issuer_url": issuer.URL, "client_id": "trex-cli", "url": api.URL})
		if _, stderr, err := run(home, "logout"); err != nil {
			t.Fatalf("logout failed: %v %s", err, stderr)
		}
		if len(issuer.revoked) != 1 || issuer.revoked[0] != "refresh-latest" {
			t.Fatalf("logout must revoke the saved refresh token, revoked = %v", issuer.revoked)
		}
		values := readConfigFile(t, home)
		for _, field := range []string{"access_token", "refresh_token", "issuer_url", "client_id", "url"} {
			if _, present := values[field]; present {
				t.Fatalf("logout must clear %s: %#v", field, values)
			}
		}

		issuer.failRevoke = true
		writeConfigFile(t, home, map[string]any{"access_token": "a", "refresh_token": "refresh-latest", "issuer_url": issuer.URL, "client_id": "trex-cli", "url": api.URL})
		_, stderr, err := run(home, "logout")
		if err != nil || !strings.Contains(stderr, "couldn't revoke") {
			t.Fatalf("a revocation failure must only warn: err=%v stderr=%q", err, stderr)
		}
		if _, present := readConfigFile(t, home)["refresh_token"]; present {
			t.Fatal("logout must clear the credentials even when revocation fails")
		}
	})
}

// oidcPackageTest runs inside the generated module and exercises the flows of
// the oidc package that the built binary cannot reach without a browser.
const oidcPackageTest = `package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type testIssuer struct {
	*httptest.Server
	mu          sync.Mutex
	challenge   string
	redirectURI string
	badState    bool
	pending     int
	slowDown    bool
	revoked     url.Values
	refreshes   int
}

func newTestIssuer(t *testing.T) *testIssuer {
	issuer := &testIssuer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize",
			"token_endpoint": issuer.URL + "/token", "device_authorization_endpoint": issuer.URL + "/device",
			"revocation_endpoint": issuer.URL + "/revoke",
		})
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		issuer.mu.Lock()
		issuer.challenge, issuer.redirectURI = query.Get("code_challenge"), query.Get("redirect_uri")
		issuer.mu.Unlock()
		if query.Get("code_challenge_method") != "S256" || query.Get("response_type") != "code" || query.Get("client_id") != "test-client" || !strings.Contains(query.Get("scope"), "openid") {
			http.Error(w, "bad authorize request", http.StatusBadRequest)
			return
		}
		state := query.Get("state")
		if issuer.badState {
			state = "forged"
		}
		http.Redirect(w, r, query.Get("redirect_uri")+"?code=the-code&state="+url.QueryEscape(state), http.StatusFound)
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "dc", "user_code": "UC", "verification_uri": issuer.URL + "/activate", "expires_in": 600, "interval": 2})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		issuer.mu.Lock()
		defer issuer.mu.Unlock()
		fail := func(code string) {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
		}
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			if r.PostForm.Get("code") != "the-code" || r.PostForm.Get("redirect_uri") != issuer.redirectURI || PKCEChallenge(r.PostForm.Get("code_verifier")) != issuer.challenge {
				fail("invalid_grant")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "refresh_token": "refresh", "expires_in": 60})
		case "urn:ietf:params:oauth:grant-type:device_code":
			if issuer.slowDown {
				issuer.slowDown = false
				fail("slow_down")
				return
			}
			if issuer.pending > 0 {
				issuer.pending--
				fail("authorization_pending")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "device-access", "refresh_token": "device-refresh"})
		case "refresh_token":
			issuer.refreshes++
			if r.PostForm.Get("refresh_token") != "good" {
				fail("invalid_grant")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "refresh_token": "rotated", "expires_in": 60})
		default:
			fail("unsupported_grant_type")
		}
	})
	mux.HandleFunc("/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		issuer.mu.Lock()
		issuer.revoked = r.PostForm
		issuer.mu.Unlock()
	})
	issuer.Server = httptest.NewServer(mux)
	t.Cleanup(issuer.Close)
	return issuer
}

func browserOpener(t *testing.T, errs chan<- error) func(string) error {
	return func(target string) error {
		response, err := http.Get(target)
		if err == nil {
			response.Body.Close()
		}
		errs <- err
		return err
	}
}

func TestBrowserLoginUsesPKCEOnALoopbackRedirect(t *testing.T) {
	issuer := newTestIssuer(t)
	client := NewClient(issuer.URL, "test-client", false)
	opened := make(chan error, 1)
	tokens, err := client.LoginBrowser(context.Background(), BrowserOptions{Scopes: []string{"openid"}, OpenBrowser: browserOpener(t, opened), Timeout: 10 * time.Second})
	if err != nil || tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" {
		t.Fatalf("browser login = %#v, %v", tokens, err)
	}
	if !strings.HasPrefix(issuer.redirectURI, "http://127.0.0.1:") || !strings.HasSuffix(issuer.redirectURI, "/callback") {
		t.Fatalf("the redirect must be a loopback callback, got %q", issuer.redirectURI)
	}
	if issuer.challenge == "" {
		t.Fatal("the authorization request must carry a PKCE challenge")
	}
}

func TestBrowserLoginRejectsAForgedState(t *testing.T) {
	issuer := newTestIssuer(t)
	issuer.badState = true
	client := NewClient(issuer.URL, "test-client", false)
	opened := make(chan error, 1)
	_, err := client.LoginBrowser(context.Background(), BrowserOptions{Scopes: []string{"openid"}, OpenBrowser: browserOpener(t, opened), Timeout: 10 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("a callback with the wrong state must abort the login, got %v", err)
	}
}

func TestDeviceLoginPollsAndHonorsSlowDown(t *testing.T) {
	issuer := newTestIssuer(t)
	issuer.pending, issuer.slowDown = 2, true
	client := NewClient(issuer.URL, "test-client", false)
	var waits []time.Duration
	client.Sleep = func(d time.Duration) { waits = append(waits, d) }
	tokens, err := client.LoginDevice(context.Background(), []string{"openid"}, nil)
	if err != nil || tokens.RefreshToken != "device-refresh" {
		t.Fatalf("device login = %#v, %v", tokens, err)
	}
	want := []time.Duration{2 * time.Second, 7 * time.Second, 7 * time.Second, 7 * time.Second}
	if len(waits) != len(want) {
		t.Fatalf("poll waits = %v, want %v", waits, want)
	}
	for index := range want {
		if waits[index] != want[index] {
			t.Fatalf("poll waits = %v, want %v (slow_down adds five seconds)", waits, want)
		}
	}
}

func TestRefreshRotatesAndMapsRejectionToSessionExpired(t *testing.T) {
	issuer := newTestIssuer(t)
	client := NewClient(issuer.URL, "test-client", false)
	tokens, err := client.Refresh(context.Background(), "good")
	if err != nil || tokens.AccessToken != "new-access" || tokens.RefreshToken != "rotated" || tokens.ExpiresAt(time.Now()) == 0 {
		t.Fatalf("refresh = %#v, %v", tokens, err)
	}
	_, err = client.Refresh(context.Background(), "stale")
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("a rejected refresh token must map to ErrSessionExpired, got %v", err)
	}
	if strings.Contains(err.Error(), "stale") {
		t.Fatalf("errors must not contain token values: %v", err)
	}
}

func TestRevokeSendsTheRefreshTokenHint(t *testing.T) {
	issuer := newTestIssuer(t)
	client := NewClient(issuer.URL, "test-client", false)
	if err := client.Revoke(context.Background(), "to-revoke"); err != nil {
		t.Fatal(err)
	}
	if issuer.revoked.Get("token") != "to-revoke" || issuer.revoked.Get("token_type_hint") != "refresh_token" || issuer.revoked.Get("client_id") != "test-client" {
		t.Fatalf("revocation request = %v", issuer.revoked)
	}
}

func TestIssuerMustBeHTTPSUnlessLoopbackOrInsecure(t *testing.T) {
	client := NewClient("http://sso.example.test", "test-client", false)
	if _, err := client.Discover(context.Background()); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("a plain-http remote issuer must be rejected, got %v", err)
	}
	client = NewClient("ftp://sso.example.test", "test-client", true)
	if _, err := client.Discover(context.Background()); err == nil {
		t.Fatal("a non-http issuer scheme must be rejected")
	}
}
`
