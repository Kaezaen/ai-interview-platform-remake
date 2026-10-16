package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

func newTestHandler() http.Handler {
	return NewHandler(Deps{Logger: slog.New(slog.DiscardHandler), DB: okPinger{}})
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string // error code in the envelope; empty for success
		wantAllow  string
	}{
		{"liveness", http.MethodGet, "/healthz", http.StatusOK, "", ""},
		{"readiness", http.MethodGet, "/readyz", http.StatusOK, "", ""},
		{"HEAD is served by the GET route", http.MethodHead, "/healthz", http.StatusOK, "", ""},
		{"unknown path", http.MethodGet, "/nope", http.StatusNotFound, "NOT_FOUND", ""},
		{"known path, wrong method", http.MethodPost, "/healthz", http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET, HEAD"},
	}

	handler := newTestHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Error("X-Request-ID header missing")
			}
			if tt.wantAllow != "" && w.Header().Get("Allow") != tt.wantAllow {
				t.Errorf("Allow = %q, want %q", w.Header().Get("Allow"), tt.wantAllow)
			}
			if tt.wantCode == "" || tt.method == http.MethodHead {
				return
			}

			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("error response is not JSON: %v\n%s", err, w.Body.String())
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
