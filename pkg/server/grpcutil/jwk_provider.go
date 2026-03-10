package grpcutil

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/glog"
)

type jwkKeyData struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwkSetData struct {
	Keys []jwkKeyData `json:"keys"`
}

type JWKKeyProvider struct {
	mu            sync.RWMutex
	keys          map[string]*rsa.PublicKey
	keysURL       string
	keysFile      string
	lastReload    time.Time
	reloadMinWait time.Duration
}

func NewJWKKeyProvider(keysURL, keysFile string) *JWKKeyProvider {
	return &JWKKeyProvider{
		keys:          make(map[string]*rsa.PublicKey),
		keysURL:       keysURL,
		keysFile:      keysFile,
		reloadMinWait: 1 * time.Minute,
	}
}

func (p *JWKKeyProvider) KeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}

	kidVal, ok := token.Header["kid"]
	if !ok {
		return nil, fmt.Errorf("token header missing kid")
	}
	kid, ok := kidVal.(string)
	if !ok {
		return nil, fmt.Errorf("token header kid is not a string")
	}

	p.mu.RLock()
	key, found := p.keys[kid]
	p.mu.RUnlock()

	if found {
		return key, nil
	}

	if time.Since(p.lastReload) < p.reloadMinWait {
		return nil, fmt.Errorf("unknown kid %q and keys were recently reloaded", kid)
	}

	if err := p.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to reload JWK keys: %w", err)
	}

	p.mu.RLock()
	key, found = p.keys[kid]
	p.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("unknown kid %q after key reload", kid)
	}
	return key, nil
}

func (p *JWKKeyProvider) loadKeys() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.lastReload = time.Now()

	if p.keysFile != "" {
		if err := p.loadKeysFromFile(); err != nil {
			glog.Warningf("JWKKeyProvider: failed to load keys from file %s: %v", p.keysFile, err)
		}
	}
	if p.keysURL != "" {
		if err := p.loadKeysFromURL(); err != nil {
			return fmt.Errorf("failed to load keys from URL %s: %w", p.keysURL, err)
		}
	}
	return nil
}

func (p *JWKKeyProvider) loadKeysFromFile() error {
	data, err := os.ReadFile(p.keysFile)
	if err != nil {
		return err
	}
	return p.parseAndStoreKeys(data)
}

func (p *JWKKeyProvider) loadKeysFromURL() error {
	resp, err := http.Get(p.keysURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWK endpoint returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return p.parseAndStoreKeys(data)
}

func (p *JWKKeyProvider) parseAndStoreKeys(data []byte) error {
	var keySet jwkSetData
	if err := json.Unmarshal(data, &keySet); err != nil {
		return fmt.Errorf("failed to parse JWK set: %w", err)
	}

	for _, k := range keySet.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pubKey, err := parseRSAPublicKey(k)
		if err != nil {
			glog.Warningf("JWKKeyProvider: skipping key kid=%s: %v", k.Kid, err)
			continue
		}
		p.keys[k.Kid] = pubKey
	}
	return nil
}

func parseRSAPublicKey(k jwkKeyData) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nb),
		E: int(new(big.Int).SetBytes(eb).Int64()),
	}, nil
}
