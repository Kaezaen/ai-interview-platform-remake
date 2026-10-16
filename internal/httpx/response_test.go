package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
)

func TestDataWritesEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	Data(w, r, http.StatusCreated, map[string]string{"id": "42"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	assertJSON(t, w.Body.Bytes(), `{"data":{"id":"42"}}`)
}

func TestErrorOmitsEmptyDetails(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	Error(w, r, http.StatusNotFound, CodeNotFound, "Not here.", nil)

	assertJSON(t, w.Body.Bytes(), `{"error":{"code":"NOT_FOUND","message":"Not here."}}`)
}

func TestInternalErrorLogsCauseButHidesItFromClient(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	r = r.WithContext(logging.WithLogger(r.Context(), logger))

	InternalError(w, r, errors.New("pq: relation \"secret_table\" does not exist"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret_table") {
		t.Errorf("response leaks the internal error: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "secret_table") {
		t.Errorf("internal error was not logged; logs: %s", logs.String())
	}
}

func TestUnencodableBodyBecomes500(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(logging.WithLogger(r.Context(), slog.New(slog.DiscardHandler)))

	// Channels cannot be encoded as JSON.
	Data(w, r, http.StatusOK, make(chan int))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	assertJSON(t, w.Body.Bytes(), fallbackBody)
}

// assertJSON compares JSON documents semantically, ignoring key order and
// whitespace.
func assertJSON(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("response is not valid JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad expectation %q: %v", want, err)
	}
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	if !bytes.Equal(gb, wb) {
		t.Errorf("body = %s, want %s", gb, wb)
	}
}
