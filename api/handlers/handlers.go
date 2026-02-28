package handlers

import (
	"net/http"
	"strings"

	"github.com/mrtrkmn/orchi/api/models"
)

// EventHandler handles event-related API endpoints.
type EventHandler struct{}

// NewEventHandler creates a new EventHandler.
func NewEventHandler() *EventHandler {
	return &EventHandler{}
}

// List returns all available events.
//
// GET /api/v1/events
func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	// In production, this would call Daemon gRPC ListEvents
	writeJSON(w, http.StatusOK, models.EventListResponse{
		Events: []models.Event{},
		Pagination: models.Pagination{
			Page:    1,
			PerPage: 20,
			Total:   0,
		},
	})
}

// Get returns a specific event by ID.
//
// GET /api/v1/events/{eventId}
func (h *EventHandler) Get(w http.ResponseWriter, r *http.Request) {
	// eventID would be extracted from URL path
	writeJSON(w, http.StatusOK, models.Event{})
}

// GetBySlug returns an event by its URL slug (subdomain name).
//
// GET /api/v1/events/by-slug/{slug}
//
// The slug corresponds to the subdomain part of <slug>.cyberorch.com.
// Used by the frontend to resolve event context from the current hostname.
func (h *EventHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	// Extract slug from URL path
	path := r.URL.Path
	prefix := "/api/v1/events/by-slug/"
	if !strings.HasPrefix(path, prefix) || len(path) <= len(prefix) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Event slug is required")
		return
	}
	slug := path[len(prefix):]

	// In production, this would look up the event by slug via gRPC
	// For now, return a placeholder event matching the slug
	writeJSON(w, http.StatusOK, models.Event{
		Name:   slug,
		Type:   "ctf",
		Status: "running",
	})
}

// Create creates a new event (admin/organizer only).
//
// POST /api/v1/events
func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateEventRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Event name is required")
		return
	}

	// In production, call Daemon gRPC CreateEvent
	writeJSON(w, http.StatusCreated, models.Event{
		Name:   req.Name,
		Type:   req.Type,
		Status: "created",
	})
}

// Delete stops and archives an event (admin only).
//
// DELETE /api/v1/events/{eventId}
func (h *EventHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// In production, call Daemon gRPC StopEvent
	w.WriteHeader(http.StatusNoContent)
}

// ChallengeHandler handles challenge-related API endpoints.
type ChallengeHandler struct{}

// NewChallengeHandler creates a new ChallengeHandler.
func NewChallengeHandler() *ChallengeHandler {
	return &ChallengeHandler{}
}

// List returns challenges for an event.
//
// GET /api/v1/events/{eventId}/challenges
func (h *ChallengeHandler) List(w http.ResponseWriter, r *http.Request) {
	// In production, call Daemon gRPC ListExercises
	writeJSON(w, http.StatusOK, models.ChallengeListResponse{
		Challenges: []models.Challenge{},
		Categories: []string{},
	})
}

// Get returns a specific challenge.
//
// GET /api/v1/events/{eventId}/challenges/{challengeId}
func (h *ChallengeHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.Challenge{})
}

// VerifyFlag checks a submitted flag.
//
// POST /api/v1/flags/verify
func (h *ChallengeHandler) VerifyFlag(w http.ResponseWriter, r *http.Request) {
	var req models.FlagSubmitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Flag == "" || req.ChallengeID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Challenge ID and flag are required")
		return
	}

	if len(req.Flag) > 256 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Flag must be at most 256 characters")
		return
	}

	// In production, call Daemon gRPC SolveChallenge
	writeJSON(w, http.StatusOK, models.FlagSubmitResponse{
		Correct: false,
		Message: "Incorrect flag. Try again.",
	})
}

// TeamHandler handles team-related API endpoints.
type TeamHandler struct{}

// NewTeamHandler creates a new TeamHandler.
func NewTeamHandler() *TeamHandler {
	return &TeamHandler{}
}

// List returns teams for an event.
//
// GET /api/v1/events/{eventId}/teams
func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	// In production, call Daemon gRPC ListEventTeams
	writeJSON(w, http.StatusOK, models.TeamListResponse{
		Teams: []models.Team{},
	})
}

// Create creates or joins a team.
//
// POST /api/v1/events/{eventId}/teams
func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}

	if req.Name == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Team name and password are required")
		return
	}

	// In production, call Daemon gRPC to create team + assign lab
	writeJSON(w, http.StatusCreated, models.Team{
		Name: req.Name,
	})
}

// Get returns team details.
//
// GET /api/v1/teams/{teamId}
func (h *TeamHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.Team{})
}

// ScoreboardHandler handles scoreboard-related API endpoints.
type ScoreboardHandler struct{}

// NewScoreboardHandler creates a new ScoreboardHandler.
func NewScoreboardHandler() *ScoreboardHandler {
	return &ScoreboardHandler{}
}

// Get returns the current scoreboard for an event.
//
// GET /api/v1/events/{eventId}/scoreboard
func (h *ScoreboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	// In production, query Store for team scores, sort by rank
	writeJSON(w, http.StatusOK, models.ScoreboardResponse{
		Teams:  []models.TeamScore{},
		Frozen: false,
	})
}

// LabHandler handles lab-related API endpoints.
type LabHandler struct{}

// NewLabHandler creates a new LabHandler.
func NewLabHandler() *LabHandler {
	return &LabHandler{}
}

// Get returns the lab status for a team.
//
// GET /api/v1/teams/{teamId}/lab
func (h *LabHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, models.LabResponse{
		Lab: models.Lab{
			Status:    "not_created",
			Exercises: []models.Exercise{},
		},
	})
}

// Reset resets the lab environment.
//
// POST /api/v1/teams/{teamId}/lab/reset
func (h *LabHandler) Reset(w http.ResponseWriter, r *http.Request) {
	// In production, call Daemon gRPC RestartTeamLab
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Lab reset initiated",
	})
}

// ResetExercise resets a specific exercise in the lab.
//
// POST /api/v1/teams/{teamId}/lab/exercises/{exerciseId}/reset
func (h *LabHandler) ResetExercise(w http.ResponseWriter, r *http.Request) {
	// In production, call Daemon gRPC ResetExercise
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Exercise reset initiated",
	})
}

// DownloadVPNConfig returns the WireGuard VPN configuration file.
//
// GET /api/v1/teams/{teamId}/vpn/config
func (h *LabHandler) DownloadVPNConfig(w http.ResponseWriter, r *http.Request) {
	// In production, generate WireGuard config from VPN service
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=orchi-vpn.conf")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# WireGuard VPN Configuration\n# Generated by Orchi API\n"))
}
