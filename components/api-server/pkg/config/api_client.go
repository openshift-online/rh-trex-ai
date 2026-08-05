package config

import (
	"github.com/spf13/pflag"
)

type APIClientConfig struct {
	BaseURL          string `json:"base_url"`
	ClientID         string `json:"client-id"`
	ClientIDFile     string `json:"client-id_file"`
	ClientSecret     string `json:"client-secret"`
	ClientSecretFile string `json:"client-secret_file"`
	SelfToken        string `json:"self_token"`
	SelfTokenFile    string `json:"self_token_file"`
	TokenURL         string `json:"token_url"`
	Debug            bool   `json:"debug"`
	EnableMock       bool   `json:"enable_mock"`
}

func NewAPIClientConfig() *APIClientConfig {
	return &APIClientConfig{
		BaseURL:          "https://api.integration.openshift.com",
		TokenURL:         "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token",
		ClientIDFile:     "secrets/api-service.clientId",
		ClientSecretFile: "secrets/api-service.clientSecret",
		SelfTokenFile:    "",
		Debug:            false,
		EnableMock:       true,
	}
}

func (c *APIClientConfig) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.ClientIDFile, "api-client-id-file", c.ClientIDFile, "File containing API privileged account client-id")
	fs.StringVar(&c.ClientSecretFile, "api-client-secret-file", c.ClientSecretFile, "File containing API privileged account client-secret")
	fs.StringVar(&c.SelfTokenFile, "self-token-file", c.SelfTokenFile, "File containing API privileged offline SSO token")
	fs.StringVar(&c.BaseURL, "api-base-url", c.BaseURL, "The base URL of the upstream API, integration by default")
	fs.StringVar(&c.TokenURL, "api-token-url", c.TokenURL, "The base URL for requesting tokens, stage by default")
	fs.BoolVar(&c.Debug, "debug", c.Debug, "Debug flag for API client")
	fs.BoolVar(&c.EnableMock, "enable-mock", c.EnableMock, "Enable mock API clients")
}

func (c *APIClientConfig) ReadFiles() error {
	if c.EnableMock {
		return nil
	}
	err := readFileValueString(c.ClientIDFile, &c.ClientID)
	if err != nil {
		return err
	}
	err = readFileValueString(c.ClientSecretFile, &c.ClientSecret)
	if err != nil {
		return err
	}
	err = readFileValueString(c.SelfTokenFile, &c.SelfToken)
	return err
}
