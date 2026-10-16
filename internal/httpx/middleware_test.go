package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
)

func TestRequestIDGeneratesAndPropagates(t *testing.T) {
	var logs bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&logs, nil))

	handler := RequestID(base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.FromContext(r.Context()).Info("inside handler")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	id := w.Header().Get(RequestIDHeader)
	if len(id) != 32 {
		t.Fatalf("generated id = %q, want 32 hex characters", id)
	}
	if !strings.Contains(logs.String(), `"request_id":"`+id+`"`) {
		t.Errorf("log line does not carry the request id %s:\n%s", id, logs.String())
	}
}

func TestRequestIDTrustsOnlySafeIncomingIDs(t *testing.T) {
	handler := RequestID(slog.New(slog.DiscardHandler))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	tests := []struct {
		name     string
		incoming string
		keep     bool
	}{
		{"proxy id is kept", "abc-123_DEF.9", true},
		{"newline is replaced", "abc\nlevel=ERROR msg=forged", false},
		{"too long is replaced", strings.Repeat("a", 65), false},
		{"empty is replaced", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(RequestIDHeader, tt.incoming)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			got := w.Header().Get(RequestIDHeader)
			if kept := got == tt.incoming; kept != tt.keep {
				t.Errorf("incoming %q -> %q; kept = %v, want %v", tt.incoming, got, kept, tt.keep)
			}
		})
	}
}

func TestRecoverTurnsPanicInto500(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	handler := RequestID(logger)(Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	assertJSON(t, w.Body.Bytes(), `{"error":{"code":"INTERNAL_ERROR","message":"Internal server error."}}`)
	if !strings.Contains(logs.String(), "boom") {
		t.Errorf("panic value was not logged:\n%s", logs.String())
	}
}

func TestRecoverDoesNotRewriteAStartedResponse(t *testing.T) {
	handler := Recover(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("after headers")
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(logging.WithLogger(r.Context(), slog.New(slog.DiscardHandler)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want the already-sent 202", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body = %q, want nothing appended after headers were sent", w.Body.String())
	}
}

func TestAccessLogRecordsStatusRouteAndBytes(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("12345"))
	})
	handler := RequestID(logger)(AccessLog(mux))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/7", nil))

	var entry map[string]any
	for line := range strings.Lines(logs.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err == nil && m["msg"] == "http request" {
			entry = m
		}
	}
	if entry == nil {
		t.Fatalf("no access log line found:\n%s", logs.String())
	}
	if entry["status"] != float64(http.StatusTeapot) || entry["bytes"] != float64(5) ||
		entry["route"] != "GET /items/{id}" || entry["path"] != "/items/7" {
		t.Errorf("unexpected access log entry: %v", entry)
	}
}

func TestStatusRecorderSupportsResponseController(t *testing.T) {
	// SSE handlers call http.NewResponseController(w).Flush(); that only
	// works through our wrapper if Unwrap reaches a real Flusher.
	handler := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through statusRecorder: %v", err)
		}
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(logging.WithLogger(r.Context(), slog.New(slog.DiscardHandler)))
	handler.ServeHTTP(httptest.NewRecorder(), r)
}
