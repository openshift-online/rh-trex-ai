package db

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/felixge/httpsnoop"

	svcerrors "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
)

// MaxBufferedResponseBytes is how much of a response TransactionMiddleware
// holds back while the request transaction is still open. A larger response
// commits the transaction before its first byte is sent.
const MaxBufferedResponseBytes = 1 << 20

// bufferPool reuses response buffers across requests, so holding the response
// back costs a copy, not an allocation and a garbage collection per request.
var bufferPool = sync.Pool{New: func() any { b := make([]byte, 0, 64<<10); return &b }}

func getBuffer() []byte { return (*bufferPool.Get().(*[]byte))[:0] }

// putBuffer returns a buffer to the pool. Buffers that grew past the response
// limit are dropped, so the pool never pins large allocations.
func putBuffer(b []byte) {
	if b == nil || cap(b) > MaxBufferedResponseBytes+MaxBufferedResponseBytes/4 {
		return
	}
	b = b[:0]
	bufferPool.Put(&b)
}

// errCommitFailed is returned to a handler that keeps writing after its
// transaction failed to commit and the error response was sent instead.
var errCommitFailed = errors.New("db: request transaction failed to commit; response replaced")

// TransactionMiddleware creates a new HTTP middleware that begins a database transaction
// and stores it in the request context.
//
// The transaction is rolled back when the handler responds with a 4xx or 5xx status,
// or panics. Otherwise it is committed, unless something called MarkForRollback.
//
// The transaction is resolved before any byte of the response is sent, so a
// client is never told "success" for a change that did not commit. The
// response is buffered until the handler returns. A handler that flushes,
// hijacks the connection, uses io.ReaderFrom, or writes more than
// MaxBufferedResponseBytes resolves the transaction at that moment instead;
// statements it runs afterwards are no longer part of the request transaction.
// When the commit fails, the buffered response is discarded and the client
// receives 500 Internal Server Error.
func TransactionMiddleware(next http.Handler, connection SessionFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a new Context with the transaction stored in it. Make sure
		// it carries a TxResult, so the commit outcome is known even when no
		// outer hook (such as audit) asked for it.
		base := r.Context()
		if TxResultFromContext(base) == nil {
			base, _ = WithTxResult(base)
		}
		ctx, err := NewContext(base, connection)
		if err != nil {
			slog.ErrorContext(ctx, "Could not create transaction", "error", err)
			writeInternalError(ctx, w)
			return
		}

		// Set the value of the request pointer to the value of a new copy of the request with the new context key,vale
		// stored in it
		*r = *r.WithContext(ctx)

		cw := &commitFirstWriter{w: w, req: r, headerBefore: w.Header().Clone()}

		// Resolve the transaction when the handler returns or panics.
		defer func() {
			if p := recover(); p != nil {
				if !cw.resolved {
					MarkForRollback(r.Context(), fmt.Errorf("panic: %v", p))
					cw.resolved = true
					Resolve(r.Context())
				}
				panic(p)
			}
			cw.finish()
		}()

		// Continue handling requests.
		next.ServeHTTP(cw.wrap(), r)
	})
}

// commitFirstWriter holds the response back until the request transaction is
// resolved.
type commitFirstWriter struct {
	w            http.ResponseWriter
	req          *http.Request
	headerBefore http.Header

	status   int
	buf      []byte
	resolved bool // the transaction was committed or rolled back
	failed   bool // the commit failed and the error response was sent
}

// wrap returns the writer handed to the handler. httpsnoop keeps exactly the
// optional interfaces (Flusher, Hijacker, ReaderFrom, ...) of the original.
func (c *commitFirstWriter) wrap() http.ResponseWriter {
	return httpsnoop.Wrap(c.w, httpsnoop.Hooks{
		WriteHeader: func(httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
			return func(code int) {
				if c.resolved {
					if !c.failed {
						c.w.WriteHeader(code)
					}
					return
				}
				if c.status == 0 {
					c.status = code
				}
			}
		},
		Write: func(httpsnoop.WriteFunc) httpsnoop.WriteFunc {
			return func(b []byte) (int, error) {
				if c.resolved {
					if c.failed {
						return 0, errCommitFailed
					}
					return c.w.Write(b)
				}
				if c.status == 0 {
					c.status = http.StatusOK
				}
				if c.buf == nil {
					c.buf = getBuffer()
				}
				c.buf = append(c.buf, b...)
				if len(c.buf) > MaxBufferedResponseBytes && !c.release() {
					return 0, errCommitFailed
				}
				return len(b), nil
			}
		},
		Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
			return func() {
				if c.release() {
					next()
				}
			}
		},
		Hijack: func(next httpsnoop.HijackFunc) httpsnoop.HijackFunc {
			return func() (net.Conn, *bufio.ReadWriter, error) {
				if !c.resolve() {
					return nil, nil, errCommitFailed
				}
				return next()
			}
		},
		ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
			return func(src io.Reader) (int64, error) {
				if !c.release() {
					return 0, errCommitFailed
				}
				return next(src)
			}
		},
	})
}

// resolve commits or rolls back the request transaction once. It reports
// false when the commit failed; the caller must then send nothing but the
// error response.
func (c *commitFirstWriter) resolve() bool {
	if c.resolved {
		return !c.failed
	}
	c.resolved = true
	ctx := c.req.Context()
	if c.status >= http.StatusBadRequest {
		MarkForRollback(ctx, fmt.Errorf("HTTP status %d", c.status))
	}
	if ResolveErr(ctx) != nil && TxResultFromContext(ctx).Outcome == TxCommitFailed {
		c.failed = true
		putBuffer(c.buf)
		c.buf = nil
		// Drop what the handler set for the success response (Location,
		// ETag, ...) before sending the error.
		h := c.w.Header()
		for k := range h {
			delete(h, k)
		}
		for k, v := range c.headerBefore {
			h[k] = v
		}
		writeInternalError(ctx, c.w)
		return false
	}
	return true
}

// release resolves the transaction and sends what is buffered, so the
// response can continue unbuffered. It reports false when the commit failed.
func (c *commitFirstWriter) release() bool {
	wasResolved := c.resolved
	if !c.resolve() {
		return false
	}
	if !wasResolved {
		c.sendBuffered()
	}
	return true
}

// finish runs after the handler returns.
func (c *commitFirstWriter) finish() {
	if c.resolved {
		return
	}
	if c.resolve() {
		c.sendBuffered()
	}
}

func (c *commitFirstWriter) sendBuffered() {
	if c.status != 0 {
		c.w.WriteHeader(c.status)
	}
	if len(c.buf) > 0 {
		if _, err := c.w.Write(c.buf); err != nil {
			slog.ErrorContext(c.req.Context(), "Could not write response after commit", "error", err)
		}
	}
	putBuffer(c.buf)
	c.buf = nil
}

func writeInternalError(ctx context.Context, w http.ResponseWriter) {
	// Use the default error to avoid exposing internals to users.
	svcErr := svcerrors.GeneralError("")
	writeJSONResponse(w, svcErr.HttpCode, svcErr.AsOpenapiError(logger.GetOperationID(ctx)))
}

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}
