package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthRegisterSuccess(t *testing.T) {
	h := NewAuthHandler([]byte("test-secret"))

	body := `{"username":"alice","email":"alice@example.com","password":"SecureP@ss123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	user := resp["user"].(map[string]interface{})
	if user["username"] != "alice" {
		t.Errorf("Username = %v, want alice", user["username"])
	}
}

func TestAuthRegisterMissingFields(t *testing.T) {
	h := NewAuthHandler([]byte("test-secret"))

	body := `{"username":"alice"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAuthRegisterShortPassword(t *testing.T) {
	h := NewAuthHandler([]byte("test-secret"))

	body := `{"username":"alice","email":"alice@example.com","password":"short"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAuthLoginSuccess(t *testing.T) {
	h := NewAuthHandler([]byte("test-secret"))

	body := `{"email":"alice@example.com","password":"SecureP@ss123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["token"] == nil {
		t.Error("Response should contain token")
	}
}

func TestAuthLoginMissingFields(t *testing.T) {
	h := NewAuthHandler([]byte("test-secret"))

	body := `{"email":"alice@example.com"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFlagVerifyMissingFields(t *testing.T) {
	h := NewChallengeHandler()

	body := `{"flag":""}`
	req := httptest.NewRequest("POST", "/api/v1/flags/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.VerifyFlag(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFlagVerifyTooLong(t *testing.T) {
	h := NewChallengeHandler()

	longFlag := strings.Repeat("A", 300)
	body := `{"challenge_id":"abc","flag":"` + longFlag + `"}`
	req := httptest.NewRequest("POST", "/api/v1/flags/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.VerifyFlag(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestFlagVerifyValid(t *testing.T) {
	h := NewChallengeHandler()

	body := `{"event_id":"evt-1","challenge_id":"chal-1","flag":"FLAG{test}"}`
	req := httptest.NewRequest("POST", "/api/v1/flags/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.VerifyFlag(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestTeamCreateMissingFields(t *testing.T) {
	h := NewTeamHandler()

	body := `{"name":"team1"}`
	req := httptest.NewRequest("POST", "/api/v1/events/evt-1/teams", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestTeamCreateSuccess(t *testing.T) {
	h := NewTeamHandler()

	body := `{"name":"CyberWolves","password":"team-secret"}`
	req := httptest.NewRequest("POST", "/api/v1/events/evt-1/teams", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestEventCreateSuccess(t *testing.T) {
	h := NewEventHandler()

	body := `{"name":"CTF Spring 2026","type":"jeopardy","max_teams":100}`
	req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestEventCreateMissingName(t *testing.T) {
	h := NewEventHandler()

	body := `{"type":"jeopardy"}`
	req := httptest.NewRequest("POST", "/api/v1/events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestScoreboardGet(t *testing.T) {
	h := NewScoreboardHandler()

	req := httptest.NewRequest("GET", "/api/v1/events/evt-1/scoreboard", nil)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestLabGet(t *testing.T) {
	h := NewLabHandler()

	req := httptest.NewRequest("GET", "/api/v1/teams/team-1/lab", nil)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestVPNConfigDownload(t *testing.T) {
	h := NewLabHandler()

	req := httptest.NewRequest("GET", "/api/v1/teams/team-1/vpn/config", nil)
	rr := httptest.NewRecorder()

	h.DownloadVPNConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusOK)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
}
