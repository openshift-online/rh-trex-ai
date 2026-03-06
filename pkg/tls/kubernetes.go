package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	// Standard Kubernetes service account paths
	DefaultServiceCAPath      = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	DefaultServiceTokenPath   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	DefaultServiceNamespace   = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	
	// OpenShift service serving certificate paths
	DefaultServingCertPath    = "/etc/serving-certs/tls.crt"
	DefaultServingKeyPath     = "/etc/serving-certs/tls.key"
	
	// OpenShift service CA bundle path
	DefaultServiceCABundle    = "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
	DefaultOpenShiftCABundle  = "/var/run/configmaps/service-ca/service-ca.crt"
)

// KubernetesCALoader handles loading CA certificates from Kubernetes/OpenShift
type KubernetesCALoader struct {
	ServiceCAPath    string
	ServiceCABundle  string
	OpenShiftCAPath  string
	CustomCAPaths    []string
}

// NewKubernetesCALoader creates a new CA loader with default paths
func NewKubernetesCALoader() *KubernetesCALoader {
	return &KubernetesCALoader{
		ServiceCAPath:   DefaultServiceCAPath,
		ServiceCABundle: DefaultServiceCABundle,
		OpenShiftCAPath: DefaultOpenShiftCABundle,
	}
}

// LoadServiceCA loads the Kubernetes service account CA certificate
func (k *KubernetesCALoader) LoadServiceCA() (*x509.CertPool, error) {
	if !k.fileExists(k.ServiceCAPath) {
		return nil, fmt.Errorf("service CA not found at %s (not running in Kubernetes?)", k.ServiceCAPath)
	}
	
	return k.loadCAFromFile(k.ServiceCAPath)
}

// LoadSystemCA loads the system CA bundle
func (k *KubernetesCALoader) LoadSystemCA() (*x509.CertPool, error) {
	// Try OpenShift service CA first
	if k.fileExists(k.OpenShiftCAPath) {
		pool, err := k.loadCAFromFile(k.OpenShiftCAPath)
		if err == nil {
			return pool, nil
		}
	}
	
	// Fall back to system CA bundle
	if k.fileExists(k.ServiceCABundle) {
		return k.loadCAFromFile(k.ServiceCABundle)
	}
	
	// Use system default CA pool
	pool, err := x509.SystemCertPool()
	if err != nil {
		// Fallback to empty pool on systems where SystemCertPool fails
		return x509.NewCertPool(), nil
	}
	return pool, nil
}

// LoadCombinedCA loads both service and system CAs into a single pool
func (k *KubernetesCALoader) LoadCombinedCA() (*x509.CertPool, error) {
	// Start with system CA
	pool, err := k.LoadSystemCA()
	if err != nil {
		pool = x509.NewCertPool()
	}
	
	// Add service CA if available
	serviceCA, err := k.LoadServiceCA()
	if err == nil {
		// Merge service CA into the pool
		for _, cert := range serviceCA.Subjects() {
			pool.AppendCertsFromPEM(cert)
		}
	}
	
	// Add any custom CA files
	for _, caPath := range k.CustomCAPaths {
		if k.fileExists(caPath) {
			customCA, err := k.loadCAFromFile(caPath)
			if err == nil {
				for _, cert := range customCA.Subjects() {
					pool.AppendCertsFromPEM(cert)
				}
			}
		}
	}
	
	return pool, nil
}

// IsRunningInKubernetes detects if the service is running in a Kubernetes cluster
func (k *KubernetesCALoader) IsRunningInKubernetes() bool {
	return k.fileExists(k.ServiceCAPath) && k.fileExists(DefaultServiceTokenPath)
}

// IsRunningInOpenShift detects if the service is running in OpenShift
func (k *KubernetesCALoader) IsRunningInOpenShift() bool {
	return k.IsRunningInKubernetes() && k.fileExists(k.OpenShiftCAPath)
}

// GetNamespace returns the current Kubernetes namespace
func (k *KubernetesCALoader) GetNamespace() (string, error) {
	if !k.fileExists(DefaultServiceNamespace) {
		return "", fmt.Errorf("namespace file not found (not running in Kubernetes?)")
	}
	
	data, err := ioutil.ReadFile(DefaultServiceNamespace)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace: %w", err)
	}
	
	return string(data), nil
}

// LoadServingCertificate loads OpenShift service serving certificates
func (k *KubernetesCALoader) LoadServingCertificate() (tls.Certificate, error) {
	if !k.fileExists(DefaultServingCertPath) || !k.fileExists(DefaultServingKeyPath) {
		return tls.Certificate{}, fmt.Errorf("serving certificate not found at %s/%s", DefaultServingCertPath, DefaultServingKeyPath)
	}
	
	cert, err := tls.LoadX509KeyPair(DefaultServingCertPath, DefaultServingKeyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load serving certificate: %w", err)
	}
	
	return cert, nil
}

