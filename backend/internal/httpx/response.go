// Package httpx contains HTTP response helpers and middleware.
package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// WriteJSON serialises payload (which must encode to a JSON object) and appends
// "time_ns": the handler's execution time in nanoseconds, measured from start
// until just before the response is written.
func WriteJSON(w http.ResponseWriter, status int, start time.Time, payload any) {
	body, err := json.Marshal(payload)
	if err != nil || len(body) < 2 || body[0] != '{' {
		slog.Error("encode response", "err", err)
		body = []byte(`{"error":"internal server error"}`)
		status = http.StatusInternalServerError
	}

	elapsed := strconv.FormatInt(time.Since(start).Nanoseconds(), 10)

	var buf bytes.Buffer
	buf.Grow(len(body) + 32)
	buf.Write(body[:len(body)-1]) // drop closing '}'
	if !bytes.Equal(bytes.TrimSpace(body), []byte("{}")) {
		buf.WriteByte(',')
	}
	buf.WriteString(`"time_ns":"`)
	buf.WriteString(elapsed)
	buf.WriteString(`"}`)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// ErrorBody is the shape of every error response.
type ErrorBody struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, status int, start time.Time, msg string, details ...string) {
	WriteJSON(w, status, start, ErrorBody{Error: msg, Details: details})
}

// DecodeJSON strictly decodes a request body (unknown fields rejected, 1 MiB cap).
func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
