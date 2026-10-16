package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
)

// RequestIDHeader carries the request id in both directions.
const RequestIDHeader = "X-Request-ID"

// validRequestID limits which incoming ids we trust. A reverse proxy may set
// the header, but so can any client, and a value with newlines or quotes
// could forge log lines.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// RequestID gives every request an id, echoes it in the response header,
// and stores a logger tagged with that id in the request context. Every log
// line written through logging.FromContext then carries the same id.
func RequestID(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(RequestIDHeader)
			if !validRequestID.MatchString(id) {
				id = newRequestID()
			}

			w.Header().Set(RequestIDHeader, id)
			ctx := logging.WithLogger(r.Context(), base.With("request_id", id))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// newRequestID returns 128 random bits as hex. Since Go 1.24 crypto/rand.Read
// never returns an error (it crashes the program if the OS source fails).
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// AccessLog writes one log line per request after the handler finishes.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := record(w)

		next.ServeHTTP(rec, r)

		logging.FromContext(r.Context()).Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			// r.Pattern is filled in by ServeMux while routing; empty means
			// no route matched.
			"route", r.Pattern,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
	})
}

// Recover turns a panic in a handler into a logged error and, if nothing was
// sent yet, a JSON 500. Without it, net/http would recover the panic itself,
// log it without our request id, and drop the connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := record(w)
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// http.ErrAbortHandler is net/http's documented way to abort a
			// response on purpose; let it through untouched.
			if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(v)
			}

			logging.FromContext(r.Context()).Error("panic recovered",
				"panic", v,
				"stack", string(debug.Stack()),
			)
			if !rec.wroteHeader {
				Error(rec, r, http.StatusInternalServerError, CodeInternal, "Internal server error.", nil)
			}
		}()

		next.ServeHTTP(rec, r)
	})
}

// statusRecorder wraps a ResponseWriter to remember the status code and the
// number of body bytes written.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

// record wraps w, reusing an existing recorder so stacked middleware share
// one view of the response.
func record(w http.ResponseWriter) *statusRecorder {
	if rec, ok := w.(*statusRecorder); ok {
		return rec
	}
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the underlying writer. Streaming
// handlers (SSE) need it to call Flush and extend write deadlines through
// this wrapper.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}
