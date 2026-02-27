package models

import "time"

// LoginRequest represents user login credentials.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest represents user registration data.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned after successful authentication.
type AuthResponse struct {
	User         UserInfo `json:"user"`
	Token        string   `json:"token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresAt    string   `json:"expires_at"`
}

// RefreshRequest represents a token refresh request.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// UserInfo contains public user information.
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
	TeamID   string `json:"team_id,omitempty"`
}

// Event represents a CTF event.
type Event struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	TeamCount       int       `json:"team_count"`
	MaxTeams        int       `json:"max_teams"`
	ChallengesCount int       `json:"challenges_count"`
	VPNEnabled      bool      `json:"vpn_enabled"`
	BrowserAccess   bool      `json:"browser_access"`
}

// EventListResponse is the paginated list of events.
type EventListResponse struct {
	Events     []Event    `json:"events"`
	Pagination Pagination `json:"pagination"`
}

// Pagination contains pagination metadata.
type Pagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// CreateEventRequest contains data for creating a new event.
type CreateEventRequest struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	MaxTeams    int      `json:"max_teams"`
	StartTime   string   `json:"start_time"`
	EndTime     string   `json:"end_time"`
	Frontends   []string `json:"frontends"`
	Exercises   []string `json:"exercises"`
	VPNEnabled  bool     `json:"vpn_enabled"`
	MaxLabHours int      `json:"max_lab_hours"`
}

// Team represents a team in an event.
type Team struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Score           int    `json:"score"`
	Rank            int    `json:"rank"`
	MembersCount    int    `json:"members_count"`
	ChallengesSolve int    `json:"challenges_solved"`
	LastSolveAt     string `json:"last_solve_at,omitempty"`
}

// TeamListResponse is the response for listing teams.
type TeamListResponse struct {
	Teams []Team `json:"teams"`
}

// CreateTeamRequest contains data for creating/joining a team.
type CreateTeamRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// Challenge represents a CTF challenge.
type Challenge struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Category       string   `json:"category"`
	Difficulty     string   `json:"difficulty"`
	Points         int      `json:"points"`
	Description    string   `json:"description"`
	SolvedBy       int      `json:"solved_by"`
	Solved         bool     `json:"solved"`
	Tags           []string `json:"tags"`
	HasInstance    bool     `json:"has_instance"`
	InstanceStatus string   `json:"instance_status,omitempty"`
}

// ChallengeListResponse is the response for listing challenges.
type ChallengeListResponse struct {
	Challenges []Challenge `json:"challenges"`
	Categories []string    `json:"categories"`
}

// FlagSubmitRequest contains data for flag verification.
type FlagSubmitRequest struct {
	EventID     string `json:"event_id"`
	ChallengeID string `json:"challenge_id"`
	Flag        string `json:"flag"`
}

// FlagSubmitResponse is returned after flag verification.
type FlagSubmitResponse struct {
	Correct       bool   `json:"correct"`
	PointsAwarded int    `json:"points_awarded,omitempty"`
	NewTotalScore int    `json:"new_total_score,omitempty"`
	NewRank       int    `json:"new_rank,omitempty"`
	FirstBlood    bool   `json:"first_blood,omitempty"`
	Message       string `json:"message"`
}

// ScoreboardResponse contains the full scoreboard data.
type ScoreboardResponse struct {
	EventID     string      `json:"event_id"`
	LastUpdated string      `json:"last_updated"`
	Teams       []TeamScore `json:"teams"`
	Frozen      bool        `json:"frozen"`
}

// TeamScore represents a team's score entry on the scoreboard.
type TeamScore struct {
	Rank             int    `json:"rank"`
	TeamID           string `json:"team_id"`
	TeamName         string `json:"team_name"`
	Score            int    `json:"score"`
	ChallengesSolved int    `json:"challenges_solved"`
	LastSolveAt      string `json:"last_solve_at,omitempty"`
}

// Lab represents a team's lab environment.
type Lab struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	CreatedAt string     `json:"created_at"`
	ExpiresAt string     `json:"expires_at"`
	Exercises []Exercise `json:"exercises"`
	Frontend  *Frontend  `json:"frontend,omitempty"`
}

// Exercise represents a running exercise instance.
type Exercise struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	IP     string `json:"ip,omitempty"`
}

// Frontend represents a browser-accessible frontend (e.g., Kali VM).
type Frontend struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	RDPURL string `json:"rdp_url,omitempty"`
}

// LabResponse wraps a Lab in an API response.
type LabResponse struct {
	Lab Lab `json:"lab"`
}

// ScoreUpdateMessage is sent via WebSocket for live scoreboard updates.
type ScoreUpdateMessage struct {
	Type string          `json:"type"`
	Data ScoreUpdateData `json:"data"`
}

// ScoreUpdateData contains the details of a score update.
type ScoreUpdateData struct {
	TeamID        string `json:"team_id"`
	TeamName      string `json:"team_name"`
	NewScore      int    `json:"new_score"`
	NewRank       int    `json:"new_rank"`
	ChallengeName string `json:"challenge_name"`
	Points        int    `json:"points"`
	FirstBlood    bool   `json:"first_blood"`
	Timestamp     string `json:"timestamp"`
}

// APIError is the standard error response format.
type APIError struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}
