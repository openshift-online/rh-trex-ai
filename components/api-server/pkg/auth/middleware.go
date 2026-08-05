package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/glog"
)

// JWTHandler provides JWT authentication without OCM dependencies
type JWTHandler struct {
	keysURLs       []string
	keysFile       string
	publicKeys     map[string]*rsa.PublicKey
	keysMutex      sync.RWMutex
	aclFile        string
	publicPaths    []string
	httpClient     *http.Client
	refreshStop    chan struct{}
	lastKidRefresh time.Time
	kidRefreshWait time.Duration
}

// NewJWTHandler creates a new JWT handler instance
func NewJWTHandler() *JWTHandler {
	return &JWTHandler{
		publicKeys:     make(map[string]*rsa.PublicKey),
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		refreshStop:    make(chan struct{}),
		kidRefreshWait: 30 * time.Second,
	}
}

// WithKeysURLs sets the JWK endpoint URLs for key discovery (supports multiple issuers)
func (j *JWTHandler) WithKeysURLs(urls []string) *JWTHandler {
	j.keysURLs = urls
	return j
}

// WithKeysFile sets a local file path for JWK keys
func (j *JWTHandler) WithKeysFile(file string) *JWTHandler {
	j.keysFile = file
	return j
}

// WithACLFile sets the access control list file
func (j *JWTHandler) WithACLFile(file string) *JWTHandler {
	j.aclFile = file
	return j
}

// WithPublicPath adds a public path that doesn't require authentication
func (j *JWTHandler) WithPublicPath(pattern string) *JWTHandler {
	j.publicPaths = append(j.publicPaths, pattern)
	return j
}

// Build creates the HTTP middleware handler
func (j *JWTHandler) Build() (func(http.Handler) http.Handler, error) {
	// Load JWK keys from URL or file
	if err := j.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load JWT keys: %v", err)
	}

	// Start automatic key refresh for URL sources (hourly) and file sources (every 5 minutes)
	go j.refreshKeysLoop()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a public path
			if j.isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract JWT token from Authorization header
			token, err := j.extractToken(r)
			if err != nil {
				glog.Warningf("JWT extraction failed: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Validate and parse the JWT token
			parsedToken, err := j.validateToken(token)
			if err != nil {
				glog.Warningf("JWT validation failed: %v", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Add token to request context using the same key as auth0 middleware
			ctx := context.WithValue(r.Context(), ContextAuthKey, parsedToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// extractToken gets the JWT token from the Authorization header
func (j *JWTHandler) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	// Check for Bearer token format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("invalid Authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return "", fmt.Errorf("empty bearer token")
	}

	return token, nil
}

// validateToken parses and validates the JWT token
func (j *JWTHandler) validateToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Find the corresponding public key (thread-safe read)
		j.keysMutex.RLock()
		publicKey, exists := j.publicKeys[kid]
		j.keysMutex.RUnlock()

		if exists {
			return publicKey, nil
		}

		// Unknown kid — attempt a one-shot refresh if outside cooldown window
		if time.Since(j.lastKidRefresh) >= j.kidRefreshWait {
			j.lastKidRefresh = time.Now()
			if refreshErr := j.loadKeys(); refreshErr != nil {
				glog.Warningf("JWT key refresh on unknown kid failed: %v", refreshErr)
			} else {
				j.keysMutex.RLock()
				publicKey, exists = j.publicKeys[kid]
				j.keysMutex.RUnlock()
				if exists {
					return publicKey, nil
				}
			}
		}

		return nil, fmt.Errorf("unknown key ID: %s", kid)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token, nil
}

// isPublicPath checks if the given path is public (doesn't require auth)
func (j *JWTHandler) isPublicPath(path string) bool {
	for _, pattern := range j.publicPaths {
		// Exact path matching only - prevents auth bypass vulnerability
		if path == pattern || path == pattern+"/" {
			return true
		}
	}
	return false
}

// loadKeys loads JWT verification keys from all configured sources (file and URLs) additively
func (j *JWTHandler) loadKeys() error {
	if j.keysFile == "" && len(j.keysURLs) == 0 {
		return fmt.Errorf("no keys URL or file specified")
	}

	newKeys := make(map[string]*rsa.PublicKey)
	var lastErr error

	if j.keysFile != "" {
		if err := j.loadKeysFromFileInto(newKeys); err != nil {
			glog.Warningf("Failed to load JWT keys from file %s: %v", j.keysFile, err)
			lastErr = err
		}
	}

	for _, url := range j.keysURLs {
		if url == "" {
			continue
		}
		if err := j.loadKeysFromURLInto(url, newKeys); err != nil {
			glog.Warningf("Failed to load JWT keys from URL %s: %v", url, err)
			lastErr = err
		}
	}

	if len(newKeys) == 0 {
		if lastErr != nil {
			return fmt.Errorf("no valid RSA keys loaded from any source: %v", lastErr)
		}
		return fmt.Errorf("no valid RSA keys found in any configured source")
	}

	j.keysMutex.Lock()
	j.publicKeys = newKeys
	j.keysMutex.Unlock()

	glog.Infof("Updated JWT keys: %d RSA keys loaded from all sources", len(newKeys))
	return nil
}

// JWKSet represents a JSON Web Key Set
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"` // Key Type
	Kid string `json:"kid"` // Key ID
	Use string `json:"use"` // Public Key Use
	N   string `json:"n"`   // RSA modulus
	E   string `json:"e"`   // RSA exponent
}

// refreshKeysLoop periodically refreshes JWK keys.
// Uses 5-minute interval when file-only (for kubelet-synced rotations), 1-hour otherwise.
func (j *JWTHandler) refreshKeysLoop() {
	interval := 1 * time.Hour
	if len(j.keysURLs) == 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := j.loadKeys(); err != nil {
				glog.Warningf("Failed to refresh JWT keys: %v", err)
			} else {
				glog.V(1).Info("JWT keys refreshed successfully")
			}
		case <-j.refreshStop:
			glog.V(1).Info("Stopping JWT key refresh loop")
			return
		}
	}
}

