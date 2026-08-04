package grpcutil

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func jwkSetJSON(kid string, key *rsa.PublicKey) string {
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
	return fmt.Sprintf(`{"keys":[{"kid":%q,"kty":"RSA","alg":"RS256","use":"sig","n":%q,"e":%q}]}`, kid, n, e)
}

func jwkServer(t *testing.T, kid string, key *rsa.PublicKey) *httptest.Server {
	t.Helper()
	body := jwkSetJSON(kid, key)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
}

func signedToken(t *testing.T, kid string, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestNewJWKKeyProvider_EmptyURLs(t *testing.T) {
	p := NewJWKKeyProvider([]string{}, "")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if err := p.loadKeys(); err != nil {
		t.Errorf("loadKeys with no sources returned error: %v", err)
	}
	if len(p.keys) != 0 {
		t.Errorf("expected empty key map, got %d keys", len(p.keys))
	}
}

func TestLoadKeys_SingleURL(t *testing.T) {
	key := generateTestKey(t)
	srv := jwkServer(t, "kid-1", &key.PublicKey)
	defer srv.Close()

	p := NewJWKKeyProvider([]string{srv.URL}, "")
	if err := p.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	if _, ok := p.keys["kid-1"]; !ok {
		t.Error("expected kid-1 in key map after load")
	}
}

func TestLoadKeys_MultipleURLsMergeAdditively(t *testing.T) {
	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	srv1 := jwkServer(t, "kid-a", &key1.PublicKey)
	defer srv1.Close()
	srv2 := jwkServer(t, "kid-b", &key2.PublicKey)
	defer srv2.Close()

	p := NewJWKKeyProvider([]string{srv1.URL, srv2.URL}, "")
	if err := p.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	if _, ok := p.keys["kid-a"]; !ok {
		t.Error("expected kid-a from first URL")
	}
	if _, ok := p.keys["kid-b"]; !ok {
		t.Error("expected kid-b from second URL")
	}
}

func TestLoadKeys_OneURLFailsOtherStillLoads(t *testing.T) {
	key := generateTestKey(t)
	good := jwkServer(t, "kid-good", &key.PublicKey)
	defer good.Close()

	p := NewJWKKeyProvider([]string{"http://127.0.0.1:0/bad", good.URL}, "")
	if err := p.loadKeys(); err != nil {
		t.Fatalf("loadKeys returned error when it should warn-and-continue: %v", err)
	}

	if _, ok := p.keys["kid-good"]; !ok {
		t.Error("expected kid-good to load despite bad first URL")
	}
}

func TestLoadKeys_URLReturnsNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	good := generateTestKey(t)
	goodSrv := jwkServer(t, "kid-ok", &good.PublicKey)
	defer goodSrv.Close()

	p := NewJWKKeyProvider([]string{srv.URL, goodSrv.URL}, "")
	if err := p.loadKeys(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.keys["kid-ok"]; !ok {
		t.Error("expected kid-ok to load despite 404 from first server")
	}
}

func TestKeyFunc_ResolvesKidFromCache(t *testing.T) {
	key := generateTestKey(t)
	srv := jwkServer(t, "kid-1", &key.PublicKey)
	defer srv.Close()

	p := NewJWKKeyProvider([]string{srv.URL}, "")
	if err := p.loadKeys(); err != nil {
		t.Fatalf("loadKeys: %v", err)
	}

	tokenStr := signedToken(t, "kid-1", key, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	parsed, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, p.KeyFunc)
	if err != nil {
		t.Fatalf("ParseWithClaims: %v", err)
	}
	if !parsed.Valid {
		t.Error("expected valid token")
	}
}

func TestKeyFunc_TriggersReloadOnUnknownKid(t *testing.T) {
	key := generateTestKey(t)
	srv := jwkServer(t, "kid-new", &key.PublicKey)
	defer srv.Close()

	p := NewJWKKeyProvider([]string{srv.URL}, "")
	p.reloadMinWait = 0

	tokenStr := signedToken(t, "kid-new", key, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	parsed, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, p.KeyFunc)
	if err != nil {
		t.Fatalf("ParseWithClaims after reload: %v", err)
	}
	if !parsed.Valid {
		t.Error("expected valid token after reload")
	}
}

func TestKeyFunc_RejectsUnknownKidAfterRecentReload(t *testing.T) {
	p := NewJWKKeyProvider([]string{}, "")
	p.lastReload = time.Now()
	p.reloadMinWait = time.Hour

	key := generateTestKey(t)
	tokenStr := signedToken(t, "kid-missing", key, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, p.KeyFunc)
	if err == nil {
		t.Fatal("expected error for unknown kid with recent reload")
	}
}

func TestKeyFunc_RejectsWrongSigningMethod(t *testing.T) {
	p := NewJWKKeyProvider([]string{}, "")

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user1"})
	tok.Header["kid"] = "some-kid"
	tokenStr, _ := tok.SignedString([]byte("secret"))

	_, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, p.KeyFunc)
	if err == nil {
		t.Fatal("expected error for non-RSA signing method")
	}
}

func TestKeyFunc_RejectsMissingKid(t *testing.T) {
	key := generateTestKey(t)
	p := NewJWKKeyProvider([]string{}, "")
	p.keys["kid-1"] = &key.PublicKey

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "user1"})
	tokenStr, _ := tok.SignedString(key)

	_, err := jwt.ParseWithClaims(tokenStr, jwt.MapClaims{}, p.KeyFunc)
	if err == nil {
		t.Fatal("expected error for missing kid in header")
	}
}

func TestParseAndStoreKeys_SkipsNonRSA(t *testing.T) {
	p := NewJWKKeyProvider([]string{}, "")
	p.mu.Lock()
	defer p.mu.Unlock()

	data, _ := json.Marshal(jwkSetData{Keys: []jwkKeyData{
		{Kid: "ec-key", Kty: "EC"},
		{Kid: "rsa-key", Kty: "RSA"},
	}})

	_ = p.parseAndStoreKeys(data)

	if _, ok := p.keys["ec-key"]; ok {
		t.Error("EC key should have been skipped")
	}
}

func TestParseAndStoreKeys_InvalidJSON(t *testing.T) {
	p := NewJWKKeyProvider([]string{}, "")
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.parseAndStoreKeys([]byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}
