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
├── kustomization.yaml          # Kustomize base configuration
├── README.md                   # This file
│
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
│   ├── poddisruptionbudget.yaml        # PDBs for critical services
│   ├── pod-security.yaml       # Pod Security Admission namespace labels
│   ├── feature-flags.yaml      # Feature flags for gradual cutover
│   └── external-secrets.yaml   # ExternalSecret CRs (production secret management)
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
│   ├── hpa.yaml                    # HorizontalPodAutoscalers (Amigo, Guacamole)
│   ├── backup-cronjob.yaml         # Store backup CronJob (S3 upload)
│   ├── velero-schedule.yaml        # Velero cluster backup schedules
│   └── migration-job.yaml          # One-time data migration Job
│
├── networking/                 # Network policies, ingress, DNS
│   ├── networkpolicy-default-deny.yaml     # Block all (baseline)
│   ├── networkpolicy-intra-lab.yaml        # Pod-to-pod + DNS within lab
│   ├── networkpolicy-vpn-ingress.yaml      # WireGuard → lab challenges
│   ├── networkpolicy-guacamole-access.yaml # guacd → frontend RDP
│   ├── networkpolicy-operator-access.yaml  # Operator → all lab pods
│   ├── networkpolicy-store-access.yaml     # Operator → store (restrict access)
│   ├── ingress.yaml                        # HTTPS ingress (Amigo + Guacamole)
│   └── lab-dns-configmap.yaml              # CoreDNS zone file per lab
│
├── observability/              # Monitoring, alerting, dashboards
│   ├── prometheus-servicemonitor.yaml      # ServiceMonitors for all components
│   ├── prometheus-rules.yaml               # Alerting rules (health, capacity, resources)
│   └── grafana-dashboard-configmap.yaml    # Grafana dashboard (auto-provisioned)
│
└── overlays/                   # Kustomize environment overlays
    ├── dev/kustomization.yaml      # Dev — reduced resources, local registry
    ├── staging/kustomization.yaml  # Staging — staging endpoints, moderate scale
    └── prod/kustomization.yaml     # Prod — full capacity, backups, external secrets
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

### Security (Manual → Pod Security + External Secrets)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `daemon.Config.SigningKey` in config.yml | ExternalSecret → AWS/Vault/GCP secret store |
| `daemon.Config.APICreds` in config.yml | ExternalSecret → rotated credentials |
| Hardcoded Secret YAML with `REPLACE_BEFORE_DEPLOY` | ExternalSecret CRs pull from vault |
| `ServiceConfig.CertConfig` TLS certs | cert-manager + in-cluster mTLS |
| No pod security enforcement | Pod Security Admission (restricted/baseline) |
| gRPC TLS auth for store access | NetworkPolicy restricts store access to operator |

### Backup & Disaster Recovery

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| No automated backup | CronJob `store-backup` (every 6h → S3) |
| `daemon.Config.ConfFiles.EventsDir` on disk | Velero full backup (daily, 7-day retention) |
| Manual filesystem backup | Velero CRD-only backup (every 4h, 3-day retention) |
| No disaster recovery plan | Velero restore from S3 + PVC snapshots |

### CI/CD & Deployment (Ansible → Kustomize)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `scripts/ansible/main.yml` playbook | Kustomize overlays (dev/staging/prod) |
| `scripts/deploy/deploy.sh` SSH deploy | `kubectl apply -k k8s/overlays/prod` |
| `daemon.NewConfigFromFile(path)` | ConfigMap per environment via Kustomize |
| `.goreleaser.yml` binary builds | Container image builds (CI pipeline) |
| Single `config.yml` for all settings | Kustomize base + overlay patches |

### Cutover (Legacy → Kubernetes)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| Feature-flagged code paths | `orchi-feature-flags` ConfigMap |
| `daemon.New(config)` initialization | Operator startup (same binary, K8s path) |
| `store.NewGRPClientDBConnection()` | Migration Job converts data to CRs |
| In-memory `eventPool` state | Event CRs in etcd (persistent) |
| `store.Team` gRPC CRUD | Team CRs via K8s API |

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

### Using Kustomize (recommended)

```bash
# Development
kubectl apply -k k8s/overlays/dev

# Staging
kubectl apply -k k8s/overlays/staging

# Production (includes backups, external secrets)
kubectl apply -k k8s/overlays/prod
```

### Manual step-by-step

#### Step 1 — Install CRDs
```bash
kubectl apply -f k8s/crds/
```

#### Step 2 — Deploy base resources (RBAC, operator, security)
```bash
kubectl apply -f k8s/base/orchi-operator-rbac.yaml
kubectl apply -f k8s/base/pod-security.yaml
kubectl apply -f k8s/base/feature-flags.yaml
kubectl apply -f k8s/base/orchi-operator-deployment.yaml
kubectl apply -f k8s/base/poddisruptionbudget.yaml
# Production only:
kubectl apply -f k8s/base/external-secrets.yaml
```

