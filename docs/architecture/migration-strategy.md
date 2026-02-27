# Migration Strategy

## Overview

This document outlines the migration path from the current monolithic
Amigo-based architecture (server-side rendered templates) to the fully
decoupled frontend + API architecture.

---

## Current State

```
┌──────────────────────────────────────────────────┐
│ Current Architecture                              │
│                                                  │
│  ┌──────────────────────────────────────────┐    │
│  │ Amigo Service (svcs/amigo/)              │    │
│  │                                          │    │
│  │  ┌─────────────┐  ┌──────────────────┐   │    │
│  │  │ Go Templates│  │ HTTP Handlers    │   │    │
│  │  │ (11 .tmpl   │  │ (login, signup,  │   │    │
│  │  │  files)     │  │  flags, teams,   │   │    │
│  │  └─────────────┘  │  scoreboard)     │   │    │
│  │                    └──────────────────┘   │    │
│  │  ┌─────────────┐  ┌──────────────────┐   │    │
│  │  │ Vue.js SPA  │  │ WebSocket Server │   │    │
│  │  │ (embedded)  │  │ (scores, chals)  │   │    │
│  │  └─────────────┘  └──────────────────┘   │    │
│  │  ┌─────────────┐                         │    │
│  │  │ Static Files│ (CSS, JS, images)       │    │
│  │  └─────────────┘                         │    │
│  └──────────────────────────────────────────┘    │
│                         │                        │
│                    gRPC │                        │
│                         ▼                        │
│  ┌──────────────────────────────────────────┐    │
│  │ Daemon (gRPC server :5454)               │    │
│  │ Event management, Team management,       │    │
│  │ Lab orchestration, Auth                  │    │
│  └──────────────────────────────────────────┘    │
└──────────────────────────────────────────────────┘
```

### Problems with Current Architecture
1. **Tight coupling**: Frontend templates are embedded in Go service
2. **Shared runtime**: Template rendering + API logic in same process
3. **No API contract**: Frontend talks to Go handlers directly
4. **No independent deployment**: UI changes require backend redeploy
5. **Limited scalability**: Cannot scale frontend independently
6. **Mixed concerns**: Auth, routing, rendering all in `amigo.go`

---

## Target State

```
┌──────────────────────────────────────────────────┐
│ Target Architecture                               │
│                                                  │
│  ┌──────────────────┐  ┌──────────────────────┐  │
│  │ Frontend (React) │  │ API Gateway (Go)     │  │
│  │ Static SPA       │  │                      │  │
│  │ Dark CTF theme   │  │ /api/v1/* endpoints  │  │
│  │ JWT auth         │──│ JWT middleware       │  │
│  │ WebSocket client │  │ CORS, Rate limiting  │  │
│  │                  │  │ WebSocket server     │  │
│  │ orchi-frontend/  │  │ orchi-system/        │  │
│  └──────────────────┘  └──────────┬───────────┘  │
│                                   │ gRPC         │
│                                   ▼              │
│  ┌──────────────────────────────────────────┐    │
│  │ Backend Services (existing)              │    │
│  │ Daemon, Store, Exercise                  │    │
│  └──────────────────────────────────────────┘    │
└──────────────────────────────────────────────────┘
```

---

## Migration Phases

### Phase 1: API Layer (Weeks 1-3)

**Goal**: Create the REST API Gateway alongside existing Amigo service.

1. Create `api/` directory with Go REST handlers
2. Implement JWT authentication middleware
3. Map each Amigo HTTP handler to a REST endpoint:

   | Amigo Handler | New API Endpoint |
   |--------------|------------------|
   | `POST /signup` | `POST /api/v1/auth/register` |
   | `POST /login` | `POST /api/v1/auth/login` |
   | `POST /logout` | `POST /api/v1/auth/logout` |
   | `POST /flags/verify` | `POST /api/v1/flags/verify` |
   | `GET /scores` (WS) | `GET /api/v1/ws/scoreboard` |
   | `GET /scoreboard` | `GET /api/v1/events/{id}/scoreboard` |
   | `GET /challenges` | `GET /api/v1/events/{id}/challenges` |
   | `GET /teams` | `GET /api/v1/events/{id}/teams` |
   | `POST /vpn/download` | `GET /api/v1/teams/{id}/vpn/config` |
   | `POST /reset/challenge` | `POST /api/v1/teams/{id}/lab/exercises/{id}/reset` |

