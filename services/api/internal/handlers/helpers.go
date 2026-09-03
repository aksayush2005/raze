package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/aksayush2005/raze/services/api/internal/repositories"
)

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// readJSON decodes the request body into dst.
func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// writeError writes a JSON error payload.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// pathInt parses a named path parameter as int64.
func pathInt(r *http.Request, name string) (int64, bool) {
	v := r.PathValue(name)
	if v == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// mapStoreErr maps repository errors to HTTP statuses.
func mapStoreErr(w http.ResponseWriter, err error, notFoundMsg string) bool {
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundMsg)
		return true
	case errors.Is(err, repositories.ErrStaleVersion), errors.Is(err, repositories.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, err.Error())
		return true
	}
	return false
}
