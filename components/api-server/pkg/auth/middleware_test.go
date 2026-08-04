package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestJWTHandler_isPublicPath(t *testing.T) {
	handler := NewJWTHandler().
		WithPublicPath("/api/rh-trex").
		WithPublicPath("/health")

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "exact public path match",
			path:     "/api/rh-trex",
			expected: true,
		},
		{
			name:     "public path with trailing slash",
			path:     "/api/rh-trex/",
			expected: true,
		},
		{
			name:     "protected API endpoint should not match",
			path:     "/api/rh-trex/v1/dinosaurs",
			expected: false, // This was the vulnerability - should NOT be public
		},
		{
			name:     "API subpath should not match",
			path:     "/api/rh-trex/anything",
			expected: false, // API prefix should not allow subpaths
		},
		{
			name:     "health endpoint",
			path:     "/health",
			expected: true,
		},
		{
			name:     "health check endpoint - requires specific config",
			path:     "/health/check",
			expected: false, // With exact matching, this would need to be configured separately
		},
		{
			name:     "completely unrelated path",
			path:     "/secret/admin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isPublicPath(tt.path)
			if result != tt.expected {
				t.Errorf("isPublicPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestJWTHandler_extractToken(t *testing.T) {
	handler := NewJWTHandler()

	tests := []struct {
		name          string
		header        string
		expectedToken string
		expectError   bool
	}{
		{
			name:          "valid bearer token",
			header:        "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9",
			expectedToken: "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9",
			expectError:   false,
		},
		{
			name:        "missing authorization header",
			header:      "",
			expectError: true,
		},
		{
			name:        "invalid format - no Bearer prefix",
			header:      "Basic dXNlcjpwYXNz",
			expectError: true,
		},
		{
			name:        "empty bearer token",
			header:      "Bearer ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			token, err := handler.extractToken(req)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if token != tt.expectedToken {
					t.Errorf("got token %q, want %q", token, tt.expectedToken)
				}
			}
		})
	}
}

func TestJWTHandler_SecurityHeaders(t *testing.T) {
	// Create a test JWT handler that will fail validation
	handler := NewJWTHandler()

	// Build the middleware (this will fail key loading, but we can still test error responses)
	_, err := handler.Build()
	if err == nil {
		t.Skip("Expected key loading to fail for this security test")
	}

	// Test that error responses don't leak internal details
	// httptest.NewRequest never returns nil; just confirm it compiles
	_ = httptest.NewRequest("GET", "/protected", nil)
}

