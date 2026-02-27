# Kubernetes Migration — orchi Platform

Kubernetes manifests for migrating the orchi platform from Docker + legacy daemon to Kubernetes-native resources.

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              Kubernetes Cluster                                  │
│                                                                                  │
│  ┌───── External Traffic ──────────────────────────────────────────────────────┐  │
│  │   Internet ──► Ingress (nginx) ──► orchi.cicibogaz.com                     │  │
│  │                 │  ├── /           → Amigo (challenge frontend)            │  │
│  │                 │  └── /guacamole  → Guacamole (remote desktop)            │  │
│  │   VPN Clients ──► LoadBalancer:51820 ──► WireGuard Pod                     │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌───── orchi-system namespace ────────────────────────────────────────────────┐  │
│  │                                                                            │  │
│  │  ┌─────────────────┐  ┌──────────┐  ┌──────────────┐  ┌──────────────┐     │  │
│  │  │  orchi-operator  │  │  Amigo   │  │  Guacamole   │  │  WireGuard   │     │  │
│  │  │  (Deployment)    │  │ (Deploy) │  │  (Deploy)    │  │  (Deploy)    │     │  │
│  │  │  Watches CRDs    │  │  HPA     │  │  guacd + web │  │  VPN gateway │     │  │
│  │  └────────┬─────────┘  └──────────┘  │  + MySQL     │  └──────────────┘     │  │
│  │           │                          └──────────────┘                       │  │
│  │  ┌────────┴─────────┐  ┌──────────────────────────────────────────────┐     │  │
│  │  │  orchi-store      │  │  Observability                               │     │  │
│  │  │  (StatefulSet)    │  │  ServiceMonitors → Prometheus → Grafana      │     │  │
│  │  │  gRPC on :5454    │  │  PrometheusRules → AlertManager              │     │  │
│  │  │  PVC: 5Gi         │  └──────────────────────────────────────────────┘     │  │
│  │  └──────────────────┘                                                       │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌───── orchi-lab-{id} namespace (one per lab) ────────────────────────────────┐  │
│  │                                                                            │  │
│  │  ┌──────────────────────────────────────────────────────────┐               │  │
│  │  │  NetworkPolicies                                         │               │  │
│  │  │  default-deny → intra-lab → vpn-ingress → guac-access   │               │  │
│  │  └──────────────────────────────────────────────────────────┘               │  │
│  │                                                                            │  │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────────────┐    │  │
│  │  │ challenge-a│  │ challenge-b│  │ challenge-c│  │ ResourceQuota      │    │  │
│  │  │ (Pod)      │  │ (Pod)      │  │ (Pod)      │  │ LimitRange         │    │  │
│  │  └────────────┘  └────────────┘  └────────────┘  │ (enforced limits)  │    │  │
│  │                                                  └────────────────────┘    │  │
│  │  ┌────────────────────────────────────────────┐                            │  │
│  │  │ lab-dns-records ConfigMap (CoreDNS zone)   │                            │  │
│  │  └────────────────────────────────────────────┘                            │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
│  ┌───── Cluster-scoped CRDs ──────────────────────────────────────────────────┐  │
│  │  Event CR ──► Lab CR (1:many) ──► Team CRs (1:many)                       │  │
│  │  Challenge CRDs (per lab namespace)                                        │  │
│  └────────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

## Directory Layout

