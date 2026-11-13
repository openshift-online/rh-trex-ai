package audit

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

// Recorder stores or ships audit events. TRex captures events; a Recorder
// decides where they go. An environment chooses one with OverrideAudit.
//
// Record is called synchronously after a request completes, with a context
// that is not canceled with the request and has a short deadline. It must be
// safe for concurrent use. A returned error is logged and counted; it does
// not change the response.
type Recorder interface {
	Record(ctx context.Context, e Event) error
}

// DefaultRecorder is the recorder TRex's own environments use: JSON lines
// written to the destination selected by --audit-sink.
func DefaultRecorder(cfg *config.AuditConfig) (Recorder, error) {
	if path, ok := cfg.SinkFile(); ok {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("--audit-sink: opening %s: %w", path, err)
		}
		return &JSONRecorder{w: f, closer: f}, nil
	}
	return NewJSONRecorder(os.Stdout), nil
}

// JSONRecorder writes each event as one JSON line (schema version 1).
type JSONRecorder struct {
	mu     sync.Mutex
	w      io.Writer
	closer io.Closer
}

// NewJSONRecorder writes JSON lines to w.
func NewJSONRecorder(w io.Writer) *JSONRecorder { return &JSONRecorder{w: w} }

func (r *JSONRecorder) Record(_ context.Context, e Event) error {
	line, err := MarshalLine(e)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, err = r.w.Write(line)
	return err
}

// Close closes the destination when the recorder owns it.
func (r *JSONRecorder) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// MemoryRecorder keeps events in memory. It is meant for tests.
type MemoryRecorder struct {
	mu     sync.Mutex
	events []Event
}

func NewMemoryRecorder() *MemoryRecorder { return &MemoryRecorder{} }

func (m *MemoryRecorder) Record(_ context.Context, e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// Events returns a copy of the recorded events, oldest first.
func (m *MemoryRecorder) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Event(nil), m.events...)
}

// Reset forgets every recorded event.
func (m *MemoryRecorder) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}
