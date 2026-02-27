<p align="center"> 
<img src=".github/logo/blue240px.png"  />
<div align="center">
  <a href="https://github.com/mrtrkmn/orchi/releases">
    <img src="https://img.shields.io/github/v/release/mrtrkmn/orchi?style=flat-square" alt="GitHub release">
  </a>
   <a href="https://www.gnu.org/licenses/gpl-3.0">
    <img src="https://img.shields.io/badge/License-GPLv3-blue.svg?longCache=true&style=flat-square" alt="licence">
  </a>
  <a href="https://github.com/mrtrkmn/orchi/issues">
    <img src="https://img.shields.io/github/issues/mrtrkmn/orchi?style=flat-square" alt="issues">
  </a>
 </div>
&nbsp;
<div align="center">
<h1>Orchi</h1>
</div>

Orchi is a Kubernetes-native platform for security education. It orchestrates CTF (Capture The Flag) environments by managing challenge containers, team isolation, VPN access, and remote desktop sessions — all on Kubernetes.

## Architecture

Orchi uses a Kubernetes operator pattern:

- **CRDs** define Events, Labs, Teams, and Challenges as first-class resources
- **Operator** watches CRDs and reconciles the desired state (namespaces, pods, network policies)
- **Amigo** is the web frontend for teams to submit flags and track progress
- **Guacamole** provides browser-based remote desktop access to lab VMs
- **WireGuard** provides VPN access to lab environments
- **Store** persists event and team data as a StatefulSet with PVC

```
Internet ──► Ingress ──► Amigo (flag submission UI)
                    └──► Guacamole (remote desktop)
VPN ──────► WireGuard LoadBalancer ──► Lab pods

orchi-system namespace:
  ├── orchi-operator (watches CRDs, reconciles labs)
  ├── amigo (web frontend, HPA 2-10 replicas)
  ├── guacamole (guacd + web + MySQL)
  ├── wireguard (VPN gateway)
  ├── orchi-store (StatefulSet, gRPC on :5454)
  └── observability (Prometheus ServiceMonitors, Grafana)

orchi-lab-{id} namespaces (one per lab):
  ├── challenge pods (isolated per team)
  ├── NetworkPolicies (default-deny + allow rules)
  ├── ResourceQuota + LimitRange
  └── CoreDNS zone ConfigMap
```

## Quick Start

### Prerequisites

- Kubernetes 1.25+
- kubectl
- [Kustomize](https://kustomize.io/) (built into kubectl)
- A CNI plugin that supports NetworkPolicy (Calico, Cilium)

### Deploy

```bash
# Development
kubectl apply -k k8s/overlays/dev

# Staging
kubectl apply -k k8s/overlays/staging

# Production
kubectl apply -k k8s/overlays/prod
```

### Create an Event

```yaml
apiVersion: orchi.cicibogaz.com/v1alpha1
kind: Event
metadata:
  name: ctf-2024
spec:
  tag: ctf-2024
  name: "CTF Competition 2024"
  capacity: 50
  lab:
    exercises:
      - sql-injection
      - xss-basic
      - buffer-overflow
```

```bash
kubectl apply -f event.yaml
kubectl get events.orchi.cicibogaz.com
```

### Verify

```bash
kubectl -n orchi-system get pods
kubectl get events.orchi.cicibogaz.com
kubectl get labs.orchi.cicibogaz.com
kubectl get teams.orchi.cicibogaz.com -n orchi-system
```

## Project Structure

```
k8s/                    # Kubernetes manifests (CRDs, workloads, networking, observability)
daemon/                 # Operator / daemon core logic
client/                 # CLI client
svcs/amigo/             # Amigo web frontend (flag submission UI)
svcs/guacamole/         # Guacamole integration
store/                  # Data persistence layer
exercise/               # Exercise/challenge definitions
network/                # Network and VPN management
virtual/                # Container runtime abstraction
logging/                # Structured logging
```

See [`k8s/README.md`](k8s/README.md) for the full Kubernetes manifest documentation, including CRD schemas, deployment order, network policy strategy, and observability setup.

## Development

```bash
# Get dependencies
go mod download

# Run tests
go test -v --race ./...

# Build
go build -o orchi ./main.go
```

## Contributing

Contributions are welcome. See [Contributing Guide](.github/CONTRIBUTING.md).

## License

[GPLv3](LICENSE)

Copyright (c) 2019-present, Orchi