```
k8s/
├── crds/                       # Custom Resource Definitions
│   ├── challenge-crd.yaml      # Challenge CRD (replaces store.Exercise)
│   ├── event-crd.yaml          # Event CRD (replaces store.EventConfig)
│   ├── lab-crd.yaml            # Lab CRD (replaces lab.Lab interface)
│   └── team-crd.yaml           # Team CRD (replaces store.Team)
│
├── base/                       # Cluster setup, RBAC, operator, resource controls
│   ├── namespace.yaml          # Lab namespace template with pod security labels
│   ├── orchi-operator-rbac.yaml        # ServiceAccount, ClusterRole, ClusterRoleBinding
│   ├── orchi-operator-deployment.yaml  # Operator Deployment + ConfigMap + Secret
│   ├── resource-quotas.yaml    # ResourceQuota + LimitRange per lab namespace
│   └── poddisruptionbudget.yaml # PDBs for critical services
│
├── workloads/                  # Application deployments and services
│   ├── challenge-deployment.yaml   # Challenge pod template
│   ├── challenge-service.yaml      # ClusterIP per challenge
│   ├── challenge-configmap.yaml    # Non-sensitive challenge config
│   ├── challenge-secret.yaml       # Flags and credentials
│   ├── amigo-deployment.yaml       # Amigo flag submission frontend
│   ├── guacamole-deployment.yaml   # Guacamole remote desktop stack
│   ├── wireguard-deployment.yaml   # WireGuard VPN gateway
│   ├── store-statefulset.yaml      # Store database (StatefulSet + PVC)
│   └── hpa.yaml                    # HorizontalPodAutoscalers (Amigo, Guacamole)
│
├── networking/                 # Network policies, ingress, DNS
│   ├── networkpolicy-default-deny.yaml     # Block all (baseline)
│   ├── networkpolicy-intra-lab.yaml        # Pod-to-pod + DNS within lab
│   ├── networkpolicy-vpn-ingress.yaml      # WireGuard → lab challenges
│   ├── networkpolicy-guacamole-access.yaml # guacd → frontend RDP
│   ├── networkpolicy-operator-access.yaml  # Operator → all lab pods
│   ├── ingress.yaml                        # HTTPS ingress (Amigo + Guacamole)
│   └── lab-dns-configmap.yaml              # CoreDNS zone file per lab
│
├── observability/              # Monitoring, alerting, dashboards
│   ├── prometheus-servicemonitor.yaml      # ServiceMonitors for all components
│   ├── prometheus-rules.yaml               # Alerting rules (health, capacity, resources)
│   └── grafana-dashboard-configmap.yaml    # Grafana dashboard (auto-provisioned)
│
└── README.md                   # This file
```

## Legacy-to-Kubernetes Mapping

### Container Runtime (Docker → Kubernetes)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `docker.NewContainer(conf)` | `Deployment` + `Pod` |
| `docker.ContainerConfig.Image` | `spec.containers[].image` |
| `docker.ContainerConfig.EnvVars` | `ConfigMap` + `Secret` env refs |
| `docker.Resources{MemoryMB, CPU}` | `resources.requests` / `resources.limits` |
| `docker.ContainerConfig.DNS` | CoreDNS (cluster DNS, automatic) |
| `docker.ContainerConfig.Labels` | `metadata.labels` |
| `docker.Network.Connect(container)` | Pod networking (same namespace = automatic) |
| `store.Exercise` | `Challenge` CRD |
| `exercise.Environment` | Kubernetes Namespace (one per lab) |

### Orchestration (Daemon → Operator)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `daemon.New(config)` / `daemon.Run()` | Operator Deployment + controller manager |
| `daemon.Close()` | Graceful shutdown via SIGTERM |
| `daemon.Config` | ConfigMap + Secret refs |
| `daemon.eventPool` | Event CR watch + reconciliation |
| `daemon gRPC port 5454` | K8s API server (CRD CRUD replaces gRPC) |
| `daemon SIGHUP reload` | ConfigMap watch triggers reconcile |
| `store.EventConfig` | `Event` CRD `.spec` |
| `store.Event.SetStatus()` | Operator updates `Event` CR `.status.phase` |
| `lab.LabHost.NewLab()` | Operator creates `Lab` CR → reconciles namespace + pods |
| `store.Team` / `store.TeamStore` | `Team` CRD, K8s API (list/get/create/delete) |
| `guacamole.New()` / `guacamole.create()` | Guacamole Deployment (guacd + web + MySQL) |
| `svcs/amigo` | Amigo Deployment |

