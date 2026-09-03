// Package middleware provides HTTP middleware for the control plane.
package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"

	"github.com/aksayush2005/raze/services/api/internal/repositories"
)

// recordingWriter captures the status and body of the wrapped handler.
type recordingWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rw *recordingWriter) WriteHeader(c int) {
	rw.status = c
	rw.ResponseWriter.WriteHeader(c)
}

func (rw *recordingWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// Idempotent makes a POST handler safe to retry. When an Idempotency-Key header
// is present, repeated identical requests return the original response and a
// reused key with a different payload is rejected with 409.
func Idempotent(store *repositories.Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		sum := fmt.Sprintf("%x", sha256.Sum256(body))

		if resp, storedHash, found, gerr := store.GetIdempotency(r.Context(), key); gerr == nil && found {
			if storedHash != sum {
				writeError(w, http.StatusConflict, "idempotency key reused with a different payload")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(resp)
			return
		}

		rw := &recordingWriter{ResponseWriter: w, status: http.StatusOK}
		next(rw, r)
		if rw.status >= 200 && rw.status < 300 {
			_ = store.SetIdempotency(r.Context(), key, sum, rw.body.Bytes())
		}
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":` + fmt.Sprintf("%q", msg) + `}`))
}