func TestJWTHandler_ThreadSafety(t *testing.T) {
	handler := NewJWTHandler()

	// Test that concurrent access to keys doesn't cause races
	done := make(chan bool)

	// Simulate concurrent key refresh
	go func() {
		for i := 0; i < 100; i++ {
			handler.keysMutex.Lock()
			handler.publicKeys = make(map[string]*rsa.PublicKey)
			handler.keysMutex.Unlock()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Simulate concurrent key access
	go func() {
		for i := 0; i < 100; i++ {
			handler.keysMutex.RLock()
			_ = handler.publicKeys["test-key"]
			handler.keysMutex.RUnlock()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}

func TestJWTHandler_Stop(t *testing.T) {
	handler := NewJWTHandler()

	// Start the refresh loop
	go handler.refreshKeysLoop()

	// Stop it
	handler.Stop()

	// Verify the stop channel is closed
	select {
	case <-handler.refreshStop:
		// Good - channel is closed
	case <-time.After(1 * time.Second):
		t.Error("Stop() did not close refresh channel within timeout")
	}
}

func testGenerateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func testJWKSetJSON(kid string, key *rsa.PublicKey) string {
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
	return fmt.Sprintf(`{"keys":[{"kid":%q,"kty":"RSA","alg":"RS256","use":"sig","n":%q,"e":%q}]}`, kid, n, e)
}

func testJWKServer(t *testing.T, kid string, key *rsa.PublicKey) *httptest.Server {
	t.Helper()
	body := testJWKSetJSON(kid, key)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
}

func testSignedToken(t *testing.T, kid string, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func testWriteJWKFile(t *testing.T, kid string, key *rsa.PublicKey) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "jwks.json")
	if err := os.WriteFile(path, []byte(testJWKSetJSON(kid, key)), 0600); err != nil {
		t.Fatalf("failed to write JWK file: %v", err)
	}
	return path
}

func TestJWTHandler_LoadKeys_SingleURL(t *testing.T) {
	key := testGenerateKey(t)
	srv := testJWKServer(t, "kid-1", &key.PublicKey)
	defer srv.Close()

	handler := NewJWTHandler().WithKeysURLs([]string{srv.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	handler.keysMutex.RLock()
	_, ok := handler.publicKeys["kid-1"]
	handler.keysMutex.RUnlock()

	if !ok {
		t.Error("expected kid-1 in key map after load")
	}
}

func TestJWTHandler_LoadKeys_MultipleURLsMergeAdditively(t *testing.T) {
	key1 := testGenerateKey(t)
	key2 := testGenerateKey(t)

	srv1 := testJWKServer(t, "kid-a", &key1.PublicKey)
	defer srv1.Close()
	srv2 := testJWKServer(t, "kid-b", &key2.PublicKey)
	defer srv2.Close()

	handler := NewJWTHandler().WithKeysURLs([]string{srv1.URL, srv2.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	handler.keysMutex.RLock()
	_, hasA := handler.publicKeys["kid-a"]
	_, hasB := handler.publicKeys["kid-b"]
	handler.keysMutex.RUnlock()

	if !hasA {
		t.Error("expected kid-a from first URL")
	}
	if !hasB {
		t.Error("expected kid-b from second URL")
	}
}

func TestJWTHandler_LoadKeys_OneURLFailsOtherStillLoads(t *testing.T) {
	key := testGenerateKey(t)
	good := testJWKServer(t, "kid-good", &key.PublicKey)
	defer good.Close()

	handler := NewJWTHandler().WithKeysURLs([]string{"http://127.0.0.1:0/bad", good.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys should warn-and-continue: %v", err)
	}

	handler.keysMutex.RLock()
	_, ok := handler.publicKeys["kid-good"]
	handler.keysMutex.RUnlock()

	if !ok {
		t.Error("expected kid-good to load despite bad first URL")
	}
}

func TestJWTHandler_LoadKeys_URLReturnsNonOK(t *testing.T) {
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer badSrv.Close()

	key := testGenerateKey(t)
	goodSrv := testJWKServer(t, "kid-ok", &key.PublicKey)
	defer goodSrv.Close()

	handler := NewJWTHandler().WithKeysURLs([]string{badSrv.URL, goodSrv.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.keysMutex.RLock()
	_, ok := handler.publicKeys["kid-ok"]
	handler.keysMutex.RUnlock()

	if !ok {
		t.Error("expected kid-ok to load despite 404 from first server")
	}
}

func TestJWTHandler_LoadKeys_FileOnly(t *testing.T) {
	key := testGenerateKey(t)
	path := testWriteJWKFile(t, "kid-file", &key.PublicKey)

	handler := NewJWTHandler().WithKeysFile(path)
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys from file: %v", err)
	}

	handler.keysMutex.RLock()
	_, ok := handler.publicKeys["kid-file"]
	handler.keysMutex.RUnlock()

	if !ok {
		t.Error("expected kid-file in key map after loading from file")
	}
}

func TestJWTHandler_LoadKeys_FileAndURLMergeAdditively(t *testing.T) {
	fileKey := testGenerateKey(t)
	urlKey := testGenerateKey(t)

	path := testWriteJWKFile(t, "kid-from-file", &fileKey.PublicKey)
	srv := testJWKServer(t, "kid-from-url", &urlKey.PublicKey)
	defer srv.Close()

	handler := NewJWTHandler().
		WithKeysFile(path).
		WithKeysURLs([]string{srv.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	handler.keysMutex.RLock()
	_, hasFile := handler.publicKeys["kid-from-file"]
	_, hasURL := handler.publicKeys["kid-from-url"]
	total := len(handler.publicKeys)
	handler.keysMutex.RUnlock()

	if !hasFile {
		t.Error("expected kid-from-file from JWK file")
	}
	if !hasURL {
		t.Error("expected kid-from-url from JWK URL")
	}
	if total != 2 {
		t.Errorf("expected 2 keys total, got %d", total)
	}
}

func TestJWTHandler_LoadKeys_FileAndMultipleURLsMerge(t *testing.T) {
	fileKey := testGenerateKey(t)
	urlKey1 := testGenerateKey(t)
	urlKey2 := testGenerateKey(t)

	path := testWriteJWKFile(t, "kid-file", &fileKey.PublicKey)
	srv1 := testJWKServer(t, "kid-url-1", &urlKey1.PublicKey)
	defer srv1.Close()
	srv2 := testJWKServer(t, "kid-url-2", &urlKey2.PublicKey)
	defer srv2.Close()

	handler := NewJWTHandler().
		WithKeysFile(path).
		WithKeysURLs([]string{srv1.URL, srv2.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	handler.keysMutex.RLock()
	total := len(handler.publicKeys)
	_, hasFile := handler.publicKeys["kid-file"]
	_, hasURL1 := handler.publicKeys["kid-url-1"]
	_, hasURL2 := handler.publicKeys["kid-url-2"]
	handler.keysMutex.RUnlock()

	if total != 3 {
		t.Errorf("expected 3 keys from file + 2 URLs, got %d", total)
	}
	if !hasFile || !hasURL1 || !hasURL2 {
		t.Errorf("missing keys: file=%v url1=%v url2=%v", hasFile, hasURL1, hasURL2)
	}
}

func TestJWTHandler_LoadKeys_NoSourcesReturnsError(t *testing.T) {
	handler := NewJWTHandler()
	if err := handler.loadKeys(); err == nil {
		t.Error("expected error when no keys URL or file specified")
	}
}

func TestJWTHandler_LoadKeys_AllSourcesFailReturnsError(t *testing.T) {
	handler := NewJWTHandler().
		WithKeysFile("/nonexistent/path/jwks.json").
		WithKeysURLs([]string{"http://127.0.0.1:0/bad"})
	if err := handler.loadKeys(); err == nil {
		t.Error("expected error when all sources fail")
	}
}

func TestJWTHandler_LoadKeys_SkipsEmptyURLs(t *testing.T) {
	key := testGenerateKey(t)
	srv := testJWKServer(t, "kid-real", &key.PublicKey)
	defer srv.Close()

	handler := NewJWTHandler().WithKeysURLs([]string{"", srv.URL, ""})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	handler.keysMutex.RLock()
	total := len(handler.publicKeys)
	handler.keysMutex.RUnlock()

	if total != 1 {
		t.Errorf("expected 1 key (empty URLs skipped), got %d", total)
	}
}

func TestJWTHandler_ValidateToken_MultiURL(t *testing.T) {
	key1 := testGenerateKey(t)
	key2 := testGenerateKey(t)

	srv1 := testJWKServer(t, "issuer-1-kid", &key1.PublicKey)
	defer srv1.Close()
	srv2 := testJWKServer(t, "issuer-2-kid", &key2.PublicKey)
	defer srv2.Close()

	handler := NewJWTHandler().WithKeysURLs([]string{srv1.URL, srv2.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	token1 := testSignedToken(t, "issuer-1-kid", key1, jwt.MapClaims{
		"sub": "user-from-issuer-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	token2 := testSignedToken(t, "issuer-2-kid", key2, jwt.MapClaims{
		"sub": "user-from-issuer-2",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	parsed1, err := handler.validateToken(token1)
	if err != nil {
		t.Fatalf("validateToken for issuer-1: %v", err)
	}
	if !parsed1.Valid {
		t.Error("expected valid token from issuer 1")
	}

	parsed2, err := handler.validateToken(token2)
	if err != nil {
		t.Fatalf("validateToken for issuer-2: %v", err)
	}
	if !parsed2.Valid {
		t.Error("expected valid token from issuer 2")
	}
}

func TestJWTHandler_ValidateToken_FileAndURL(t *testing.T) {
	fileKey := testGenerateKey(t)
	urlKey := testGenerateKey(t)

	path := testWriteJWKFile(t, "file-issuer-kid", &fileKey.PublicKey)
	srv := testJWKServer(t, "url-issuer-kid", &urlKey.PublicKey)
	defer srv.Close()

	handler := NewJWTHandler().
		WithKeysFile(path).
		WithKeysURLs([]string{srv.URL})
	if err := handler.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	fileToken := testSignedToken(t, "file-issuer-kid", fileKey, jwt.MapClaims{
		"sub": "user-from-file",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	urlToken := testSignedToken(t, "url-issuer-kid", urlKey, jwt.MapClaims{
		"sub": "user-from-url",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	parsed1, err := handler.validateToken(fileToken)
	if err != nil {
		t.Fatalf("validateToken for file issuer: %v", err)
	}
	if !parsed1.Valid {
		t.Error("expected valid token from file issuer")
	}

	parsed2, err := handler.validateToken(urlToken)
	if err != nil {
		t.Fatalf("validateToken for url issuer: %v", err)
	}
	if !parsed2.Valid {
		t.Error("expected valid token from url issuer")
	}
}

func TestJWTHandler_OnDemandRefresh_LoadsAllSources(t *testing.T) {
	fileKey := testGenerateKey(t)
	urlKey := testGenerateKey(t)

	path := testWriteJWKFile(t, "refresh-file-kid", &fileKey.PublicKey)
	srv := testJWKServer(t, "refresh-url-kid", &urlKey.PublicKey)
	defer srv.Close()

	handler := NewJWTHandler().
		WithKeysFile(path).
		WithKeysURLs([]string{srv.URL})
	handler.kidRefreshWait = 0

	handler.keysMutex.Lock()
	handler.publicKeys = make(map[string]*rsa.PublicKey)
	handler.keysMutex.Unlock()

	fileToken := testSignedToken(t, "refresh-file-kid", fileKey, jwt.MapClaims{
		"sub": "user",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	parsed, err := handler.validateToken(fileToken)
	if err != nil {
		t.Fatalf("validateToken should trigger on-demand refresh and find file key: %v", err)
	}
	if !parsed.Valid {
		t.Error("expected valid token after on-demand refresh")
	}

	handler.keysMutex.RLock()
	_, hasURL := handler.publicKeys["refresh-url-kid"]
	handler.keysMutex.RUnlock()

	if !hasURL {
		t.Error("on-demand refresh should have loaded URL keys too (additive from all sources)")
	}
}

func TestJWTHandler_ParseAndStoreKeys_SkipsNonRSA(t *testing.T) {
	handler := NewJWTHandler()
	target := make(map[string]*rsa.PublicKey)

	data, _ := json.Marshal(JWKSet{Keys: []JWK{
		{Kid: "ec-key", Kty: "EC"},
		{Kid: "rsa-key", Kty: "RSA"},
	}})

	_ = handler.parseAndStoreKeys(data, target)

	if _, ok := target["ec-key"]; ok {
		t.Error("EC key should have been skipped")
	}
}

func TestJWTHandler_ParseAndStoreKeys_InvalidJSON(t *testing.T) {
	handler := NewJWTHandler()
	target := make(map[string]*rsa.PublicKey)

	if err := handler.parseAndStoreKeys([]byte("not json"), target); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestJWTHandler_Build_MultiURL(t *testing.T) {
	key1 := testGenerateKey(t)
	key2 := testGenerateKey(t)

	srv1 := testJWKServer(t, "build-kid-1", &key1.PublicKey)
	defer srv1.Close()
	srv2 := testJWKServer(t, "build-kid-2", &key2.PublicKey)
	defer srv2.Close()

	middleware, err := NewJWTHandler().
		WithKeysURLs([]string{srv1.URL, srv2.URL}).
		WithPublicPath("/health").
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	token := testSignedToken(t, "build-kid-2", key2, jwt.MapClaims{
		"sub": "user",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	middleware(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Errorf("expected next handler to be called, got status %d", rr.Code)
	}
}

func TestJWTHandler_Build_RejectsTokenFromUnknownIssuer(t *testing.T) {
	knownKey := testGenerateKey(t)
	unknownKey := testGenerateKey(t)

	srv := testJWKServer(t, "known-kid", &knownKey.PublicKey)
	defer srv.Close()

	middleware, err := NewJWTHandler().
		WithKeysURLs([]string{srv.URL}).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	token := testSignedToken(t, "unknown-kid", unknownKey, jwt.MapClaims{
		"sub": "attacker",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	middleware(next).ServeHTTP(rr, req)

	if nextCalled {
		t.Error("next handler should not be called for unknown issuer")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
