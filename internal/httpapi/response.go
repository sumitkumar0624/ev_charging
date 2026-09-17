package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/example/evcharging/internal/domain"
)

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return false
	}
	return true
}

func respond(w http.ResponseWriter, value any, err error, successStatus int) {
	if err == nil {
		writeJSON(w, successStatus, value)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrConnectorInUse),
		errors.Is(err, domain.ErrSessionNotActive):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNoConnectorAvailable):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidPromo), errors.Is(err, domain.ErrUnsupportedConnector):
		status = http.StatusUnprocessableEntity
	}
	message := err.Error()
	if status == http.StatusInternalServerError {
		message = "internal server error"
	}
	writeError(w, status, message)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"status": status, "message": message,
	}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