### Network Isolation (iptables → NetworkPolicy)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `ipTab.createRejectRule(labSubnet)` | `networkpolicy-default-deny.yaml` |
| `ipTab.createAcceptRule(labSubnet, vpnIPs)` | `networkpolicy-vpn-ingress.yaml` |
| `ipTab.createStateRule(labSubnet)` | K8s NetworkPolicy (stateful by default) |
| `docker.NewNetwork(isVPN)` | Namespace + NetworkPolicies |
| `dns.New(records)` | `lab-dns-configmap.yaml` (CoreDNS zone file) |
| `dns.Server.Run(ctx)` | Cluster CoreDNS reads ConfigMap automatically |
| External WireGuard gRPC service | `wireguard-deployment.yaml` in orchi-system |
| `daemon.Config.Host.Http` | Ingress host rule |
| `daemon.Config.Port.Secure` | Ingress HTTPS (443) via TLS |
| `daemon.Config.Certs` | cert-manager annotation on Ingress |

### Persistence (gRPC Store → StatefulSet)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `store.NewGRPClientDBConnection()` | In-cluster Service `orchi-store:5454` |
| `daemon.Config.Database` | ConfigMap + Secret env vars |
| `ServiceConfig.Grpc` endpoint | K8s Service DNS: `orchi-store.orchi-system.svc` |
| `ServiceConfig.CertConfig` | Not needed in-cluster (mTLS via service mesh) |
| File-based persistence (EventsDir) | PVC mounted at `/data` |

### Observability (Polling → Prometheus + Grafana)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `daemon.MonitorHost()` CPU/Mem stream | Prometheus + node_exporter + cAdvisor |
| File-based logging (`logging.Pool`) | stdout/stderr → log collector (Loki/ELK) |
| `logging.NewPool(dir)` | Container-native JSON logging to stdout |
| No metrics endpoint | `/metrics` on each component (Prometheus) |
| No alerting | PrometheusRule alert definitions |
| No dashboard | Grafana dashboard (auto-provisioned ConfigMap) |
| No resource quotas | ResourceQuota + LimitRange per lab namespace |
| No autoscaling | HPA for Amigo and Guacamole |
| No disruption protection | PodDisruptionBudgets for critical services |

### Resource Management

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `ExerciseInstanceConfig.MemoryMB` | LimitRange default memory |
| `ExerciseInstanceConfig.CPU` | LimitRange default CPU |
| 50MB minimum memory validation | LimitRange min: 64Mi |
| In-memory pool size limits | ResourceQuota pod count |
| `SetFrontendMemory()` / `SetFrontendCpu()` | Update Challenge CR `.spec.resources` |
| No HA / single daemon | PDB minAvailable: 1 |

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
      │     └── status.challengeStatuses: [{tag: "sql-injection", phase: "Running"}]
      │
      └── Team CRs (namespaced in orchi-system)
            └── spec.eventRef: "ctf-2024"
            └── status.labRef: "lab-abc123-team-alpha"
            └── status.challenges: [{tag: "sql-injection", completedAt: "..."}]
```

## Operator Reconciliation Flow

```
1. User creates Event CR
   └── Operator reconciles:
       ├── Validates spec (capacity, exercises)
       ├── Creates lab namespace: orchi-lab-{event-tag}
       ├── Applies ResourceQuota + LimitRange
       ├── Deploys Challenge CRs into namespace
       ├── Applies NetworkPolicies
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
kubectl apply -f k8s/crds/
```

### Step 2 — Deploy base resources (RBAC, operator)
```bash
kubectl apply -f k8s/base/orchi-operator-rbac.yaml
kubectl apply -f k8s/base/orchi-operator-deployment.yaml
kubectl apply -f k8s/base/poddisruptionbudget.yaml
```

### Step 3 — Deploy workloads
```bash
kubectl apply -f k8s/workloads/store-statefulset.yaml
kubectl apply -f k8s/workloads/amigo-deployment.yaml
kubectl apply -f k8s/workloads/guacamole-deployment.yaml
kubectl apply -f k8s/workloads/wireguard-deployment.yaml
kubectl apply -f k8s/workloads/hpa.yaml
```

### Step 4 — Deploy networking
```bash
kubectl apply -f k8s/networking/ingress.yaml
# NetworkPolicies and DNS ConfigMaps are applied per-lab by the operator
```

### Step 5 — Deploy observability (requires prometheus-operator)
```bash
kubectl apply -f k8s/observability/
```

### Step 6 — Create an event
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

### Step 7 — Verify
```bash
# CRDs
kubectl get crds | grep orchi

