package tui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 8 << 20

type ClientConfig struct {
	BaseURL         string
	Token           string
	Insecure        bool
	TrustedOrigins  []string
	HTTPClient      *http.Client
	RefreshInterval time.Duration
}

type Client struct {
	descriptor       Descriptor
	config           ClientConfig
	httpClient       *http.Client
	streamClient     *http.Client
	credentialOrigin string
	trustedOrigins   map[string]bool
}

type RequestInput struct {
	Values map[string]any
	Body   []byte
}

type Result struct {
	Status        int
	Headers       http.Header
	Body          any
	RequestURL    string
	RequestMethod string
}

func NewClient(descriptor Descriptor, config ClientConfig) (*Client, error) {
	base := strings.TrimRight(config.BaseURL, "/")
	if base == "" && len(descriptor.Servers) > 0 {
		base = strings.TrimRight(descriptor.Servers[0].URL, "/")
	}
	if base == "" {
		return nil, fmt.Errorf("an API server URL is required")
	}
	parsed, err := validateServerURL(base, config.Insecure)
	if err != nil {
		return nil, err
	}
	config.BaseURL = base
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if config.Insecure && parsed.Scheme == "https" {
		transport.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec -- explicit runtime opt-in
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	streamClient := *httpClient
	streamClient.Timeout = 0
	trusted := make(map[string]bool)
	for _, raw := range config.TrustedOrigins {
		originURL, parseErr := url.Parse(raw)
		if parseErr != nil || originURL.Scheme == "" || originURL.Host == "" {
			return nil, fmt.Errorf("invalid trusted origin %q", raw)
		}
		trusted[origin(originURL)] = true
	}
	return &Client{
		descriptor: descriptor, config: config, httpClient: httpClient, streamClient: &streamClient,
		credentialOrigin: origin(parsed), trustedOrigins: trusted,
	}, nil
}

func (client *Client) Execute(ctx context.Context, operation Operation, input RequestInput) (Result, error) {
	request, err := client.BuildRequest(ctx, operation, input)
	if err != nil {
		return Result{}, err
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return Result{}, newAPIError(operation.ID, request, nil, nil, fmt.Errorf("send request: %w", err))
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Result{}, newAPIError(operation.ID, request, response, data, fmt.Errorf("read response: %w", err))
	}
	if len(data) > maxResponseBytes {
		return Result{}, newAPIError(operation.ID, request, response, data[:maxResponseBytes], fmt.Errorf("response exceeds %d bytes", maxResponseBytes))
	}
	if !acceptsStatus(operation.SuccessStatuses, response.StatusCode) {
		return Result{}, newAPIError(operation.ID, request, response, data, nil)
	}
	result := Result{
		Status: response.StatusCode, Headers: response.Header.Clone(),
		RequestURL: request.URL.String(), RequestMethod: request.Method,
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&result.Body); err != nil {
		return Result{}, newAPIError(operation.ID, request, response, data, fmt.Errorf("decode response: %w", err))
	}
	return result, nil
}

func (client *Client) OpenStream(ctx context.Context, operation Operation, input RequestInput) (*http.Response, error) {
	request, err := client.BuildRequest(ctx, operation, input)
	if err != nil {
		return nil, err
	}
	response, err := client.streamClient.Do(request)
	if err != nil {
		return nil, newAPIError(operation.ID, request, nil, nil, fmt.Errorf("open stream: %w", err))
	}
	if !acceptsStatus(operation.SuccessStatuses, response.StatusCode) {
		defer response.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		if len(data) > maxResponseBytes {
			return nil, newAPIError(operation.ID, request, response, data[:maxResponseBytes], fmt.Errorf("response exceeds %d bytes", maxResponseBytes))
		}
		if readErr != nil {
			return nil, newAPIError(operation.ID, request, response, data, fmt.Errorf("read response: %w", readErr))
		}
		return nil, newAPIError(operation.ID, request, response, data, nil)
	}
	return response, nil
}

func (client *Client) BuildRequest(ctx context.Context, operation Operation, input RequestInput) (*http.Request, error) {
	if err := validateRequestBody(operation, input.Body); err != nil {
		return nil, err
	}
	path, err := BuildPath(operation, input.Values)
	if err != nil {
		return nil, err
	}
	query, headers, err := BuildQueryAndHeaders(operation, input.Values)
	if err != nil {
		return nil, err
	}
	server, err := client.serverFor(operation)
	if err != nil {
		return nil, err
	}
	requestURL, err := url.Parse(strings.TrimRight(server, "/") + path)
	if err != nil {
		return nil, fmt.Errorf("construct request for %s: %w", operation.ID, err)
	}
	var body io.Reader
	if len(input.Body) > 0 {
		body = bytes.NewReader(input.Body)
	}
	request, err := http.NewRequestWithContext(ctx, operation.Method, requestURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("construct request for %s: %w", operation.ID, err)
	}
	request.URL.RawQuery = query
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	if operation.Response.ContentType != "" {
		request.Header.Set("Accept", operation.Response.ContentType)
	} else {
		request.Header.Set("Accept", "application/json")
	}
	if operation.RequestBody != nil && len(input.Body) > 0 {
		request.Header.Set("Content-Type", operation.RequestBody.ContentType)
	}
	if !operation.Security.None {
		bearerSupported := false
		for _, alternative := range operation.Security.Requirements {
			if len(alternative.Schemes) > 0 {
				bearerSupported = true
			}
		}
		if client.config.Token != "" && bearerSupported {
			requestOrigin := origin(requestURL)
			if requestOrigin != client.credentialOrigin && !client.trustedOrigins[requestOrigin] {
				return nil, fmt.Errorf("operation %s uses untrusted credential origin %s", operation.ID, requestOrigin)
			}
			request.Header.Set("Authorization", "Bearer "+client.config.Token)
		}
	}
	return request, nil
}

func validateRequestBody(operation Operation, body []byte) error {
	trimmed := bytes.TrimSpace(body)
	if operation.RequestBody == nil {
		if len(trimmed) > 0 {
			return fmt.Errorf("operation %s does not declare a request body", operation.ID)
		}
		return nil
	}
	if len(trimmed) == 0 {
		if operation.RequestBody.Required {
			return fmt.Errorf("operation %s requires a request body", operation.ID)
		}
		return nil
	}
	if !strings.Contains(strings.ToLower(operation.RequestBody.ContentType), "json") {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("operation %s request body is invalid JSON: %w", operation.ID, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("operation %s request body is invalid JSON: %w", operation.ID, err)
	}
	if len(operation.RequestBody.Fields) == 0 {
		return nil
	}
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("operation %s request body must be a JSON object", operation.ID)
	}
	for _, field := range operation.RequestBody.Fields {
		fieldValue, present := object[field.Name]
		if field.Required && (!present || fieldValue == nil) {
			return fmt.Errorf("operation %s request body requires field %s", operation.ID, field.Name)
		}
		if !present || fieldValue == nil || field.Type == "" {
			continue
		}
		if !jsonValueMatchesType(fieldValue, field.Type) {
			return fmt.Errorf("operation %s request body field %s must be %s", operation.ID, field.Name, field.Type)
		}
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("contains more than one JSON value")
		}
		return err
	}
	return nil
}