4. Both Amigo and API Gateway run in parallel
5. API Gateway connects to same gRPC backends

**Verification**: All API endpoints return correct JSON responses.

### Phase 2: Frontend Development (Weeks 2-5)

**Goal**: Build the React SPA that replaces Amigo templates.

1. Create `frontend/` directory with React + Vite + TypeScript
2. Implement pages corresponding to each template:

   | Template File | React Page |
   |--------------|------------|
   | `login.tmpl.html` | `LoginPage.tsx` |
   | `signup.tmpl.html` | `RegisterPage.tsx` |
   | `challenges.tmpl.html` | `ChallengesPage.tsx` |
   | `scoreboard.tmpl.html` | `ScoreboardPage.tsx` |
   | `teams.tmpl.html` | `TeamsPage.tsx` |
   | `hosts.tmpl.html` | `LabPage.tsx` |
   | `info.tmpl.html` | `DashboardPage.tsx` |
   | `index.tmpl.html` | `LandingPage.tsx` |

3. Implement API client connecting to new REST API
4. Implement WebSocket client for live scoreboard
5. Implement JWT auth flow

**Verification**: Frontend works with API Gateway, no Amigo dependency.

### Phase 3: Kubernetes Manifests (Weeks 4-6)

**Goal**: Deploy both old and new systems in parallel.

1. Create frontend Kubernetes Deployment + Service
2. Create API Gateway Kubernetes Deployment + Service
3. Update Ingress to route:
   - `cyberorch.com` → Frontend (new)
   - `api.cyberorch.com` → API Gateway (new)
   - `legacy.cyberorch.com` → Amigo (old, for rollback)
4. Create NetworkPolicies for new components
5. Add HPA for new components

### Phase 4: Parallel Operation (Weeks 5-7)

**Goal**: Run old and new systems simultaneously.

```
                    Ingress
                      │
           ┌──────────┼──────────┐
           │          │          │
      cyberorch.com   api.cyberorch.com  legacy.cyberorch.com
      (React)    (API GW)      (Amigo)
           │          │          │
           │          ▼          │
           │    ┌──────────┐    │
           └───>│ Backends │<───┘
                └──────────┘
```

- Feature flags to gradually shift traffic
- Monitor error rates on new system
- A/B testing with percentage-based routing
- Shared backend ensures data consistency

### Phase 5: Cutover (Week 8)

**Goal**: Complete migration to new architecture.

1. Redirect all `legacy.cyberorch.com` traffic to `cyberorch.com`
2. Remove Amigo deployment from production
3. Keep Amigo code in repository for reference (deprecated)
4. Update documentation
5. Remove legacy Ingress rules

---

## Rollback Plan

At each phase, rollback is straightforward:

### Phase 1 Rollback
- Remove API Gateway deployment
- No impact on existing Amigo service

### Phase 2 Rollback
- Remove frontend deployment
- Amigo continues serving as before

### Phase 3-4 Rollback
- Update Ingress to route all traffic to Amigo
- Remove frontend and API Gateway deployments
- 5-minute recovery time

### Phase 5 Rollback (Post-Cutover)
- Re-deploy Amigo from last known-good image
- Update Ingress to route to Amigo
- API Gateway can remain (serves both old and new frontend)

---

## Data Migration

**No data migration required.** Both old and new architectures use the same
backend Store service. The API Gateway translates REST calls to gRPC calls
to the existing Store service. No schema changes needed.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| API contract mismatch | Medium | High | OpenAPI spec validation, integration tests |
| WebSocket compatibility | Low | Medium | Fallback to polling |
| JWT token issues | Low | High | Extensive auth testing, token refresh logic |
| Performance regression | Low | Medium | Load testing before cutover |
| Browser compatibility | Low | Low | Polyfills, testing matrix |

---

## Success Criteria

1. All existing functionality works in new frontend
2. API response times < 100ms p95
3. WebSocket reconnection works reliably
4. Zero data loss during migration
5. Frontend loads in < 2 seconds
6. All security tests pass
7. Can scale frontend independently from backend