// Stop stops the key refresh loop
func (j *JWTHandler) Stop() {
	close(j.refreshStop)
}

// loadKeysFromURLInto fetches JWK keys from the specified URL and merges them into the target map
func (j *JWTHandler) loadKeysFromURLInto(url string, target map[string]*rsa.PublicKey) error {
	glog.V(2).Infof("Loading JWT keys from URL: %s", url)

	resp, err := j.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch JWK set from %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWK endpoint %s returned HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JWK response from %s: %v", url, err)
	}

	return j.parseAndStoreKeys(body, target)
}

// loadKeysFromFileInto loads JWK keys from a local file and merges them into the target map
func (j *JWTHandler) loadKeysFromFileInto(target map[string]*rsa.PublicKey) error {
	glog.V(2).Infof("Loading JWT keys from file: %s", j.keysFile)

	data, err := os.ReadFile(j.keysFile)
	if err != nil {
		return fmt.Errorf("failed to read JWK file: %v", err)
	}

	return j.parseAndStoreKeys(data, target)
}

// parseAndStoreKeys parses a JWK set JSON and adds RSA keys to the target map
func (j *JWTHandler) parseAndStoreKeys(data []byte, target map[string]*rsa.PublicKey) error {
	var jwkSet JWKSet
	if err := json.Unmarshal(data, &jwkSet); err != nil {
		return fmt.Errorf("failed to parse JWK set: %v", err)
	}

	count := 0
	for _, jwk := range jwkSet.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		publicKey, err := j.jwkToRSAPublicKey(&jwk)
		if err != nil {
			glog.Warningf("Failed to convert JWK to RSA public key for kid %s: %v", jwk.Kid, err)
			continue
		}

		target[jwk.Kid] = publicKey
		count++
		glog.V(2).Infof("Loaded RSA public key with kid: %s", jwk.Kid)
	}

	if count == 0 {
		return fmt.Errorf("no valid RSA keys found in JWK set")
	}

	return nil
}

// jwkToRSAPublicKey converts a JWK to an RSA public key
func (j *JWTHandler) jwkToRSAPublicKey(jwk *JWK) (*rsa.PublicKey, error) {
	// Decode base64url-encoded modulus and exponent
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %v", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %v", err)
	}

	// Convert bytes to big integers
	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	// Create RSA public key
	publicKey := &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}

	return publicKey, nil
}

// TokenFromContext extracts the JWT token from the request context
// This replaces authentication.TokenFromContext from OCM SDK
func TokenFromContext(ctx context.Context) (*jwt.Token, error) {
	token := ctx.Value(ContextAuthKey)
	if token == nil {
		return nil, fmt.Errorf("no JWT token found in context")
	}

	jwtToken, ok := token.(*jwt.Token)
	if !ok {
		return nil, fmt.Errorf("invalid token type in context")
	}

	return jwtToken, nil
}
