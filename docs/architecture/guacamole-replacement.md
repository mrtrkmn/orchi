# Guacamole Replacement — Remote Desktop Architecture

## Executive Summary

This document analyzes alternatives to Apache Guacamole for providing browser-based
remote desktop access to KubeVirt VMs in the Orchi CTF platform, selects the
best-fit replacement, and provides a complete architecture and migration plan.

---

## 1. Current Architecture: Apache Guacamole

### How Guacamole Works Today

```
Browser ──── HTTPS ────▶ Ingress ──▶ Amigo ──▶ Guacamole Web (Tomcat)
                                                      │
                                                      ▼
                                              guacd (proxy daemon)
                                                      │
                                                  RDP (3389)
                                                      │
                                                      ▼
                                              KubeVirt VM (Kali/Ubuntu)
```

**Components:**
- `guacamole-web` (Java/Tomcat) — web app, user management, connection management
- `guacd` (C daemon) — protocol translator (RDP/VNC/SSH → WebSocket)
- `MySQL` — stores users, connections, session data
- Go proxy in `svcs/guacamole/` — reverse-proxies WebSocket traffic

**Problems with Current Setup:**
1. **Heavy stack**: 3 containers (web + guacd + MySQL) per Guacamole instance
2. **Guacamole manages its own users/sessions** — duplicates auth done by Orchi
3. **Scaling is complex**: guacd is single-process, no native horizontal scaling
4. **MySQL dependency**: stateful database per instance, backup/recovery overhead
5. **RDP dependency**: requires xrdp or Windows RDP on every VM image
6. **Legacy Docker integration**: `svcs/guacamole/guacamole.go` still uses Docker APIs
7. **Configuration overhead**: each team requires API calls to create Guacamole users
   and RDP connections (CreateUser, CreateRDPConn, addConnectionToUser)
8. **Not Kubernetes-native**: designed for standalone deployment, not K8s operators

---

## 2. Alternatives Analysis

### 2.1 noVNC + websockify

**Architecture:** JavaScript VNC client + WebSocket-to-TCP proxy.

| Aspect | Assessment |
|--------|-----------|
| Protocol | VNC (natively available in KubeVirt via `virtctl vnc`) |
| Client | Pure JavaScript, no plugins |
| Server | `websockify` — Python/C WebSocket-to-TCP bridge |
| K8s Native | ✅ Works directly with KubeVirt VNC endpoints |
| Auth | Requires external auth (pair with JWT middleware) |
| Scaling | Stateless proxy — scales horizontally trivially |
| Complexity | Low — 1 container (websockify), no database |
| Maturity | Stable, widely used in OpenStack Horizon, Proxmox |

**Strengths:** Eliminates RDP, eliminates MySQL, eliminates Tomcat. Works natively
with KubeVirt's built-in VNC support.

**Weaknesses:** VNC image quality can be lower than RDP at high resolutions;
no built-in file transfer; no built-in audio.

---

### 2.2 KubeVirt VNC Proxy (virtctl proxy)

**Architecture:** KubeVirt's built-in VNC subresource exposed via API server.

| Aspect | Assessment |
|--------|-----------|
| Protocol | VNC via Kubernetes API subresource |
| Client | noVNC (JavaScript) or any VNC viewer |
| Server | KubeVirt API server handles WebSocket upgrade |
| K8s Native | ✅✅ Native KubeVirt feature, no extra components |
| Auth | Uses Kubernetes RBAC + bearer tokens |
| Scaling | Scales with Kubernetes API server |
| Complexity | Minimal — built into KubeVirt |
| Maturity | Production-ready, core KubeVirt feature |

**Strengths:** Zero additional server components. VNC is exposed as a K8s API
subresource (`/apis/subresources.kubevirt.io/v1/namespaces/{ns}/virtualmachineinstances/{name}/vnc`).
Authentication is handled via Kubernetes service account tokens or OIDC.

**Weaknesses:** Requires careful RBAC to limit users to their own VMs;
API server becomes the bottleneck; no built-in session recording.

---

### 2.3 Apache Guacamole (Keep/Upgrade)

| Aspect | Assessment |
|--------|-----------|
| Protocol | RDP, VNC, SSH, Telnet |
| K8s Native | ❌ Not designed for K8s; can be containerized |
| Auth | Built-in (duplicates Orchi auth) |
| Scaling | guacd is single-process; limited horizontal scaling |
| Complexity | High — 3 containers + MySQL + user/connection management |
| Maturity | Very mature, widely adopted |

**Strengths:** Multi-protocol, session recording, file transfer, audio support.
**Weaknesses:** All problems listed in Section 1 remain.

---

### 2.4 Selkies-GStreamer (formerly neko)

**Architecture:** WebRTC-based remote desktop streaming.

