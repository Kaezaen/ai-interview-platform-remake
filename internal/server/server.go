// Package server wires routes and middleware into an *http.Server.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/health"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/httpx"
)

// Deps are the collaborators the HTTP layer needs. New dependencies are
// added here explicitly instead of being reached through globals.
type Deps struct {
	Logger *slog.Logger
	DB     health.Pinger
}

// NewHandler builds the routing table and wraps it in middleware.
func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()

	h := health.NewHandler(d.DB)
	mux.HandleFunc("GET /healthz", h.Live)
	mux.HandleFunc("GET /readyz", h.Ready)

	// Middleware, outermost first:
	//   RequestID — every later log line carries the request id.
	//   AccessLog — measures the whole request, including recovered panics.
	//   Recover   — turns a panic into a logged JSON 500.
	handler := jsonFallback(mux)
	handler = httpx.Recover(handler)
	handler = httpx.AccessLog(handler)
	handler = httpx.RequestID(d.Logger)(handler)
	return handler
}

// New returns an *http.Server with explicit timeouts. The zero values mean
// "no timeout", which lets a slow or malicious client hold a connection open
// forever.
func New(addr string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
		// Time allowed to send request headers. Protects against Slowloris.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Streaming (SSE) handlers will extend their own deadline with
		// http.ResponseController instead of raising this for everyone.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		// Errors net/http logs itself (TLS handshakes, bad requests) go
		// through our logger too.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}
}

// jsonFallback makes unmatched requests answer with the JSON error envelope
// instead of ServeMux's plain-text 404/405, so clients can rely on a single
// error format.
func jsonFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		// No route matched. The mux knows whether the path exists with other
		// methods (405) or not at all (404), so let it answer into a probe,
		// then re-emit that answer as JSON.
		probe := &statusProbe{header: http.Header{}}
		mux.ServeHTTP(probe, r)

		if probe.status == http.StatusMethodNotAllowed {
			w.Header().Set("Allow", probe.header.Get("Allow"))
			httpx.Error(w, r, http.StatusMethodNotAllowed, httpx.CodeMethodNotAllowed, "Method not allowed.", nil)
			return
		}
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeNotFound, "Route not found.", nil)
	})
}

// statusProbe is a ResponseWriter that only records the status and headers.
type statusProbe struct {
	header http.Header
	status int
}

func (p *statusProbe) Header() http.Header { return p.header }

func (p *statusProbe) WriteHeader(code int) {
	if p.status == 0 {
		p.status = code
	}
}

func (p *statusProbe) Write(b []byte) (int, error) {
	if p.status == 0 {
		p.status = http.StatusOK
	}
	return len(b), nil
}
