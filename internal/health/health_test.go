package health

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestLiveDoesNotDependOnDatabase(t *testing.T) {
	h := NewHandler(fakePinger{err: errors.New("database is down")})
	w := httptest.NewRecorder()

	h.Live(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 even when the database is down", w.Code)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{"database reachable", nil, http.StatusOK, `"ready"`},
		{"database down", errors.New("connection refused"), http.StatusServiceUnavailable, `"SERVICE_UNAVAILABLE"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(fakePinger{err: tt.pingErr})
			r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			r = r.WithContext(logging.WithLogger(r.Context(), slog.New(slog.DiscardHandler)))
			w := httptest.NewRecorder()

			h.Ready(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body = %s, want it to contain %s", w.Body.String(), tt.wantBody)
			}
		})
	}
}