| Aspect | Assessment |
|--------|-----------|
| Protocol | WebRTC (video/audio) + data channels |
| Client | JavaScript, hardware-accelerated |
| Server | GStreamer pipeline inside the VM |
| K8s Native | ✅ Designed for Kubernetes (Google Cloud) |
| Auth | External (OIDC, JWT) |
| Scaling | One streamer per VM (sidecar model) |
| Complexity | Medium — requires GStreamer on each VM |
| Maturity | Emerging — used in Google Cloud Workstations |

**Strengths:** Best video quality (WebRTC adaptive bitrate), audio support,
clipboard, GPU acceleration support.

**Weaknesses:** Requires GStreamer agent in every VM image; higher CPU usage;
still relatively new; modifying VM images adds operational burden.

---

### 2.5 NICE DCV / Parsec

Commercial solutions — not suitable for open-source CTF platform. **Rejected.**

---

### 2.6 Comparison Summary

| Feature | noVNC+websockify | KubeVirt VNC Proxy | Guacamole | Selkies |
|---------|------------------|--------------------|-----------|---------|
| Extra containers | 1 (websockify) | 0 | 3 + MySQL | 1 per VM |
| Database needed | No | No | Yes (MySQL) | No |
| K8s native | ✅ | ✅✅ | ❌ | ✅ |
| Auth integration | External | K8s RBAC | Built-in | External |
| Horizontal scaling | Trivial | API server | Complex | Per-VM |
| Protocol to VM | VNC | VNC | RDP | WebRTC |
| VM modification | None | None | xrdp required | GStreamer agent |
| Session recording | Custom | Custom | Built-in | Custom |
| File transfer | No | No | Yes | Clipboard |
| Audio | No | No | Yes | Yes |
| 500+ sessions | ✅ | ✅ (with RBAC) | ⚠️ (guacd limit) | ✅ |

---

## 3. Recommendation: noVNC + websockify with KubeVirt VNC API

### Why This Architecture

The recommended approach combines **noVNC** (browser VNC client) with a
**Go-based VNC WebSocket proxy** that leverages KubeVirt's native VNC
subresource API. This eliminates the need for separate websockify instances
by building VNC proxying directly into the Orchi API gateway.

**Justification:**

1. **Eliminates 3 containers + MySQL** — Guacamole's entire stack is removed
2. **No VM modifications** — KubeVirt VMs already expose VNC; no xrdp needed
3. **No duplicate auth** — Uses Orchi's existing JWT authentication
4. **Stateless** — No database for session/connection management
5. **Horizontally scalable** — Proxy is stateless; scale with HPA
6. **Kubernetes-native** — Uses KubeVirt API to discover and connect to VMs
7. **Minimal operational overhead** — One Deployment replaces three + MySQL

---

## 4. Target Architecture

```
                         Browser (noVNC JavaScript client)
                                    │
                                    │ WSS (WebSocket Secure)
                                    ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Kubernetes Ingress (Traefik)                   │
│  Route: wss://desktop.cyberorch.com/vnc/{lab-namespace}/{vm-name}    │
│  TLS termination, rate limiting                                  │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                   VNC Proxy Service (Go)                         │
│                   Deployment: orchi-vnc-proxy                    │
│                   Namespace: orchi-system                        │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  1. Authenticate: Validate JWT from WebSocket handshake   │  │
│  │  2. Authorize: Check user owns the lab namespace          │  │
│  │  3. Discover: Query KubeVirt API for VMI VNC endpoint     │  │
│  │  4. Proxy: WebSocket ↔ KubeVirt VNC subresource           │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Replicas: 3–10 (HPA on CPU + active connections)                │
│  No database. No user management. Stateless.                     │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               │ KubeVirt API
                               │ /apis/subresources.kubevirt.io/v1/
                               │   namespaces/{ns}/
                               │   virtualmachineinstances/{name}/vnc
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│              KubeVirt VMI (lab namespace)                         │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                      │
│  │ Kali VM  │  │Ubuntu VM │  │Parrot VM │                      │
│  │ VNC :5900│  │ VNC :5900│  │ VNC :5900│                      │
│  └──────────┘  └──────────┘  └──────────┘                      │
│                                                                  │
│  Namespace: orchi-lab-{event-id}-{team-id}                      │
│  NetworkPolicy: only VNC proxy can reach port 5900              │
└──────────────────────────────────────────────────────────────────┘
```

---

## 5. Component Design

### 5.1 VNC Proxy Service (Go)

The VNC proxy is a lightweight Go service that:

1. **Accepts WebSocket connections** from the noVNC client in the browser
2. **Validates JWT tokens** from the `Authorization` header or WebSocket subprotocol
3. **Checks authorization** — ensures the authenticated user/team owns the
   target lab namespace
