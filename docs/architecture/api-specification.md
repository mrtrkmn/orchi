# API Specification

## Overview

The Orchi API is a versioned REST API that serves as the sole communication
interface between the frontend and backend services. All endpoints are prefixed
with `/api/v1/` and require JWT authentication unless explicitly marked as public.

---

## Base URL

```
Production:  https://api.orchi.io/api/v1
Staging:     https://api-staging.orchi.io/api/v1
Development: http://localhost:8080/api/v1
```

---

## Authentication

All authenticated endpoints require the `Authorization` header:
```
Authorization: Bearer <JWT_TOKEN>
```

### Token Structure (JWT Claims)
```json
{
  "sub": "user-uuid",
  "email": "user@example.com",
  "role": "participant",
  "team_id": "team-uuid",
  "event_id": "event-uuid",
  "iat": 1700000000,
  "exp": 1700003600
}
```

### Roles
- `admin` — Full platform access
- `organizer` — Event management access
- `participant` — Team member access

---

## API Versioning Strategy

- URL path versioning: `/api/v1/`, `/api/v2/`
- Breaking changes increment major version
- Non-breaking additions within same version
- Minimum 6-month deprecation window for old versions
- `Sunset` header on deprecated endpoints

---

## Endpoints

### Auth Service

#### POST /api/v1/auth/register (Public)
Register a new user account.

**Request:**
```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "SecureP@ss123"
}
```

**Response (201):**
```json
{
  "user": {
    "id": "uuid",
    "username": "alice",
    "email": "alice@example.com",
    "role": "participant",
    "created_at": "2026-01-15T10:00:00Z"
  },
  "token": "eyJhbG...",
  "refresh_token": "eyJhbG..."
}
```

#### POST /api/v1/auth/login (Public)
Authenticate and receive tokens.

**Request:**
```json
{
  "email": "alice@example.com",
  "password": "SecureP@ss123"
}
```

**Response (200):**
```json
{
  "user": {
    "id": "uuid",
    "username": "alice",
    "role": "participant",
    "team_id": "team-uuid"
  },
  "token": "eyJhbG...",
  "refresh_token": "eyJhbG...",
  "expires_at": "2026-01-15T11:00:00Z"
}
```

#### POST /api/v1/auth/refresh (Public)
Refresh an expired access token.

**Request:**
```json
{
  "refresh_token": "eyJhbG..."
}
```

**Response (200):**
```json
{
  "token": "eyJhbG...",
  "refresh_token": "eyJhbG...",
  "expires_at": "2026-01-15T12:00:00Z"
}
```

#### POST /api/v1/auth/logout
Revoke the current session.

**Response (204):** No content.

---

### Events

#### GET /api/v1/events
List available events.

**Query Parameters:**
- `status` — Filter by status: `running`, `upcoming`, `closed`
- `page` — Page number (default: 1)
- `per_page` — Items per page (default: 20, max: 100)

**Response (200):**
```json
{
  "events": [
    {
      "id": "evt-uuid",
      "name": "CTF Spring 2026",
      "type": "jeopardy",
      "status": "running",
      "start_time": "2026-03-01T09:00:00Z",
      "end_time": "2026-03-01T21:00:00Z",
      "team_count": 42,
      "max_teams": 100,
      "challenges_count": 25,
      "vpn_enabled": true,
      "browser_access": true
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 1
  }
}
```

#### GET /api/v1/events/{event_id}
Get event details.

#### POST /api/v1/events (Admin/Organizer)
Create a new event.

**Request:**
```json
{
  "name": "CTF Spring 2026",
  "type": "jeopardy",
  "max_teams": 100,
  "start_time": "2026-03-01T09:00:00Z",
  "end_time": "2026-03-01T21:00:00Z",
  "frontends": ["kali"],
  "exercises": ["sql-injection", "xss-basic"],
  "vpn_enabled": true,
  "max_lab_hours": 8
}
```

#### DELETE /api/v1/events/{event_id} (Admin)
Stop and archive an event.

---

### Teams

#### GET /api/v1/events/{event_id}/teams
List teams in an event.

**Response (200):**
```json
{
  "teams": [
    {
      "id": "team-uuid",
      "name": "CyberWolves",
      "score": 450,
      "rank": 3,
      "members_count": 4,
      "challenges_solved": 8,
      "last_solve_at": "2026-03-01T14:30:00Z"
    }
  ]
}
```

#### POST /api/v1/events/{event_id}/teams
Create/join a team.

**Request:**
```json
{
  "name": "CyberWolves",
  "password": "team-secret"
}
```

#### GET /api/v1/teams/{team_id}
Get team details (own team).

---

### Challenges

#### GET /api/v1/events/{event_id}/challenges
List challenges for the event.

