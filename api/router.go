package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/mrtrkmn/orchi/api/handlers"
	"github.com/mrtrkmn/orchi/api/middleware"
)

// Config holds the configuration for the API gateway.
type Config struct {
	// SigningKey is the JWT HMAC signing key.
	SigningKey []byte

	// AllowedOrigins is the list of CORS allowed origins.
	AllowedOrigins []string

	// RateLimitPerMinute is the default rate limit per IP per minute.
	RateLimitPerMinute int
}

// NewRouter creates the API gateway HTTP handler with all routes, middleware,
// and handlers configured.
//
// Route structure:
//
//	/healthz              - Health check (public)
//	/readyz               - Readiness check (public)
//	/api/v1/auth/*        - Authentication endpoints (public)
//	/api/v1/events/*      - Event endpoints (authenticated)
//	/api/v1/teams/*       - Team endpoints (authenticated)
//	/api/v1/flags/*       - Flag verification (authenticated)
//	/api/v1/admin/*       - Admin endpoints (admin role)
//	/api/v1/ws/*          - WebSocket endpoints (authenticated)
func NewRouter(cfg Config) http.Handler {
	mux := http.NewServeMux()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg.SigningKey)
	eventHandler := handlers.NewEventHandler()
	challengeHandler := handlers.NewChallengeHandler()
	teamHandler := handlers.NewTeamHandler()
	scoreboardHandler := handlers.NewScoreboardHandler()
	labHandler := handlers.NewLabHandler()

	// Rate limiters
	authLimiter := middleware.NewRateLimiter(10, time.Minute)
	apiLimiter := middleware.NewRateLimiter(
		cfg.RateLimitPerMinute,
		time.Minute,
	)

	// Health checks (no auth, no CORS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// Public auth endpoints (rate limited, CORS)
	mux.Handle("/api/v1/auth/register", chain(
		http.HandlerFunc(authHandler.Register),
		middleware.RateLimit(authLimiter),
		methodFilter("POST"),
	))
	mux.Handle("/api/v1/auth/login", chain(
		http.HandlerFunc(authHandler.Login),
		middleware.RateLimit(authLimiter),
		methodFilter("POST"),
	))
	mux.Handle("/api/v1/auth/refresh", chain(
		http.HandlerFunc(authHandler.Refresh),
		middleware.RateLimit(authLimiter),
		methodFilter("POST"),
	))

	// Authenticated endpoints
	jwtMiddleware := middleware.JWTAuth(cfg.SigningKey)

	mux.Handle("/api/v1/auth/logout", chain(
		http.HandlerFunc(authHandler.Logout),
		jwtMiddleware,
		methodFilter("POST"),
	))

	// Events
	mux.Handle("/api/v1/events", chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case "GET":
				eventHandler.List(w, r)
			case "POST":
				eventHandler.Create(w, r)
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}),
		jwtMiddleware,
		middleware.RateLimit(apiLimiter),
	))

	// Challenges and related event sub-routes
	mux.Handle("/api/v1/events/", chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/challenges"):
				if r.Method == http.MethodGet {
					challengeHandler.List(w, r)
				} else {
					w.Header().Set("Allow", "GET")
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case strings.HasSuffix(path, "/teams"):
				switch r.Method {
				case http.MethodPost:
					teamHandler.Create(w, r)
				case http.MethodGet:
					teamHandler.List(w, r)
				default:
					w.Header().Set("Allow", "GET, POST")
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			case strings.HasSuffix(path, "/scoreboard"):
				if r.Method == http.MethodGet {
					scoreboardHandler.Get(w, r)
				} else {
					w.Header().Set("Allow", "GET")
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			default:
				if r.Method == http.MethodGet {
					eventHandler.Get(w, r)
				} else {
					w.Header().Set("Allow", "GET")
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}
		}),
		jwtMiddleware,
		middleware.RateLimit(apiLimiter),
	))

	// Flag verification
	mux.Handle("/api/v1/flags/verify", chain(
		http.HandlerFunc(challengeHandler.VerifyFlag),
		jwtMiddleware,
		middleware.RateLimit(middleware.NewRateLimiter(30, time.Minute)),
		methodFilter("POST"),
	))

	// Teams
	mux.Handle("/api/v1/teams/", chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case strings.HasSuffix(path, "/lab"):
				labHandler.Get(w, r)
			case strings.HasSuffix(path, "/lab/reset"):
				labHandler.Reset(w, r)
			case strings.Contains(path, "/lab/exercises/") && strings.HasSuffix(path, "/reset"):
				labHandler.ResetExercise(w, r)
			case strings.HasSuffix(path, "/vpn/config"):
				labHandler.DownloadVPNConfig(w, r)
			default:
				teamHandler.Get(w, r)
			}
		}),
		jwtMiddleware,
		middleware.RateLimit(apiLimiter),
	))

	// Apply global middleware
	var handler http.Handler = mux
	handler = middleware.CORS(cfg.AllowedOrigins)(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.RequestID(handler)

	return handler
}

// chain applies middleware in reverse order so the first middleware listed
// is the outermost (executed first).
func chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// methodFilter creates a middleware that only allows the specified HTTP method.
func methodFilter(method string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method && r.Method != "OPTIONS" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
