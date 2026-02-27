package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/mrtrkmn/orchi/api/models"
)

type contextKey string

const (
	// UserContextKey is the context key for the authenticated user claims.
	UserContextKey contextKey = "user"
)

// Claims represents the JWT claims structure used by the Orchi API.
type Claims struct {
	UserID  string `json:"sub"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	TeamID  string `json:"team_id,omitempty"`
	EventID string `json:"event_id,omitempty"`
	jwt.StandardClaims
}

// JWTAuth creates a middleware that validates JWT tokens from the Authorization header.
// It extracts the Bearer token, validates it against the provided signing key,
// and stores the claims in the request context.
func JWTAuth(signingKey []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid authorization header format")
				return
			}

			tokenString := parts[1]
			claims := &Claims{}

			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return signingKey, nil
			})

			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims extracts the JWT claims from the request context.
func GetClaims(r *http.Request) *Claims {
	claims, ok := r.Context().Value(UserContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// RequireRole creates a middleware that checks if the authenticated user has
// one of the specified roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r)
			if claims == nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		})
	}
}

// CORS creates a middleware that handles Cross-Origin Resource Sharing headers.
// It allows requests from the specified origins and handles preflight OPTIONS requests.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originsSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originsSet[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if originsSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter tracks request counts per key within a time window.
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter with the specified limit and window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks whether a request from the given key should be allowed.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Clean old entries
	var valid []time.Time
	for _, t := range rl.requests[key] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.requests[key] = valid

	if len(valid) >= rl.limit {
		return false
	}

	rl.requests[key] = append(rl.requests[key], now)
	return true
}

// RateLimit creates a middleware that limits requests based on client IP.
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				key = strings.Split(forwarded, ",")[0]
			}

			if !limiter.Allow(strings.TrimSpace(key)) {
				writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds security-related HTTP headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// RequestID generates a unique request ID and adds it to the response headers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r)
	})
}

func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.APIError{
		Error: models.ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
