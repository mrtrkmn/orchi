# Security Model

## Overview

The Orchi platform implements defense-in-depth security with zero-trust
networking, token-based authentication, encrypted communications, and
comprehensive audit logging.

---

## Authentication Model

### JWT Token Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Token Lifecycle                             │
│                                                              │
│  Login Request                                               │
│       │                                                      │
│       ▼                                                      │
│  ┌─────────────────┐                                         │
│  │  Access Token    │ ← RS256 signed                         │
│  │  TTL: 1 hour     │                                        │
│  │  Claims: user_id,│                                        │
│  │  role, team_id,  │                                        │
│  │  event_id        │                                        │
│  └─────────────────┘                                         │
│       +                                                      │
│  ┌─────────────────┐                                         │
│  │  Refresh Token   │ ← Opaque, stored server-side           │
│  │  TTL: 7 days     │                                        │
│  │  One-time use    │ ← Rotation on each refresh             │
│  │  Revocable       │                                        │
│  └─────────────────┘                                         │
│                                                              │
│  Refresh Flow:                                               │
│  1. Access token expires (401)                               │
│  2. Client sends refresh token                               │
│  3. Server validates + rotates refresh token                 │
│  4. Server issues new access + refresh tokens                │
│  5. Old refresh token invalidated                            │
└──────────────────────────────────────────────────────────────┘
```

### Token Storage (Frontend)
- **Access Token**: In-memory only (Zustand store)
- **Refresh Token**: HttpOnly cookie (if same-origin) or secure localStorage
- **Never** in URL parameters or non-HttpOnly cookies

### Key Management
- RS256 asymmetric signing (RSA 2048-bit minimum)
- Public key published at `/.well-known/jwks.json`
- Key rotation every 90 days with overlap period
- Keys stored in Kubernetes Secrets (or Vault in production)

---

## RBAC Model

```
Role: admin
  ├── Manage all events (create, stop, archive)
  ├── Manage all users (create, delete, role change)
  ├── Manage challenges (CRUD)
  ├── View all teams and labs
  ├── Freeze/unfreeze scoreboard
  └── Access system metrics

Role: organizer
  ├── Manage own events
  ├── View teams in own events
  ├── Freeze scoreboard for own events
  └── Cannot manage users or system settings

Role: participant
  ├── Join events and teams
  ├── View challenges in joined events
  ├── Submit flags
  ├── View scoreboard
  ├── Access own lab
  └── Download VPN config for own team
```

### Permission Matrix

| Resource | Admin | Organizer | Participant |
|----------|-------|-----------|-------------|
| Events: Create | ✅ | ✅ | ❌ |
| Events: List | ✅ (all) | ✅ (own) | ✅ (public) |
| Events: Delete | ✅ | ❌ | ❌ |
| Teams: Create | ✅ | ✅ | ✅ |
| Teams: View | ✅ (all) | ✅ (event) | ✅ (own) |
| Challenges: CRUD | ✅ | ✅ | ❌ |
| Challenges: View | ✅ | ✅ | ✅ (event) |
| Flags: Submit | ❌ | ❌ | ✅ |
| Lab: Access | ✅ (any) | ✅ (event) | ✅ (own) |
| Users: Manage | ✅ | ❌ | ❌ |
| Scoreboard: Freeze | ✅ | ✅ (event) | ❌ |

---

## Network Security

### Zero-Trust Networking

```
┌──────────────────────────────────────────────────────────┐
│ Kubernetes Cluster                                        │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ orchi-frontend namespace                             │ │
│  │  ┌─────────┐                                        │ │
│  │  │Frontend │ ← Only ingress traffic allowed         │ │
│  │  └─────────┘                                        │ │
│  │  NetworkPolicy: ingress from Ingress only           │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ orchi-system namespace                               │ │
│  │  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐            │ │
│  │  │ API  │  │ Auth │  │ Core │  │ Lab  │            │ │
│  │  │ GW   │→ │ Svc  │  │ Svc  │  │ Svc  │            │ │
│  │  └──────┘  └──────┘  └──────┘  └──────┘            │ │
│  │  NetworkPolicy: API GW ← Ingress only              │ │
│  │  NetworkPolicy: Services ← API GW only             │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ orchi-store namespace                                │ │
│  │  ┌──────┐                                           │ │
│  │  │Store │ ← Only from orchi-system                  │ │
│  │  └──────┘                                           │ │
│  │  NetworkPolicy: ingress from orchi-system only      │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ orchi-lab-{id} namespace (per team)                  │ │
│  │  ┌──────┐ ┌──────┐ ┌──────┐                        │ │
│  │  │Kali  │ │Ex1   │ │Ex2   │                        │ │
│  │  └──────┘ └──────┘ └──────┘                        │ │
│  │  NetworkPolicy: no egress to other namespaces       │ │
│  │  NetworkPolicy: intra-namespace only                │ │
│  └─────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### NetworkPolicy Examples