**Response (200):**
```json
{
  "challenges": [
    {
      "id": "chal-uuid",
      "name": "SQL Injection 101",
      "category": "web",
      "difficulty": "easy",
      "points": 100,
      "description": "Find the flag in the vulnerable login form.",
      "solved_by": 12,
      "solved": false,
      "tags": ["sql", "web", "beginner"],
      "has_instance": true,
      "instance_status": "running"
    }
  ],
  "categories": ["web", "crypto", "forensics", "pwn", "misc"]
}
```

#### GET /api/v1/events/{event_id}/challenges/{challenge_id}
Get challenge details.

#### POST /api/v1/flags/verify
Submit a flag for verification.

**Request:**
```json
{
  "event_id": "evt-uuid",
  "challenge_id": "chal-uuid",
  "flag": "FLAG{sql_1nj3ct10n_m4st3r}"
}
```

**Response (200):**
```json
{
  "correct": true,
  "points_awarded": 100,
  "new_total_score": 450,
  "new_rank": 3,
  "first_blood": false,
  "message": "Correct! +100 points"
}
```

**Response (200, incorrect):**
```json
{
  "correct": false,
  "message": "Incorrect flag. Try again.",
  "attempts_remaining": null
}
```

---

### Scoreboard

#### GET /api/v1/events/{event_id}/scoreboard
Get current scoreboard.

**Response (200):**
```json
{
  "event_id": "evt-uuid",
  "last_updated": "2026-03-01T14:30:00Z",
  "teams": [
    {
      "rank": 1,
      "team_id": "team-uuid",
      "team_name": "HackTheBox",
      "score": 1250,
      "challenges_solved": 15,
      "last_solve_at": "2026-03-01T14:25:00Z"
    }
  ],
  "frozen": false
}
```

#### WebSocket: /api/v1/ws/scoreboard?event_id={event_id}
Live scoreboard updates via WebSocket.

**Server Messages:**
```json
{
  "type": "score_update",
  "data": {
    "team_id": "team-uuid",
    "team_name": "HackTheBox",
    "new_score": 1350,
    "new_rank": 1,
    "challenge_name": "Buffer Overflow",
    "points": 100,
    "first_blood": true,
    "timestamp": "2026-03-01T14:35:00Z"
  }
}
```

---

### Labs

#### GET /api/v1/teams/{team_id}/lab
Get lab status for the team.

**Response (200):**
```json
{
  "lab": {
    "id": "lab-uuid",
    "status": "running",
    "created_at": "2026-03-01T09:15:00Z",
    "expires_at": "2026-03-01T17:15:00Z",
    "exercises": [
      {
        "id": "ex-uuid",
        "name": "sql-injection",
        "status": "running",
        "ip": "10.0.1.5"
      }
    ],
    "frontend": {
      "type": "kali",
      "status": "running",
      "rdp_url": "https://guac.orchi.io/session/abc123"
    }
  }
}
```

#### POST /api/v1/teams/{team_id}/lab/reset
Reset the lab environment.

#### POST /api/v1/teams/{team_id}/lab/exercises/{exercise_id}/reset
Reset a specific exercise.

#### GET /api/v1/teams/{team_id}/vpn/config
Download WireGuard VPN configuration.

**Response (200):** `application/octet-stream` with `.conf` file.

---

### Admin

#### GET /api/v1/admin/events (Admin/Organizer)
List all events with admin details.

#### GET /api/v1/admin/users (Admin)
List all users.

#### PUT /api/v1/admin/users/{user_id}/role (Admin)
Update user role.

#### POST /api/v1/admin/events/{event_id}/freeze-scoreboard (Organizer)
Freeze the scoreboard.

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": {
    "code": "INVALID_FLAG",
    "message": "The submitted flag is incorrect.",
    "details": {}
  }
}
```

### Error Codes
| HTTP | Code | Description |
|------|------|-------------|
| 400 | `VALIDATION_ERROR` | Request validation failed |
| 401 | `UNAUTHORIZED` | Missing or invalid token |
| 403 | `FORBIDDEN` | Insufficient permissions |
| 404 | `NOT_FOUND` | Resource not found |
| 409 | `CONFLICT` | Resource already exists |
| 429 | `RATE_LIMITED` | Too many requests |
| 500 | `INTERNAL_ERROR` | Server error |

---

## Rate Limiting

| Endpoint Pattern | Rate Limit |
|-----------------|------------|
| `POST /auth/login` | 10/min per IP |
| `POST /flags/verify` | 30/min per team |
| `GET /scoreboard` | 60/min per user |
| `GET /*` | 120/min per user |
| `POST /*` | 60/min per user |
| WebSocket | 1 connection per user |

Rate limit headers:
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1700003600
```

---

## CORS Policy

```
Access-Control-Allow-Origin: https://orchi.io, https://staging.orchi.io
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, X-Request-ID
Access-Control-Max-Age: 86400
Access-Control-Allow-Credentials: false
```
