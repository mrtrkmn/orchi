# Orchi Platform — Architecture Design

## Overview

Orchi is a Kubernetes-native Capture The Flag (CTF) competition platform, redesigned
with a fully decoupled architecture where the **Web Frontend** and **Backend** operate
as independent, separately deployable systems communicating exclusively through
versioned REST APIs and WebSocket connections.

---

## High-Level Architecture

```
                    ┌──────────────────────────────────────────────────────┐
                    │                    INTERNET                         │
                    └─────────────────────┬────────────────────────────────┘
                                          │
                                          ▼
                    ┌──────────────────────────────────────────────────────┐
                    │              Kubernetes Ingress (Traefik)            │
                    │         TLS termination, rate limiting, CORS        │
                    └──────┬──────────────────────────────────┬───────────┘
                           │                                  │
              ┌────────────▼──────────┐         ┌─────────────▼───────────┐
              │   frontend.cyberorch.com   │         │    api.cyberorch.com         │
              │   (Static SPA)        │         │    (REST API Gateway)   │
              └───────────────────────┘         └─────────────┬───────────┘
                                                              │
                           ┌──────────────────────────────────┤
                           │              │                   │
              ┌────────────▼──┐  ┌────────▼───────┐  ┌───────▼──────────┐
              │  Auth Service │  │  Core Service   │  │  Lab Service     │
              │  (JWT/OIDC)   │  │  (Events,Teams, │  │  (Orchestration) │
              │               │  │   Challenges,   │  │                  │
              │               │  │   Scores)       │  │                  │
              └───────┬───────┘  └────────┬────────┘  └───────┬──────────┘
                      │                   │                   │
              ┌───────▼───────────────────▼───────────────────▼──────────┐
              │                     Store (gRPC :5454)                   │
              │                   StatefulSet + PVC                      │
              └──────────────────────────────────────────────────────────┘
                           │                            │
              ┌────────────▼──────────┐    ┌────────────▼──────────┐
              │  WireGuard VPN        │    │  VNC Proxy (noVNC)    │
              │  (Per-team tunnels)   │    │  (Browser desktops)   │
              └───────────────────────┘    └───────────────────────┘
```

---

## Component Responsibilities

### Frontend (Static SPA)
- **Technology**: React 18 + TypeScript + Vite
- **Deployment**: Static container served via Nginx, behind CDN
- **Responsibilities**:
  - All UI rendering (zero server-side rendering in backend)
  - JWT token management (storage, refresh, logout)
  - WebSocket client for live scoreboard updates
  - Role-based UI rendering (participant vs admin)
  - Responsive dark-mode cybersecurity aesthetic
- **No direct access** to databases, gRPC services, or internal APIs

### API Gateway
- **Technology**: Go (net/http + chi router)
- **Deployment**: Kubernetes Deployment with HPA
- **Responsibilities**:
  - Versioned REST API (`/api/v1/...`)
  - JWT authentication and authorization middleware
  - CORS policy enforcement
  - Rate limiting per client/IP
  - Request validation and sanitization
  - WebSocket upgrade for `/api/v1/ws/scoreboard`
  - Translates REST ↔ gRPC calls to backend services

### Auth Service
- **Responsibilities**:
  - User registration and login
  - JWT token issuance and refresh
  - Password hashing (bcrypt)
  - Role management (admin, organizer, participant)
  - Session revocation

### Core Service
- **Responsibilities**:
  - Event CRUD and lifecycle management
  - Team creation and management
  - Challenge catalog and flag verification
  - Score calculation and leaderboard
  - Profile management

### Lab Service
- **Responsibilities**:
  - Lab environment provisioning via Kubernetes CRDs
  - Exercise container lifecycle (start, stop, reset)
  - VPN configuration generation (WireGuard)
  - VNC session management (VNC Proxy, replacing Guacamole — see [guacamole-replacement.md](guacamole-replacement.md))
  - Resource quota enforcement per team

### Store
- **Technology**: gRPC service on port 5454
- **Deployment**: StatefulSet with persistent volume
- **Responsibilities**:
  - All data persistence
  - Event, team, user, challenge data storage
  - Time-series data for scoring

---

## Technology Stack

| Layer | Technology | Justification |
|-------|-----------|---------------|
| Frontend | React 18 + TypeScript + Vite | Modern, fast builds, strong ecosystem |
| UI Components | Tailwind CSS + shadcn/ui | Utility-first, dark mode native |
| State Management | Zustand | Lightweight, TypeScript-native |
| API Client | Axios + React Query | Caching, retry, optimistic updates |
| Real-time | Native WebSocket | Live scoreboard, challenge updates |
| API Gateway | Go + chi router | Performance, matches existing codebase |
| Authentication | JWT (RS256) | Stateless, scalable |
| API Spec | OpenAPI 3.1 | Industry standard, code generation |
| Container Runtime | Docker | Standard |
| Orchestration | Kubernetes | Cloud-native, operator pattern |
| Ingress | Traefik | Native K8s integration |
| Observability | Prometheus + Grafana + Loki | Full-stack monitoring |
| CI/CD | GitHub Actions | Existing pipeline |

---

## Separation of Concerns

