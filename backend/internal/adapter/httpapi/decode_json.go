package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	defaultJSONBodyLimit = 64 << 10 // 64 KiB — auth, settings, hunts, tokens
	smallJSONBodyLimit   = 16 << 10 // 16 KiB — anomaly ack/assign
	largeJSONBodyLimit   = 1 << 20  // 1 MiB — TLS PEM upload
)

// decodeJSONBody reads a single JSON object from r.Body with size cap, rejects
// unknown fields (mass-assignment guard), and rejects trailing JSON.
// On failure it writes 400/413 and returns false.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	if maxBytes <= 0 {
		maxBytes = defaultJSONBodyLimit
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeDomainError(w, "json body too large", maxErr)
			return false
		}
		writeBadRequest(w, "invalid json")
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeBadRequest(w, "invalid json")
		return false
	}
	return true
}
