package handlers

import (
	"net/http"

	"github.com/mrtrkmn/orchi/api/models"
)

// AuthHandler handles authentication-related API endpoints.
type AuthHandler struct {
	signingKey []byte
}

// NewAuthHandler creates a new AuthHandler with the given JWT signing key.
func NewAuthHandler(signingKey []byte) *AuthHandler {
	return &AuthHandler{signingKey: signingKey}
}

// Register handles user registration.
//
// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Username, email, and password are required")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Password must be at least 8 characters")
		return
	}

	if len(req.Username) > 32 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Username must be at most 32 characters")
		return
	}

	// In production, this would call the Store gRPC service to create the user
	// and generate a JWT token. For now, return a structured response.
	writeJSON(w, http.StatusCreated, models.AuthResponse{
		User: models.UserInfo{
			ID:       "placeholder-uuid",
			Username: req.Username,
			Email:    req.Email,
			Role:     "participant",
		},
		Token:        "placeholder-token",
		RefreshToken: "placeholder-refresh-token",
		ExpiresAt:    "2026-01-15T11:00:00Z",
	})
}

// Login handles user authentication.
//
// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Email and password are required")
		return
	}

	// In production, this would:
	// 1. Call Store gRPC to fetch user by email
	// 2. Compare password hash with bcrypt
	// 3. Generate JWT access + refresh tokens
	// 4. Return user info with tokens
	writeJSON(w, http.StatusOK, models.AuthResponse{
		User: models.UserInfo{
			ID:       "placeholder-uuid",
			Username: "user",
			Email:    req.Email,
			Role:     "participant",
		},
		Token:        "placeholder-token",
		RefreshToken: "placeholder-refresh-token",
		ExpiresAt:    "2026-01-15T11:00:00Z",
	})
}

// Refresh handles token refresh.
//
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Refresh token is required")
		return
	}

	// In production, validate refresh token, rotate it, issue new tokens
	writeJSON(w, http.StatusOK, models.AuthResponse{
		Token:        "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresAt:    "2026-01-15T12:00:00Z",
	})
}

// Logout handles session termination.
//
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// In production, revoke the refresh token
	w.WriteHeader(http.StatusNoContent)
}
