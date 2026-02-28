package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestRouter() http.Handler {
	return NewRouter(Config{
		SigningKey:         []byte("test-secret-key-for-tests"),
		AllowedOrigins:     []string{"https://cyberorch.com"},
		RateLimitPerMinute: 120,
	})
}

func TestHealthEndpoint(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Health check status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("Health status = %q, want %q", resp["status"], "ok")
	}
}

func TestReadyEndpoint(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Ready check status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestAuthRegisterEndpoint(t *testing.T) {
	router := newTestRouter()

	body := `{"username":"alice","email":"alice@example.com","password":"SecureP@ss123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Register status = %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestAuthLoginEndpoint(t *testing.T) {
	router := newTestRouter()

	body := `{"email":"alice@example.com","password":"SecureP@ss123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Login status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestAuthRegisterWrongMethod(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/auth/register", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET register status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestProtectedEndpointWithoutAuth(t *testing.T) {
	router := newTestRouter()

	// GET /api/v1/events is public (event listing), POST requires auth
	req := httptest.NewRequest("POST", "/api/v1/events", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Unauthenticated events POST status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestCORSHeaders(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://cyberorch.com")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	origin := rr.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://cyberorch.com" {
		t.Errorf("CORS origin = %q, want %q", origin, "https://cyberorch.com")
	}
}

func TestSecurityHeaders(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	}

	for key, want := range headers {
		got := rr.Header().Get(key)
		if got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestFlagVerifyWithoutAuth(t *testing.T) {
	router := newTestRouter()

	body := `{"challenge_id":"chal-1","flag":"FLAG{test}"}`
	req := httptest.NewRequest("POST", "/api/v1/flags/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Unauthenticated flag verify status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
