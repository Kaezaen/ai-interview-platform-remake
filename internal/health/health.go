// Package health exposes liveness and readiness endpoints.
//
// Liveness (/healthz) answers "is the process alive?" and never touches
// dependencies: restarting the process would not fix a database outage, so
// a failing database must not make the orchestrator kill healthy processes.
//
// Readiness (/readyz) answers "can this instance serve traffic right now?"
// and checks the database, so a load balancer can stop routing to it.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/httpx"
	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
)

// Pinger is the one capability readiness needs. *pgxpool.Pool satisfies it;
// tests pass a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves the health endpoints.
type Handler struct {
	db      Pinger
	timeout time.Duration
}

// NewHandler returns a Handler that checks db for readiness.
func NewHandler(db Pinger) *Handler {
	return &Handler{db: db, timeout: 2 * time.Second}
}

type status struct {
	Status string `json:"status"`
}

// Live handles GET /healthz.
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	httpx.Data(w, r, http.StatusOK, status{Status: "ok"})
}

// Ready handles GET /readyz.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		logging.FromContext(r.Context()).Warn("readiness check failed", "err", err)
		httpx.Error(w, r, http.StatusServiceUnavailable, httpx.CodeUnavailable, "Database is not reachable.", nil)
		return
	}
	httpx.Data(w, r, http.StatusOK, status{Status: "ready"})
}
