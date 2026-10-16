// Package httpx holds the HTTP plumbing shared by every handler: the JSON
// response envelope, error codes, and middleware.
//
// Response contract (every JSON response goes through this package):
//
//	success: {"data": ...}
//	failure: {"error": {"code": "...", "message": "...", "details": ...}}
//
// "details" is optional. Error codes are a closed set defined below; add a
// code here only when a handler actually needs it.
package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/Kaezaen/ai-interview-platform-remake/internal/platform/logging"
)

// ErrorCode is a stable, machine-readable error identifier. Clients branch
// on the code; the message is for humans and may change.
type ErrorCode string

const (
	CodeNotFound         ErrorCode = "NOT_FOUND"           // 404
	CodeMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"  // 405
	CodeInternal         ErrorCode = "INTERNAL_ERROR"      // 500
	CodeUnavailable      ErrorCode = "SERVICE_UNAVAILABLE" // 503
)

type errorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details any       `json:"details,omitempty"`
}

// fallbackBody is sent when encoding the real response fails. It is a
// constant so it cannot fail to encode itself.
const fallbackBody = `{"error":{"code":"INTERNAL_ERROR","message":"Internal server error."}}`

// Data writes {"data": data} with the given status.
func Data(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeJSON(w, r, status, map[string]any{"data": data})
}

// Error writes {"error": {...}} with the given status. Use InternalError for
// unexpected failures so the cause gets logged.
func Error(w http.ResponseWriter, r *http.Request, status int, code ErrorCode, message string, details any) {
	writeJSON(w, r, status, map[string]errorBody{
		"error": {Code: code, Message: message, Details: details},
	})
}

// InternalError logs err with the request-scoped logger, then answers with a
// generic 500. The client never sees err: it may contain SQL, hostnames, or
// other internals. The request id in the response header links the client's
// report to this log line.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	logging.FromContext(r.Context()).Error("internal error",
		"err", err,
		"method", r.Method,
		"path", r.URL.Path,
	)
	Error(w, r, http.StatusInternalServerError, CodeInternal, "Internal server error.", nil)
}

// writeJSON encodes the body fully BEFORE writing the status line. If
// encoding fails we can still send a clean 500 instead of a 200 followed by
// a truncated body.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	buf, err := json.Marshal(body)
	if err != nil {
		logging.FromContext(r.Context()).Error("encode response body", "err", err)
		status, buf = http.StatusInternalServerError, []byte(fallbackBody)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// A write error means the client went away; there is nobody left to tell.
	_, _ = w.Write(buf)
}