func jsonValueMatchesType(value any, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := number.Int64()
		return err == nil
	case "number":
		_, ok := value.(json.Number)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}

func acceptsStatus(statuses []string, code int) bool {
	exact := fmt.Sprint(code)
	for _, status := range statuses {
		if status == exact {
			return true
		}
		if len(status) == 3 && status[0] == exact[0] && strings.EqualFold(status[1:], "XX") {
			return true
		}
	}
	return false
}

func (client *Client) serverFor(operation Operation) (string, error) {
	server := client.config.BaseURL
	documentServer := ""
	if len(client.descriptor.Servers) > 0 {
		documentServer = client.descriptor.Servers[0].URL
	}
	if len(operation.Servers) > 0 && operation.Servers[0].URL != "" && operation.Servers[0].URL != documentServer {
		server = operation.Servers[0].URL
	}
	if _, err := validateServerURL(server, client.config.Insecure); err != nil {
		return "", fmt.Errorf("operation %s server: %w", operation.ID, err)
	}
	return strings.TrimRight(server, "/"), nil
}

func validateServerURL(raw string, insecure bool) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid API server URL %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported API server scheme %q", parsed.Scheme)
	}
	if parsed.Scheme == "http" && !isLoopback(parsed.Hostname()) && !insecure {
		return nil, fmt.Errorf("plaintext HTTP to non-loopback server %s requires --insecure", parsed.Host)
	}
	return parsed, nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func origin(parsed *url.URL) string {
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
