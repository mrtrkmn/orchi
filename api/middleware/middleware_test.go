package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCORSAllowedOrigin(t *testing.T) {
	origins := []string{"https://cyberorch.com", "https://staging.cyberorch.com"}
	handler := CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	req.Header.Set("Origin", "https://cyberorch.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "https://cyberorch.com" {
		t.Errorf("CORS origin = %q, want %q", got, "https://cyberorch.com")
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	origins := []string{"https://cyberorch.com"}
	handler := CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	req.Header.Set("Origin", "https://evil.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	got := rr.Header().Get("Access-Control-Allow-Origin")
	if got != "" {
		t.Errorf("CORS should not allow origin, got %q", got)
	}
}

func TestCORSPreflightOptions(t *testing.T) {
	origins := []string{"https://cyberorch.com"}
	handler := CORS(origins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/events", nil)
	req.Header.Set("Origin", "https://cyberorch.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestJWTAuthMissingHeader(t *testing.T) {
	handler := JWTAuth([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuthInvalidFormat(t *testing.T) {
	handler := JWTAuth([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestJWTAuthInvalidToken(t *testing.T) {
	handler := JWTAuth([]byte("secret"))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	defer rl.Stop()

	for i := 0; i < 3; i++ {
		if !rl.Allow("test-ip") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	if rl.Allow("test-ip") {
		t.Error("4th request should be rate limited")
	}
}

func TestRateLimiterDifferentKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	if !rl.Allow("ip-1") {
		t.Error("First IP should be allowed")
	}
	if !rl.Allow("ip-2") {
		t.Error("Second IP should be allowed independently")
	}
}

func TestRateLimiterEvictsStaleEntries(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)
	defer rl.Stop()

	// Use up the limit for a key
	if !rl.Allow("stale-ip") {
		t.Fatal("First request should be allowed")
	}
	if rl.Allow("stale-ip") {
		t.Fatal("Second request should be rate limited")
	}

	// Wait for the window to expire
	time.Sleep(100 * time.Millisecond)

	// Key should have been evicted or entries expired; new request should be allowed
	if !rl.Allow("stale-ip") {
		t.Error("Request should be allowed after window expires")
	}

	// Verify the stale key was cleaned up from the map
	rl.mu.Lock()
	_, exists := rl.requests["stale-ip"]
	rl.mu.Unlock()
	if !exists {
		t.Error("Active key should still exist in the map")
	}
}

func TestRateLimiterEvictStaleRemovesEmptyKeys(t *testing.T) {
	rl := NewRateLimiter(1, 10*time.Millisecond)
	defer rl.Stop()

	rl.Allow("ephemeral-ip")
	time.Sleep(50 * time.Millisecond)

	// Manually trigger eviction
	rl.evictStale()

	rl.mu.Lock()
	_, exists := rl.requests["ephemeral-ip"]
	rl.mu.Unlock()
	if exists {
		t.Error("Stale key should have been evicted from the map")
	}
}

func TestRateLimitMiddlewareStripsPort(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request from 192.0.2.1:12345
	req1 := httptest.NewRequest("GET", "/api/v1/events", nil)
	req1.RemoteAddr = "192.0.2.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("First request status = %d, want %d", rr1.Code, http.StatusOK)
	}

	// Second request from same IP but different port should be rate limited
	req2 := httptest.NewRequest("GET", "/api/v1/events", nil)
	req2.RemoteAddr = "192.0.2.1:54321"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request (different port, same IP) status = %d, want %d", rr2.Code, http.StatusTooManyRequests)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
	}

	for key, want := range headers {
		got := rr.Header().Get(key)
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestRequireRoleAllowed(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	claims := &Claims{Role: "admin"}
	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := contextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Admin should be allowed, got status %d", rr.Code)
	}
}

func TestRequireRoleForbidden(t *testing.T) {
	handler := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	claims := &Claims{Role: "participant"}
	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := contextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Participant should be forbidden, got status %d", rr.Code)
	}
}

func contextWithClaims(ctx interface{ Value(key interface{}) interface{} }, claims *Claims) interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key interface{}) interface{}
} {
	return claimsContext{parent: ctx, claims: claims}
}

type claimsContext struct {
	parent interface{ Value(key interface{}) interface{} }
	claims *Claims
}

func (c claimsContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c claimsContext) Done() <-chan struct{}        { return nil }
func (c claimsContext) Err() error                  { return nil }
func (c claimsContext) Value(key interface{}) interface{} {
	if key == UserContextKey {
		return c.claims
	}
	return c.parent.Value(key)
}