```yaml
# Default deny all in orchi-system
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: orchi-system
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress

# Allow API Gateway from Ingress
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-api-gateway-ingress
  namespace: orchi-system
spec:
  podSelector:
    matchLabels:
      app: orchi-api-gateway
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-system
      ports:
        - port: 8080
```

---

## Internal Service Authentication (mTLS)

For service-to-service communication within the cluster:

1. **Option A: Service Mesh (Istio/Linkerd)**
   - Automatic mTLS between all services
   - Certificate rotation handled by mesh
   - Zero code changes required

2. **Option B: Manual mTLS** (simpler deployments)
   - cert-manager for certificate lifecycle
   - Each service has its own TLS certificate
   - Mutual TLS verification on gRPC connections

Recommended: **Linkerd** for lightweight mTLS with minimal overhead.

---

## Secrets Management

### Kubernetes Secrets (Minimum)
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: orchi-jwt-keys
  namespace: orchi-system
type: Opaque
data:
  private-key.pem: <base64>
  public-key.pem: <base64>
```

### Production: External Secrets Operator
```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: orchi-jwt-keys
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: ClusterSecretStore
  target:
    name: orchi-jwt-keys
  data:
    - secretKey: private-key.pem
      remoteRef:
        key: orchi/jwt/private-key
```

---

## Input Validation and Sanitization

- All API inputs validated with JSON Schema
- Flag submissions: max 256 characters, alphanumeric + `{}_-!`
- Team names: max 32 characters, sanitized for XSS
- Usernames: max 32 characters, alphanumeric + `_-`
- Passwords: minimum 8 characters, bcrypt cost factor 12

---

## Audit Logging

Every security-relevant action is logged:

```json
{
  "timestamp": "2026-03-01T14:30:00Z",
  "event": "flag.submit",
  "actor": {
    "user_id": "uuid",
    "team_id": "uuid",
    "ip": "203.0.113.1"
  },
  "resource": {
    "type": "challenge",
    "id": "uuid",
    "name": "SQL Injection 101"
  },
  "result": "success",
  "details": {
    "points_awarded": 100,
    "first_blood": false
  }
}
```

Logged events:
- Authentication (login, logout, token refresh, failed attempts)
- Authorization failures
- Flag submissions (correct and incorrect)
- Lab operations (create, reset, destroy)
- Admin actions (event CRUD, user management)
- Rate limit triggers

---

## DDoS Mitigation

1. **Ingress Level**: Traefik rate limiting middleware
2. **API Level**: Per-IP and per-user rate limiting
3. **WebSocket**: Connection limits per user/IP
4. **Kubernetes**: Resource quotas per namespace
5. **External**: Cloudflare/AWS Shield for volumetric attacks

---

## CSRF Protection

Since the frontend is a separate SPA using JWT in Authorization headers:
- No cookies used for authentication → CSRF not applicable for API calls
- `SameSite=Strict` on any session cookies
- CORS restricts origins to known frontend domains
- Custom `X-Request-ID` header for request tracing

---

## Content Security Policy

Frontend serves CSP headers:
```
Content-Security-Policy:
  default-src 'self';
  script-src 'self';
  style-src 'self' 'unsafe-inline';
  img-src 'self' data: https:;
  font-src 'self';
  connect-src 'self' https://api.orchi.io wss://api.orchi.io;
  frame-src 'self' https://guac.orchi.io;
  base-uri 'self';
  form-action 'self';
```