```
┌─────────────────────────────────────────────────────────────────┐
│ FRONTEND (React SPA)                                            │
│                                                                 │
│  ┌──────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────────┐  │
│  │  Pages   │ │Components│ │   Hooks   │ │  API Client      │  │
│  │          │ │          │ │           │ │  (axios/react-   │  │
│  │ Login    │ │ ScoreBar │ │ useAuth   │ │   query)         │  │
│  │ Dashboard│ │ ChalCard │ │ useTeam   │ │                  │  │
│  │ Chals    │ │ FlagForm │ │ useScore  │ │ GET /api/v1/...  │  │
│  │ Score    │ │ LabPanel │ │ useLab    │ │ POST /api/v1/... │  │
│  │ Admin    │ │ TeamList │ │ useWS     │ │ WS /api/v1/ws/.. │  │
│  └──────────┘ └──────────┘ └───────────┘ └──────────────────┘  │
└────────────────────────────────┬────────────────────────────────┘
                                 │ HTTPS / WSS only
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ API GATEWAY (Go)                                                │
│                                                                 │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────┐  │
│  │  Routing   │ │   Auth     │ │   CORS     │ │  Rate       │  │
│  │  /api/v1/* │ │  Middleware│ │  Middleware │ │  Limiter    │  │
│  └────────────┘ └────────────┘ └────────────┘ └─────────────┘  │
│                                                                 │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────┐  │
│  │  Auth      │ │  Event     │ │  Challenge  │ │  Lab        │  │
│  │  Handlers  │ │  Handlers  │ │  Handlers   │ │  Handlers   │  │
│  └────────────┘ └────────────┘ └────────────┘ └─────────────┘  │
└────────────────────────────────┬────────────────────────────────┘
                                 │ gRPC (internal)
                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│ BACKEND SERVICES (Go)                                           │
│                                                                 │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌─────────────┐  │
│  │  Daemon    │ │  Store     │ │  Exercise   │ │  Lab        │  │
│  │  (Operator)│ │  (gRPC)    │ │  (gRPC)     │ │  (K8s CRDs) │  │
│  └────────────┘ └────────────┘ └────────────┘ └─────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Data Flow Examples

### User Login
```
Frontend                    API Gateway              Auth Service          Store
   │                            │                         │                  │
   │ POST /api/v1/auth/login    │                         │                  │
   │ {email, password}          │                         │                  │
   │ ──────────────────────────>│                         │                  │
   │                            │ Validate credentials    │                  │
   │                            │ ───────────────────────>│                  │
   │                            │                         │ GetUser(email)   │
   │                            │                         │ ────────────────>│
   │                            │                         │ <────────────────│
   │                            │                         │ bcrypt.Compare   │
   │                            │ <───────────────────────│                  │
   │                            │ Generate JWT            │                  │
   │ 200 {token, refresh, user} │                         │                  │
   │ <──────────────────────────│                         │                  │
```

### Flag Submission
```
Frontend                    API Gateway              Core Service          Store
   │                            │                         │                  │
   │ POST /api/v1/flags/verify  │                         │                  │
   │ {challenge_id, flag}       │                         │                  │
   │ Auth: Bearer <JWT>         │                         │                  │
   │ ──────────────────────────>│                         │                  │
   │                            │ Verify JWT              │                  │
   │                            │ Extract team_id         │                  │
   │                            │ SolveChallenge(req)     │                  │
   │                            │ ───────────────────────>│                  │
   │                            │                         │ UpdateSolved     │
   │                            │                         │ ────────────────>│
   │                            │                         │ <────────────────│
   │                            │ <───────────────────────│                  │
   │ 200 {correct: true,        │                         │                  │
   │      points: 100,          │                         │                  │
   │      new_score: 450}       │                         │                  │
   │ <──────────────────────────│                         │                  │
   │                            │                         │                  │
   │                            │ Broadcast via WebSocket │                  │
   │                            │ ───────────> all clients│                  │
```

### Live Scoreboard (WebSocket)
```
Frontend                    API Gateway
   │                            │
   │ GET /api/v1/ws/scoreboard  │
   │ Upgrade: websocket         │
   │ Auth: Bearer <JWT>         │
   │ ──────────────────────────>│
   │                            │ Verify JWT
   │ <─── 101 Switching ────────│
   │                            │
   │ <─── {type: "score_update",│
   │       team: "Alpha",       │
   │       score: 450,          │
   │       rank: 3}             │
   │                            │
   │ <─── {type: "score_update",│  (on each flag solve)
   │       ...}                 │
```

---

## Scaling Strategy

| Component | Scaling | Min Replicas | Max Replicas |
|-----------|---------|-------------|-------------|
| Frontend | HPA (CPU) | 2 | 10 |
| API Gateway | HPA (CPU/RPS) | 3 | 20 |
| Auth Service | HPA (CPU) | 2 | 10 |
| Core Service | HPA (CPU) | 2 | 10 |
| Lab Service | HPA (CPU) | 2 | 5 |
| Store | StatefulSet | 1 | 3 (with replication) |
| WireGuard | DaemonSet | 1 per node | 1 per node |
| VNC Proxy | HPA (CPU/connections) | 2 | 20 |

For a 500+ concurrent team competition:
- API Gateway: 5-10 replicas
- Core Service: 3-5 replicas
- WebSocket connections: Sticky sessions via Ingress annotation
- Store: Read replicas for scoreboard queries

---

## Namespace Strategy

```
orchi-system/          # Operator, API Gateway, Core Services
orchi-frontend/        # Frontend deployment, CDN origin
orchi-store/           # Store StatefulSet, backups
orchi-monitoring/      # Prometheus, Grafana, Loki
orchi-lab-{event-id}/  # Per-event lab namespaces (auto-created)
```

---

## Next Steps

- [Part 2: API Specification](./api-specification.md)
- [Part 3: Frontend Design](./frontend-design.md)
- [Part 4: Security Model](./security-model.md)
- [Part 5: Deployment Guide](./deployment-guide.md)
- [Part 6: Migration Strategy](./migration-strategy.md)
