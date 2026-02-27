package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mrtrkmn/orchi/api/middleware"
	"github.com/mrtrkmn/orchi/api/models"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, models.APIError{
		Error: models.ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// decodeJSON decodes a JSON request body into the given struct.
func decodeJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// getClaims extracts JWT claims from the request context.
func getClaims(r *http.Request) *middleware.Claims {
	return middleware.GetClaims(r)
}