4. **Opens a VNC connection** to the KubeVirt VMI via the Kubernetes API
5. **Bridges traffic** — copies bytes bidirectionally between the browser
   WebSocket and the KubeVirt VNC WebSocket

**Key Design Decisions:**
- **No database**: Connection info is derived from KubeVirt API at runtime
- **No user management**: Uses Orchi's existing JWT claims (team_id, event_id)
- **Stateless**: Any proxy replica can handle any connection
- **Service Account**: Uses a K8s ServiceAccount with RBAC to access VNC
  subresources in lab namespaces

**API Endpoints:**

```
WebSocket: /vnc/{namespace}/{vmi-name}
  Headers:
    Authorization: ******

  Response: WebSocket upgrade → VNC stream

GET /healthz        → 200 OK
GET /readyz         → 200 OK (checks KubeVirt API reachability)
GET /metrics        → Prometheus metrics
```

**Metrics Exposed:**
- `vnc_proxy_active_connections` (gauge) — current active VNC sessions
- `vnc_proxy_connections_total` (counter) — total connections established
- `vnc_proxy_connection_duration_seconds` (histogram) — session duration
- `vnc_proxy_auth_failures_total` (counter) — authentication failures
- `vnc_proxy_bytes_transferred_total` (counter) — bandwidth usage

---

### 5.2 noVNC Frontend Integration

The frontend embeds the noVNC JavaScript client as a component:

```typescript
// frontend/src/components/DesktopViewer.tsx
interface DesktopViewerProps {
  labNamespace: string;
  vmName: string;
  token: string;
}

// The noVNC client connects to:
// wss://desktop.cyberorch.com/vnc/{labNamespace}/{vmName}
// and includes the JWT via the WebSocket subprotocol, e.g.:
// new WebSocket(url, ['auth', token])
```

noVNC is served as static assets from the frontend container, eliminating
the need for a separate Guacamole web application.

---

### 5.3 Authentication Flow

```
1. User logs in via Orchi frontend → receives JWT
2. User clicks "Open Desktop" for a VM
3. Frontend opens noVNC component with WebSocket URL:
   wss://desktop.cyberorch.com/vnc/orchi-lab-evt1-team42/kali-vm
   JWT is sent via WebSocket subprotocol (not in URL)
4. VNC Proxy validates JWT:
   - Checks signature (RS256 with Orchi's public key)
   - Extracts claims: { team_id: "team42", event_id: "evt1", role: "participant" }
5. VNC Proxy checks authorization:
   - Target namespace: orchi-lab-evt1-team42
   - User's event_id + team_id must match the namespace pattern
6. VNC Proxy queries KubeVirt API:
   GET /apis/subresources.kubevirt.io/v1/namespaces/orchi-lab-evt1-team42/
       virtualmachineinstances/kali-vm/vnc
   → Returns WebSocket connection to VM's VNC
7. VNC Proxy bridges browser WebSocket ↔ KubeVirt VNC WebSocket
```

---

### 5.4 Lab Isolation

**Namespace isolation:** Each lab has its own namespace
(`orchi-lab-{event}-{team}`). The VNC proxy enforces that a user can only
connect to VMs in their own lab namespace by validating JWT claims against
the target namespace.

**Network isolation:** NetworkPolicies restrict VNC access:
- Only the VNC proxy (in `orchi-system`) can reach port 5900 on lab VMs
- Lab VMs cannot reach other lab namespaces
- Direct VNC access from the internet is blocked

**RBAC isolation:** The VNC proxy's ServiceAccount has a ClusterRole that
allows VNC subresource access only to specific namespaces (dynamically
bound per lab via the operator).

---

### 5.5 Horizontal Scaling

```
┌─────────────────────────────────────────────────────┐
│  HPA: orchi-vnc-proxy                                │
│                                                     │
│  Metric: vnc_proxy_active_connections               │
│  Target: 50 connections per replica                  │
│                                                     │
│  Min replicas: 2                                    │
│  Max replicas: 20                                   │
│                                                     │
│  For 500 concurrent sessions:                       │
│  500 / 50 = 10 replicas                             │
│                                                     │
│  Each replica: ~100MB RAM, ~200m CPU idle            │
│  Under load: ~500MB RAM, ~1 CPU per 50 sessions    │
└─────────────────────────────────────────────────────┘
```

The proxy is **stateless** — any replica can handle any session. The Ingress
distributes connections across replicas. WebSocket sticky sessions are not
required because each connection is independent.

---

## 6. Kubernetes Deployment Model

### 6.1 VNC Proxy Deployment

See `k8s/workloads/vnc-proxy-deployment.yaml` for the complete manifest.

