package logging

import (
	"bytes"
	"context"
	"io"
	"log"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugRequestLogRedactsCredentials(t *testing.T) {
	prev := slog.Default()
	w, flags := log.Writer(), log.Flags()
	t.Cleanup(func() { slog.SetDefault(prev); log.SetOutput(w); log.SetFlags(flags) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})))

	r := httptest.NewRequest("GET", "/api/x", nil)
	r.Header.Set("Authorization", "Bearer secret-token")
	r.Header.Set("Cookie", "session=secret-cookie")
	r.Header.Set("Accept", "application/json")

	out, err := NewJSONLogFormatter().FormatRequestLog(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("credentials logged: %s", out)
	}
	if !strings.Contains(out, "REDACTED") || !strings.Contains(out, "application/json") {
		t.Errorf("unexpected header log: %s", out)
	}
	if r.Header.Get("Authorization") != "Bearer secret-token" {
		t.Error("redaction changed the request's own headers")
	}
}

// TestResponseBodyUsesRequestContext checks that the response side decides
// on debug details with the request's context, like the request side.
func TestResponseBodyUsesRequestContext(t *testing.T) {
	prev := slog.Default()
	w, flags := log.Writer(), log.Flags()
	t.Cleanup(func() { slog.SetDefault(prev); log.SetOutput(w); log.SetFlags(flags) })
	slog.SetDefault(slog.New(debugForKey{}))

	type key struct{}
	f := NewJSONLogFormatter()
	withDebug, err := f.FormatResponseLog(&ResponseInfo{Body: []byte("body"), Status: 200, ctx: context.WithValue(context.Background(), key{}, true)})
	if err != nil {
		t.Fatal(err)
	}
	without, err := f.FormatResponseLog(&ResponseInfo{Body: []byte("body"), Status: 200})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withDebug, `"response_body":"body"`) || strings.Contains(without, "response_body") {
		t.Errorf("with debug ctx: %s; without: %s", withDebug, without)
	}
}

// debugForKey enables debug only for contexts that carry any value, the way
// a per-request or trace-sampled debug handler would.
type debugForKey struct{ slog.Handler }

func (debugForKey) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= slog.LevelInfo || ctx != context.Background()
}

// nonSerializableBody is an io.ReadCloser that cannot be JSON-marshaled because
// it carries a func field, mirroring an otel-wrapped request body that surfaces
// `json: unsupported type: func(int64)`.
type nonSerializableBody struct {
	Notify func(int64) `json:"notify"`
}

func (nonSerializableBody) Read(p []byte) (int, error) { return 0, io.EOF }
func (nonSerializableBody) Close() error               { return nil }

// setDebug enables or disables slog debug for the duration of a test; the
// request/response formatters only include the header and body at debug level.
func setDebug(t *testing.T, on bool) {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	level := slog.LevelInfo
	if on {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: level})))
}

func TestFormatRequestLogPlaceholderOnNonSerializableBody(t *testing.T) {
	setDebug(t, true) // include header + body in the log record
	f := NewJSONLogFormatter()

	r := httptest.NewRequest("GET", "/api/rh-trex-ai/v1/dinosaurs", nil)
	r.Body = nonSerializableBody{}

	out, err := f.FormatRequestLog(r)
	if err != nil {
		t.Fatalf("FormatRequestLog returned error, expected a placeholder record: %v", err)
	}
	if !strings.Contains(out, `"request_method":"GET"`) {
		t.Errorf("record dropped request metadata: %s", out)
	}
	if !strings.Contains(out, `"request_url":"/api/rh-trex-ai/v1/dinosaurs"`) {
		t.Errorf("record dropped request_url: %s", out)
	}
	if !strings.Contains(out, "[unserializable:") {
		t.Errorf("expected placeholder for non-serializable body, got: %s", out)
	}
	if !strings.Contains(out, "func(int64)") {
		t.Errorf("expected the root marshal error to be preserved in the placeholder, got: %s", out)
	}
}

func TestFormatRequestLogSerializableBodyUnaffected(t *testing.T) {
	setDebug(t, false) // body omitted below debug level
	f := NewJSONLogFormatter()

	r := httptest.NewRequest("GET", "/healthz", nil)

	out, err := f.FormatRequestLog(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "[unserializable:") {
		t.Errorf("unexpected placeholder on the normal path: %s", out)
	}
	if !strings.Contains(out, `"request_url":"/healthz"`) {
		t.Errorf("missing request_url: %s", out)
	}
}
