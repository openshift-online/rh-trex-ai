package logging

import (
	"bytes"
	"context"
	"encoding/json"
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

// TestDebugRequestLogWithWrappedBody formats a request whose body is wrapped
// the way otelhttp wraps it. The wrapper has an exported func field, which
// json.Marshal cannot encode.
func TestDebugRequestLogWithWrappedBody(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug})))

	const body = `{"password":"secret-value"}`
	r := httptest.NewRequest("POST", "/api/x", strings.NewReader(body))
	r.Body = &funcFieldBody{ReadCloser: r.Body, OnRead: func(int64) {}}

	out, err := NewJSONLogFormatter().FormatRequestLog(r)
	if err != nil {
		t.Fatalf("format failed: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(out), &entry); err != nil {
		t.Fatalf("log entry is not valid JSON: %v: %s", err, out)
	}
	if strings.Contains(out, "secret") {
		t.Errorf("body content logged: %s", out)
	}
	if got := entry["request_content_length"]; got != float64(len(body)) {
		t.Errorf("request_content_length = %v, want %d: %s", got, len(body), out)
	}
	read, err := io.ReadAll(r.Body)
	if err != nil || string(read) != body {
		t.Errorf("handler read %q, %v; want the full body", read, err)
	}
}

// funcFieldBody mimics otelhttp's request.BodyWrapper.
type funcFieldBody struct {
	io.ReadCloser
	OnRead func(int64)
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
