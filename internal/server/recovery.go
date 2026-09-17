// The server's top-level recovery layer (ADR-0041 clause 6).
//
// panic is reserved for programmer error, and no path reachable from
// repository content or a request may take one. The layer here is what makes a
// mistake in that rule survivable rather than fatal to a long-running server:
// a recovered panic is reported as an internal error, never as a user error —
// the operator did nothing wrong — and never as a success.
//
// "Never as a success" needs one distinction. A panic before anything is
// written becomes a 500 through the same mapping every other error uses. A
// panic after a status has gone out cannot be turned into a 500, because the
// status has already been sent; the connection is aborted instead, so the
// client sees a truncated response rather than a complete one it would read as
// successful.

package server

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/sinanganiz/commitography/internal/core"
)

// recovered turns a recovered panic value into an internal error whose
// wrapping context locates the origin (ADR-0041 clause 3). The stack goes in
// the cause, so it reaches a diagnostic and never an API response, whose
// rendering of an internal error is one fixed sentence.
func recovered(value any, doing string) error {
	return core.Internalf(fmt.Errorf("%v\n%s", value, debug.Stack()), "recovered a panic while %s", doing)
}

// recordingWriter remembers whether a response has begun, which is what decides
// between reporting a panic and aborting the connection.
type recordingWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *recordingWriter) WriteHeader(status int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}

// Flush passes a flush through, so a streaming response keeps working behind
// the layer.
func (w *recordingWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// withRecovery wraps a handler in the recovery layer. It is applied once, at
// the outermost point, so every route is behind it.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &recordingWriter{ResponseWriter: w}
		defer func() {
			value := recover()
			if value == nil {
				return
			}
			// http.ErrAbortHandler is the standard library's own way of
			// dropping a connection on purpose. It is not a defect and is
			// passed on untouched.
			if value == http.ErrAbortHandler {
				panic(value)
			}
			if recorder.wrote {
				panic(http.ErrAbortHandler)
			}
			writeAPIError(recorder, recovered(value, "serving a request"))
		}()
		next.ServeHTTP(recorder, r)
	})
}
