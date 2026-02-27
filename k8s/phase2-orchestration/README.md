# Phase 2 — Lab Orchestration

Migrate the orchi daemon, event/lab/team state, and supporting services (Guacamole, Amigo) into Kubernetes-native resources.

## Architecture

```
┌────────────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                                 │
│                                                                            │
│  ┌──────────── Namespace: orchi-system ──────────────────────────────────┐  │
│  │                                                                      │  │
│  │  ┌─────────────────┐  ┌──────────┐  ┌───────────────────────────┐    │  │
│  │  │  orchi-operator  │  │  Amigo   │  │       Guacamole           │    │  │
│  │  │  (Deployment)    │  │ (Deploy) │  │  ┌───────┐ ┌──────────┐  │    │  │
│  │  │                  │  │          │  │  │ guacd │ │   web    │  │    │  │
│  │  │  Watches:        │  │  Flag    │  │  └───────┘ └──────────┘  │    │  │
│  │  │  - Event CRs     │  │  submit  │  │  ┌──────────────────┐    │    │  │
│  │  │  - Lab CRs       │  │  UI      │  │  │   guacamole-db   │    │    │  │
│  │  │  - Team CRs      │  │          │  │  │   (MySQL)        │    │    │  │
│  │  │  - Challenge CRs │  │          │  │  └──────────────────┘    │    │  │
│  │  └────────┬─────────┘  └──────────┘  └───────────────────────────┘    │  │
│  │           │                                                           │  │
│  └───────────┼───────────────────────────────────────────────────────────┘  │
│              │ reconciles                                                   │
│              ▼                                                              │
│  ┌──────────── Namespace: orchi-lab-{id} ───────────────────────────────┐  │
│  │                                                                      │  │
│  │  Challenge Pods   +   Services   +   Secrets   +   ConfigMaps        │  │
│  │  (from Phase 1)                                                      │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                            │
│  ┌──────────── Cluster-scoped CRDs ─────────────────────────────────────┐  │
│  │  Event CR ──► Lab CR ──► Team CRs                                    │  │
│  │              (1:many)    (1:many)                                     │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
```

## Files

| File | Purpose |
|------|---------|
| `event-crd.yaml` | Event CRD — top-level orchestration unit (maps from `store.EventConfig`) |
| `lab-crd.yaml` | Lab CRD — runtime lab instance per team (maps from `lab.Lab`) |
| `team-crd.yaml` | Team CRD — participant state, challenges solved, VPN config (maps from `store.Team`) |
| `orchi-operator-rbac.yaml` | ServiceAccount, ClusterRole, ClusterRoleBinding for the operator |
| `orchi-operator-deployment.yaml` | Operator Deployment + ConfigMap + Secret (replaces `daemon`) |
| `guacamole-deployment.yaml` | Guacamole stack — guacd, web, MySQL, Services, Secret |
| `amigo-deployment.yaml` | Amigo challenge frontend — Deployment + Service |

## CRD Relationship Model

```
Event CR (cluster-scoped)
  └── spec.lab.exercises: ["sql-injection", "xss-basic", ...]
  └── status.labNamespace: "orchi-lab-abc123"
      │
      ├── Lab CR (cluster-scoped, one per team)
      │     └── spec.eventRef: "ctf-2024"
      │     └── spec.teamRef: "team-alpha"
      │     └── status.namespace: "orchi-lab-abc123-team-alpha"
      │     └── status.challengeStatuses: [{tag: "sql-injection", phase: "Running"}, ...]
      │
      └── Team CRs (namespaced in orchi-system)
            └── spec.eventRef: "ctf-2024"
            └── status.labRef: "lab-abc123-team-alpha"
            └── status.challenges: [{tag: "sql-injection", completedAt: "..."}]
```

## Legacy-to-Kubernetes Mapping

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `store.EventConfig` | `Event` CRD `.spec` |
| `store.Event` | `Event` CR + `Lab` CR |
| `store.Event.SetStatus()` | Operator updates `Event` CR `.status.phase` |
| `store.Event.Finish()` | Operator sets `Event` `.status.phase: Closed` |
| `store.EventConfig.Lab` | `Event` CRD `.spec.lab` → `Lab` CRD `.spec` |
| `store.EventConfig.OnlyVPN` | `Event` CRD `.spec.vpn.required` |
| `store.EventConfig.SecretKey` | `Event` CRD `.spec.secretKeyRef` → K8s Secret |
| `lab.Lab` interface | `Lab` CRD lifecycle (Start → Running, Stop → Stopped) |
| `lab.LabHost.NewLab()` | Operator creates `Lab` CR → reconciles namespace + pods |
| `lab.Config.Frontends` | `Lab` CRD `.spec.frontends` |
| `store.Team` | `Team` CRD |
| `store.TeamStore` | K8s API (list/get/create/delete Team CRs) |
| `store.Team.VerifyFlag()` | Operator validates flag → updates `Team` `.status.challenges` |
| `store.Team.vpnConf` | `Team` `.status.vpnConfig` |
| `store.Team.isLabAssigned` | `Team` `.status.labAssigned` |
| `daemon.New(config)` | Operator Deployment startup |
| `daemon.Run()` | Controller manager `Run()` |
| `daemon.eventPool` | Event CR watch + reconciliation |
| `daemon.Config` | ConfigMap `orchi-operator-config` + Secret `orchi-operator-secrets` |
| `guacamole.New()` | Guacamole Deployment in `orchi-system` |
| `guacamole.create()` | Deployment + Service (guacd + web + db) |
| `svcs/amigo` | Amigo Deployment in `orchi-system` |