#### Step 3 — Deploy workloads
```bash
kubectl apply -f k8s/workloads/store-statefulset.yaml
kubectl apply -f k8s/workloads/amigo-deployment.yaml
kubectl apply -f k8s/workloads/guacamole-deployment.yaml
kubectl apply -f k8s/workloads/wireguard-deployment.yaml
kubectl apply -f k8s/workloads/hpa.yaml
```

#### Step 4 — Deploy networking
```bash
kubectl apply -f k8s/networking/ingress.yaml
kubectl apply -f k8s/networking/networkpolicy-store-access.yaml
# Lab NetworkPolicies and DNS ConfigMaps are applied per-lab by the operator
```

#### Step 5 — Deploy observability (requires prometheus-operator)
```bash
kubectl apply -f k8s/observability/
```

#### Step 6 — Migrate data (one-time, during cutover)
```bash
# Update migration-config and migration-secrets with legacy store credentials
kubectl apply -f k8s/workloads/migration-job.yaml
# Monitor progress:
kubectl -n orchi-system logs -f job/orchi-data-migration
```

#### Step 7 — Deploy backup schedules (production)
```bash
kubectl apply -f k8s/workloads/backup-cronjob.yaml
kubectl apply -f k8s/workloads/velero-schedule.yaml
```

#### Step 8 — Create an event
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
      - image: ghcr.io/mrtrkmn/orchi/frontends/kali:2024.1
        memoryMB: 4096
        cpu: 2.0
```

#### Step 9 — Verify
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

# Feature flags
kubectl -n orchi-system get configmap orchi-feature-flags -o yaml

# Backups
kubectl -n orchi-system get cronjobs
kubectl -n velero get schedules

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
6. **`allow-store-access`** — operator → store on port 5454 only

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

## Security

| Component | Purpose |
|-----------|---------|
| Pod Security Admission | Enforce restricted/baseline profiles per namespace |
| ExternalSecret CRs | Pull secrets from AWS/Vault/GCP instead of hardcoded YAML |
| NetworkPolicy (store) | Restrict store access to operator only |
| cert-manager | Automatic TLS certificate provisioning for Ingress |
| ReadOnlyRootFilesystem | All containers use read-only root filesystem |
| Non-root execution | All containers run as non-root (UID 65534 or 1000) |

## Backup & Disaster Recovery

| Component | Purpose |
|-----------|---------|
| `store-backup` CronJob | Every 6h backup of store PVC to S3 |
| `orchi-full-backup` Velero | Daily full backup (namespaces + CRDs + PVCs), 7-day retention |
| `orchi-crd-backup` Velero | Every 4h CRD-only backup (lightweight), 3-day retention |

## Kustomize Overlays

| Overlay | Differences from base |
|---------|----------------------|
| `dev` | Reduced resources, single replicas, local registry, 1Gi store PVC |
| `staging` | Staging endpoints, moderate scaling, production mode enabled |
| `prod` | Full capacity, external secrets, backups, 20Gi store PVC |

## Feature Flags (Cutover Strategy)

The `orchi-feature-flags` ConfigMap controls the gradual migration:

```
FEATURE_K8S_ENABLED           = true   # Master switch
FEATURE_K8S_LABS              = true   # K8s namespaces instead of Docker
FEATURE_K8S_NETWORK_POLICIES  = true   # NetworkPolicies instead of iptables
FEATURE_K8S_VPN               = true   # K8s WireGuard instead of gRPC
FEATURE_K8S_STORE             = true   # K8s store instead of external gRPC
FEATURE_K8S_DNS               = true   # CoreDNS ConfigMaps instead of dns.Server
FEATURE_K8S_GUACAMOLE         = true   # K8s Guacamole instead of daemon-managed
FEATURE_METRICS_ENABLED       = true   # Prometheus metrics
```

**Cutover process:**
1. Start with all flags `false` (legacy mode)
2. Enable one flag at a time, validate, then move to next
3. Once all flags `true`, legacy code paths are unused
4. Remove legacy code after stabilization period

**Rollback:** Set any flag back to `false` to revert that subsystem to legacy mode.

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
| Store StatefulSet data loss | High | Low | PVC with backup; snapshot schedule; Velero |
| HPA thrashing during load spikes | Low | Medium | Stabilization windows (60s up, 300s down) |
| CRD schema migration (v1alpha1 → v1beta1) | Medium | High | Conversion webhooks; plan evolution early |
| External Secrets Operator unavailable | Medium | Low | Fallback to manual K8s Secrets; ESO is optional |
| Data migration corrupts legacy data | High | Low | Migration Job is read-only on source; verify counts |
| Feature flag misconfiguration | Medium | Medium | Default all flags to `false`; enable one at a time |
| Backup CronJob fails silently | Medium | Medium | PrometheusRule alerts on CronJob failures |
| Kustomize overlay drift | Low | Medium | CI validation of `kustomize build` for each overlay |