// NewKubernetesServerTLSConfig creates a server TLS config using Kubernetes/OpenShift certificates
func NewKubernetesServerTLSConfig() (*tls.Config, error) {
	loader := NewKubernetesCALoader()
	
	// Use secure base configuration
	config := SecureTLSConfig()
	
	// Try to load serving certificate
	if loader.fileExists(DefaultServingCertPath) && loader.fileExists(DefaultServingKeyPath) {
		cert, err := loader.LoadServingCertificate()
		if err != nil {
			return nil, fmt.Errorf("failed to load serving certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	} else {
		return nil, fmt.Errorf("no serving certificate found - ensure service has serving-certs annotation")
	}
	
	// Load CA pool for client certificate verification if needed
	clientCA, err := loader.LoadCombinedCA()
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificates: %w", err)
	}
	config.ClientCAs = clientCA
	
	return config, nil
}

// NewKubernetesClientTLSConfig creates a client TLS config using Kubernetes CA
func NewKubernetesClientTLSConfig(serverName string, enableClientCerts bool) (*tls.Config, error) {
	loader := NewKubernetesCALoader()
	
	// Use secure base configuration
	config := SecureTLSConfig()
	config.ServerName = serverName
	
	// Load CA certificates
	rootCA, err := loader.LoadCombinedCA()
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificates: %w", err)
	}
	config.RootCAs = rootCA
	
	// Add client certificate if available and enabled
	if enableClientCerts && loader.fileExists(DefaultServingCertPath) && loader.fileExists(DefaultServingKeyPath) {
		clientCert, err := loader.LoadServingCertificate()
		if err == nil {
			config.Certificates = []tls.Certificate{clientCert}
		}
	}
	
	return config, nil
}

// AddCustomCA adds a custom CA file path for loading
func (k *KubernetesCALoader) AddCustomCA(caPath string) {
	k.CustomCAPaths = append(k.CustomCAPaths, caPath)
}

// fileExists checks if a file exists
func (k *KubernetesCALoader) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadCAFromFile loads CA certificates from a file
func (k *KubernetesCALoader) loadCAFromFile(path string) (*x509.CertPool, error) {
	caCert, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file %s: %w", path, err)
	}
	
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", path)
	}
	
	return pool, nil
}

// AutoConfigureTLS automatically configures TLS based on the runtime environment
func AutoConfigureTLS(serverName string) (*tls.Config, error) {
	loader := NewKubernetesCALoader()
	
	if loader.IsRunningInKubernetes() {
		// Running in Kubernetes/OpenShift - use service certificates
		if loader.fileExists(DefaultServingCertPath) {
			// Server mode with serving certificate
			return NewKubernetesServerTLSConfig()
		} else {
			// Client mode with service CA
			return NewKubernetesClientTLSConfig(serverName, false)
		}
	} else {
		// Running outside Kubernetes - use standard TLS
		config := SecureTLSConfig()
		config.ServerName = serverName
		
		// Load system CA
		if systemCA, err := loader.LoadSystemCA(); err == nil {
			config.RootCAs = systemCA
		}
		
		return config, nil
	}
}

// GetTLSEnvironmentInfo returns information about the TLS environment
func GetTLSEnvironmentInfo() map[string]interface{} {
	loader := NewKubernetesCALoader()
	
	info := make(map[string]interface{})
	info["running_in_kubernetes"] = loader.IsRunningInKubernetes()
	info["running_in_openshift"] = loader.IsRunningInOpenShift()
	
	// Check for available certificates
	info["service_ca_available"] = loader.fileExists(DefaultServiceCAPath)
	info["serving_cert_available"] = loader.fileExists(DefaultServingCertPath)
	info["openshift_ca_available"] = loader.fileExists(DefaultOpenShiftCABundle)
	
	// Get namespace if available
	if namespace, err := loader.GetNamespace(); err == nil {
		info["namespace"] = namespace
	}
	
	// List available CA files
	var availableCAs []string
	caPaths := []string{
		DefaultServiceCAPath,
		DefaultServiceCABundle,
		DefaultOpenShiftCABundle,
	}
	
	for _, path := range caPaths {
		if loader.fileExists(path) {
			availableCAs = append(availableCAs, path)
		}
	}
	info["available_ca_files"] = availableCAs
	
	// Check for custom mounts
	customPaths := []string{
		"/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu
		"/etc/pki/tls/certs/ca-bundle.crt",   // RHEL/CentOS
		"/usr/local/share/ca-certificates",   // Custom CA directory
	}
	
	var customCAs []string
	for _, path := range customPaths {
		if loader.fileExists(path) {
			customCAs = append(customCAs, path)
		}
	}
	info["system_ca_files"] = customCAs
	
	return info
}