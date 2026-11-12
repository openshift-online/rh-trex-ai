package audit

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

// Sink receives exported audit lines. Each call to WriteLines gets complete
// JSON lines, each ending in a newline.
type Sink interface {
	WriteLines(lines [][]byte) error
	Close() error
}

// OpenSink opens the sink selected by --audit-sink. It returns nil for "none".
func OpenSink(cfg *config.AuditConfig) (Sink, error) {
	if path, ok := cfg.SinkFile(); ok {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, fmt.Errorf("--audit-sink: opening %s: %w", path, err)
		}
		return &writerSink{w: f, closer: f}, nil
	}
	switch cfg.Sink {
	case config.AuditSinkNone:
		return nil, nil
	case config.AuditSinkStdout, "":
		return &writerSink{w: os.Stdout}, nil
	}
	return nil, fmt.Errorf("--audit-sink: unknown sink %q", cfg.Sink)
}

// NewWriterSink wraps an io.Writer. It is meant for tests and custom programs.
func NewWriterSink(w io.Writer) Sink { return &writerSink{w: w} }

type writerSink struct {
	mu     sync.Mutex
	w      io.Writer
	closer io.Closer
}

func (s *writerSink) WriteLines(lines [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range lines {
		if _, err := s.w.Write(l); err != nil {
			return err
		}
	}
	if f, ok := s.w.(*os.File); ok && s.closer != nil {
		return f.Sync()
	}
	return nil
}

func (s *writerSink) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}