## Operator Reconciliation Flow

```
1. User creates Event CR
   └── Operator reconciles:
       ├── Validates spec (capacity, exercises, etc.)
       ├── Creates lab namespace: orchi-lab-{event-tag}
       ├── Deploys Challenge CRs (from Phase 1) into namespace
       ├── Configures Guacamole RDP connections
       └── Sets Event status.phase = Running

2. Team registers (via Amigo or API)
   └── Operator reconciles:
       ├── Creates Team CR in orchi-system
       ├── Creates Lab CR for this team
       ├── Lab CR reconciles into:
       │   ├── Namespace: orchi-lab-{event}-{team}
       │   ├── Challenge Deployments + Services
       │   ├── Frontend VM pods
       │   └── Network policies
       └── Updates Team status.labAssigned = true

3. Team submits flag (via Amigo)
   └── Operator reconciles:
       ├── Validates flag against Challenge CR
       ├── Updates Team status.challenges[].completedAt
       └── Updates Team status.solvedCount

4. Event finishes
   └── Operator reconciles:
       ├── Sets Event status.phase = Closed
       ├── Deletes all Lab CRs (cascades to namespaces)
       └── Archives Team data
```

## Deployment Order

### Step 1 — Install CRDs
```bash
kubectl apply -f event-crd.yaml
kubectl apply -f lab-crd.yaml
kubectl apply -f team-crd.yaml
# Phase 1 CRD
kubectl apply -f ../phase1-containers/challenge-crd.yaml
```

### Step 2 — Deploy operator RBAC
```bash
kubectl apply -f orchi-operator-rbac.yaml
```

### Step 3 — Deploy operator
```bash
kubectl apply -f orchi-operator-deployment.yaml
```

### Step 4 — Deploy supporting services
```bash
kubectl apply -f guacamole-deployment.yaml
kubectl apply -f amigo-deployment.yaml
```

### Step 5 — Create an event
```yaml
apiVersion: orchi.cicibogaz.com/v1alpha1
kind: Event
metadata:
  name: ctf-2024
spec:
  tag: ctf-2024
  name: "CTF Competition 2024"
  host: ctf.cicibogaz.com
  capacity: 50
  createdBy: admin
  lab:
    exercises:
      - sql-injection
      - xss-basic
      - buffer-overflow
    frontends:
      - image: registry.cicibogaz.com/orchi/frontends/kali:2024.1
        memoryMB: 4096
        cpu: 2.0
```

### Step 6 — Verify
```bash
kubectl get events.orchi.cicibogaz.com
kubectl get labs.orchi.cicibogaz.com
kubectl get teams.orchi.cicibogaz.com -n orchi-system
kubectl -n orchi-system get pods
```

## Rollback Strategy

| Step | Action |
|------|--------|
| 1 | Stop creating new Events via CRD path |
| 2 | Scale operator to 0: `kubectl -n orchi-system scale deploy orchi-operator --replicas=0` |
| 3 | Existing labs continue running (pods are not deleted when operator stops) |
| 4 | Re-enable legacy daemon with `daemon.Run()` |
| 5 | CRDs remain installed but inert without operator |

## Risks and Mitigation

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Operator crash loses in-flight reconciliation | Medium | Low | Leader election + idempotent reconcile; K8s retries automatically |
| Team state lost during operator restart | High | Low | All state is in CRs (etcd), not in-memory; operator is stateless |
| Guacamole DB corruption | Medium | Low | PVC with backup strategy; MySQL replication in production |
| Too many namespaces (one per team per event) | Low | Medium | Monitor namespace count; consider shared namespaces with NetworkPolicy |
| CRD schema migration (v1alpha1 → v1beta1) | Medium | High | Use conversion webhooks; plan schema evolution early |
| gRPC store client removal breaks existing tooling | Medium | Medium | Keep gRPC server as adapter layer; operator proxies to K8s API |