# Operator and workloads
kubectl -n orchi-system get pods
kubectl -n orchi-system get statefulsets

# Events, labs, teams
kubectl get events.orchi.cicibogaz.com
kubectl get labs.orchi.cicibogaz.com
kubectl get teams.orchi.cicibogaz.com -n orchi-system

# Observability
kubectl -n orchi-system get servicemonitors
kubectl -n orchi-system get prometheusrules

# Lab namespace
kubectl -n orchi-lab-example get pods,svc,networkpolicies,resourcequotas
```

## Namespace Strategy

**One namespace per lab.** Every lab instance gets a dedicated namespace:

```
orchi-lab-{lab-id}
```

- All challenge pods, services, secrets, and configmaps live inside the lab namespace
- Deleting the namespace cascades to delete all contained resources
- ResourceQuota + LimitRange are applied per namespace
- NetworkPolicies enforce isolation per namespace

**Required labels on every lab namespace:**
```yaml
labels:
  app.kubernetes.io/managed-by: orchi-operator
  app.kubernetes.io/part-of: orchi
  orchi.cicibogaz.com/lab-id: "{lab-id}"
  orchi.cicibogaz.com/component: lab
```

## NetworkPolicy Strategy

The operator creates these policies in every lab namespace:

1. **`default-deny-all`** — blocks all ingress and egress (baseline)
2. **`allow-intra-lab`** — challenge pods can talk to each other + DNS
3. **`allow-vpn-ingress`** — (if VPN required) WireGuard pod → challenges
4. **`allow-guacamole-access`** — guacd → frontend VMs on port 3389
5. **`allow-operator-access`** — operator → all pods for management

## Observability Stack

| Component | Purpose |
|-----------|---------|
| ServiceMonitors | Tell Prometheus which endpoints to scrape |
| PrometheusRules | Alert when operator is down, pods not ready, resources exhausted |
| Grafana Dashboard | Visual overview of operator health, lab count, resource usage |
| HPA | Auto-scale Amigo (2–10) and Guacamole (1–5) based on CPU |
| PDB | Prevent voluntary disruptions from taking down critical services |
| ResourceQuota | Cap total resource usage per lab namespace |
| LimitRange | Set default and max resource limits per container |

## Rollback Strategy

| Step | Action |
|------|--------|
| 1 | Stop creating new Events via CRD path |
| 2 | Scale operator to 0: `kubectl -n orchi-system scale deploy orchi-operator --replicas=0` |
| 3 | Existing labs continue running (pods are not deleted when operator stops) |
| 4 | Re-enable legacy daemon with `daemon.Run()` |
| 5 | Re-enable legacy iptables rules via `IPTables.createRejectRule()` |
| 6 | CRDs remain installed but inert without operator |

**Key point:** The legacy Go code (`exercise.NewExercise()`, `docker.NewContainer()`) is not deleted during migration. It is feature-flagged. Rolling back means flipping the flag.

## Risks and Mitigation

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Challenge image incompatible with non-root | Medium | Medium | Test with `runAsUser: 1000`; emptyDir for `/tmp` |
| CNI plugin doesn't enforce NetworkPolicy | Critical | Low | Verify CNI (Calico, Cilium — not Flannel) |
| Default-deny blocks legitimate traffic | High | Medium | Apply policies incrementally; test each one |
| Operator crash loses in-flight reconciliation | Medium | Low | Leader election + idempotent reconcile |
| WireGuard NET_ADMIN capability risk | Medium | Low | Isolate pod; strict RBAC; dedicated node pool |
| Too many namespaces (one per team per event) | Low | Medium | Monitor count; consider shared namespaces |
| Resource limits too tight cause OOMKill | Medium | Medium | Start generous (1Gi); tune from metrics |
| Store StatefulSet data loss | High | Low | PVC with backup; snapshot schedule |
| HPA thrashing during load spikes | Low | Medium | Stabilization windows (60s up, 300s down) |
| CRD schema migration (v1alpha1 → v1beta1) | Medium | High | Conversion webhooks; plan evolution early |