**Key resources:**
- `Deployment`: orchi-vnc-proxy (2–20 replicas)
- `Service`: orchi-vnc-proxy (ClusterIP, port 8443)
- `ServiceAccount`: orchi-vnc-proxy (with KubeVirt VNC RBAC)
- `ClusterRole`: orchi-vnc-proxy (VNC subresource access)
- `ClusterRoleBinding`: orchi-vnc-proxy
- `HorizontalPodAutoscaler`: CPU + custom metrics
- `PodDisruptionBudget`: minAvailable 1

### 6.2 Ingress

```yaml
# Added to k8s/networking/ingress-decoupled.yaml
- host: desktop.cyberorch.com
  http:
    paths:
      - path: /vnc
        pathType: Prefix
        backend:
          service:
            name: orchi-vnc-proxy
            port:
              number: 8443
```

### 6.3 NetworkPolicy

See `k8s/networking/networkpolicy-vnc-proxy.yaml` — replaces
`networkpolicy-guacamole-access.yaml`.

### 6.4 Namespace Strategy

No change — lab namespaces remain `orchi-lab-{event}-{team}`.
The VNC proxy runs in `orchi-system`.

---

## 7. Migration Plan

### Phase 1: Deploy VNC Proxy (Parallel with Guacamole)

1. Deploy `orchi-vnc-proxy` in `orchi-system`
2. Add noVNC component to frontend
3. Add VNC Ingress route at `desktop.cyberorch.com`
4. Configure NetworkPolicy for VNC proxy → lab VMs (port 5900)
5. Feature flag: `ENABLE_VNC_PROXY=true` in operator

**Validation:** Manually test VNC connections to lab VMs.
Both Guacamole and VNC Proxy are available.

### Phase 2: Switch Default to VNC Proxy

1. Update frontend to default to noVNC viewer
2. Keep Guacamole available at `/guacamole` (fallback)
3. Monitor VNC proxy metrics for stability
4. Run for 2+ events to validate at scale

### Phase 3: Remove Guacamole

1. Remove Guacamole deployment (`guacamole-deployment.yaml`)
2. Remove Guacamole DB (`guacamole-db`, PVC, Secret)
3. Remove Guacamole NetworkPolicy (`networkpolicy-guacamole-access.yaml`)
4. Remove Guacamole Go code (`svcs/guacamole/`)
5. Remove Guacamole references from daemon/event pool
6. Remove Guacamole routes from Amigo
7. Remove xrdp from VM images (VNC is sufficient)

### Phase 4: Cleanup

1. Remove Guacamole from monitoring (ServiceMonitor, PrometheusRules)
2. Remove Guacamole from feature flags
3. Update documentation
4. Remove Guacamole from CI/CD pipelines

---

## 8. Rollback Strategy

### During Phase 1–2 (Guacamole Still Running)

**Rollback:** Set feature flag `ENABLE_VNC_PROXY=false`. Frontend reverts
to Guacamole-based desktop access. Zero downtime.

### During Phase 3 (After Guacamole Removal)

**Rollback:** Re-apply Guacamole Kubernetes manifests from git history.
The manifests are self-contained (Deployment + Service + Secret + PVC).
Recovery time: ~5 minutes.

```bash
# Restore Guacamole from git
git show HEAD~N:k8s/workloads/guacamole-deployment.yaml | kubectl apply -f -
git show HEAD~N:k8s/networking/networkpolicy-guacamole-access.yaml | kubectl apply -f -
```

### Data Loss Risk

**None.** Guacamole's MySQL stores user/connection configuration that is
auto-generated by the operator. No user data is lost. The operator can
re-create all Guacamole users and connections from the Store's event/team data.

---

## 9. Comparison: Before and After

| Aspect | Before (Guacamole) | After (VNC Proxy) |
|--------|-------------------|-------------------|
| Containers | 3 (web + guacd + MySQL) | 1 (vnc-proxy) |
| Database | MySQL (stateful) | None |
| Auth | Guacamole internal + Orchi proxy | JWT (Orchi native) |
| Protocol to VM | RDP (requires xrdp) | VNC (KubeVirt native) |
| Scaling model | guacd bottleneck | Stateless HPA |
| VM image changes | xrdp required | None |
| Connection setup | API calls per user/VM | Zero (runtime discovery) |
| User management | Guacamole users per team | None (JWT claims) |
| Session recording | Guacamole built-in | Custom (future) |
| File transfer | Guacamole drive mapping | Separate mechanism |
| Resource usage | ~2GB RAM minimum | ~200MB per replica |

---

## 10. Future Enhancements

1. **Session recording**: Record VNC streams for anti-cheat review
2. **Clipboard integration**: Add clipboard data channel via custom protocol
3. **Performance monitoring**: Track frame rate and latency per session
4. **Multi-monitor**: Support multiple VM desktops in tabs
5. **SSH terminal**: Add xterm.js for SSH access alongside VNC
6. **WebRTC upgrade**: If video quality becomes a concern, add optional
   WebRTC streaming (Selkies) as a sidecar per VM
