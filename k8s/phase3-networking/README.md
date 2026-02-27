# Phase 3 — Networking & Security

Migrate all networking — lab isolation (iptables → NetworkPolicy), DNS (Docker CoreDNS → cluster CoreDNS ConfigMaps), VPN (external WireGuard gRPC → K8s WireGuard Deployment), and external access (port bindings → Ingress).

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Kubernetes Cluster                                 │
│                                                                             │
│  ┌───── External Traffic ──────────────────────────────────────────────────┐ │
│  │                                                                        │ │
│  │    Internet ──► Ingress (nginx) ──► orchi.cicibogaz.com                 │ │
│  │                  │  ├── /           → Amigo (challenge frontend)        │ │
│  │                  │  └── /guacamole  → Guacamole (remote desktop)        │ │
│  │                  │                                                      │ │
│  │    VPN Clients ──► LoadBalancer:51820 ──► WireGuard Pod                 │ │
│  │                                           │                             │ │
│  └───────────────────────────────────────────┼─────────────────────────────┘ │
│                                              │                               │
│  ┌───── orchi-system namespace ──────────────┼─────────────────────────────┐ │
│  │  orchi-operator │ Amigo │ Guacamole │ WireGuard                        │ │
│  └─────────────────┼───────┼───────────┼──────────────────────────────────┘ │
│                    │       │           │                                     │
│  ┌───── orchi-lab-{id} namespace ─────┼────────────────────────────────────┐ │
│  │                                    │                                    │ │
│  │  ┌──────────────────────────────────────────────────────────────────┐   │ │
│  │  │  NetworkPolicies                                                 │   │ │
│  │  │  ┌────────────────┐  ┌──────────────┐  ┌─────────────────────┐  │   │ │
│  │  │  │ default-deny   │  │ intra-lab    │  │ vpn-ingress         │  │   │ │
│  │  │  │ (block all)    │  │ (pod↔pod)    │  │ (WG→challenges)    │  │   │ │
│  │  │  └────────────────┘  └──────────────┘  └─────────────────────┘  │   │ │
│  │  │  ┌──────────────────┐  ┌──────────────────┐                     │   │ │
│  │  │  │ guacamole-access │  │ operator-access   │                    │   │ │
│  │  │  │ (RDP to frontend)│  │ (mgmt from oper.) │                    │   │ │
│  │  │  └──────────────────┘  └──────────────────┘                     │   │ │
│  │  └──────────────────────────────────────────────────────────────────┘   │ │
│  │                                                                         │ │
│  │  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐             │ │
│  │  │ challenge-a    │  │ challenge-b    │  │  frontend VM   │             │ │
│  │  │ (Pod)          │  │ (Pod)          │  │  (Pod)         │             │ │
│  │  └────────────────┘  └────────────────┘  └────────────────┘             │ │
│  │                                                                         │ │
│  │  ┌───────────────────────────────────────────────┐                      │ │
│  │  │ lab-dns-records ConfigMap                      │                      │ │
│  │  │ (CoreDNS zone file for lab-internal DNS)       │                      │ │
│  │  └───────────────────────────────────────────────┘                      │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Files

| File | Purpose |
|------|---------|
| `networkpolicy-default-deny.yaml` | Block all traffic by default in lab namespaces |
| `networkpolicy-intra-lab.yaml` | Allow challenge pods to talk to each other + DNS |
| `networkpolicy-vpn-ingress.yaml` | Allow WireGuard VPN traffic into lab challenges |
| `networkpolicy-guacamole-access.yaml` | Allow Guacamole RDP to lab frontend pods |
| `networkpolicy-operator-access.yaml` | Allow operator to manage lab pods |
| `wireguard-deployment.yaml` | WireGuard VPN gateway — Deployment, Service, ConfigMap, Secret, PVC |
| `lab-dns-configmap.yaml` | Lab-internal DNS records via CoreDNS ConfigMap |
| `ingress.yaml` | HTTPS Ingress for Amigo and Guacamole with TLS |

## Legacy-to-Kubernetes Mapping

### Network Isolation (iptables → NetworkPolicy)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `ipTab.createRejectRule(labSubnet)` | `networkpolicy-default-deny.yaml` — block all by default |
| `ipTab.createAcceptRule(labSubnet, vpnIPs)` | `networkpolicy-vpn-ingress.yaml` — allow from WG pod |
| `ipTab.createStateRule(labSubnet)` | K8s NetworkPolicy is stateful by default |
| `ipTab.removeRejectRule(labSubnet)` | Delete namespace removes all NetworkPolicies |
| `ipTab.removeAcceptRule(labSubnet, vpnIps)` | Delete VPN NetworkPolicy |
| `docker.NewNetwork(isVPN)` | Namespace + NetworkPolicies (bridge/macvlan → CNI) |
| `docker.Network.Connect(container)` | Pod networking (automatic in same namespace) |
| `docker.Network.Interface()` | CNI plugin handles interfaces |
| `docker.DefaultLinkBridge` | Not needed — Services replace bridge aliases |

### DNS (Docker CoreDNS → Cluster CoreDNS ConfigMap)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `dns.New(records)` | `lab-dns-configmap.yaml` — operator creates ConfigMap |
| `dns.Server.Run(ctx)` | Cluster CoreDNS reads ConfigMap automatically |
| `dns.Server.Container()` | No container — cluster CoreDNS handles it |
| `dns.RR{Name, Type, RData}` | Zone file entries in ConfigMap `data.zonefile` |
| `dns.PreferedIP` (3) | Service ClusterIP (auto-assigned) |
| `dns.Server.Close()` | Delete ConfigMap |
| DHCP (dhcp.New) | Not needed — K8s assigns pod IPs automatically |

### VPN (gRPC WireGuard → K8s WireGuard Deployment)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `wg.WireGuardConfig` | ConfigMap `wireguard-config` + Secret `wireguard-secret` |
| `wg.NewGRPCVPNClient()` | Operator manages WireGuard config directly (no gRPC) |
| `wg.Creds` (gRPC auth) | ServiceAccount (internal), Secret (external keys) |
| `daemon.Config.VPNConn` | ConfigMap vars (SERVERURL, etc.) |
| `store.VpnConn` | Team CR `.status.vpnConfig` |
| `store.Team.vpnKeys` | WireGuard peer keys in Secret per team |
| External WireGuard service | `wireguard-deployment.yaml` in orchi-system |

### External Access (Port Bindings → Ingress)

| Legacy (Go code) | Kubernetes Resource |
|---|---|
| `daemon.Config.Host.Http` | Ingress host rule |
| `daemon.Config.Port.Secure` | Ingress HTTPS (443) via TLS |
| `daemon.Config.Port.InSecure` | Ingress HTTP (80) → redirect |
| `daemon.Config.Certs` | cert-manager annotation on Ingress |
| `guacamole.webPort` (random) | Fixed Service port 8080 → Ingress `/guacamole` |
| Direct port bindings | Ingress path-based routing |

## NetworkPolicy Strategy

The operator creates these policies in every lab namespace:

1. **`default-deny-all`** — blocks all ingress and egress (baseline)
2. **`allow-intra-lab`** — challenge pods can talk to each other + DNS
3. **`allow-vpn-ingress`** — (if VPN required) WireGuard pod → challenges
4. **`allow-guacamole-access`** — guacd → frontend VMs on port 3389
5. **`allow-operator-access`** — operator → all pods for management

Traffic flow:
```
Internet → Ingress → Amigo/Guacamole (orchi-system)
VPN Clients → LoadBalancer → WireGuard (orchi-system) → Lab pods (orchi-lab-*)
Guacamole guacd → Lab frontend pods (RDP 3389)
Lab challenge pod ↔ Lab challenge pod (same namespace)
Lab pod → kube-system CoreDNS (UDP 53)
```

## Deployment Order

### Step 1 — Deploy WireGuard
```bash
kubectl apply -f wireguard-deployment.yaml
```

### Step 2 — Deploy Ingress
```bash
kubectl apply -f ingress.yaml
```

### Step 3 — Operator applies per-lab policies
When the operator creates a lab namespace, it applies:
```bash
kubectl apply -f networkpolicy-default-deny.yaml -n orchi-lab-{id}
kubectl apply -f networkpolicy-intra-lab.yaml -n orchi-lab-{id}
kubectl apply -f networkpolicy-operator-access.yaml -n orchi-lab-{id}
kubectl apply -f networkpolicy-guacamole-access.yaml -n orchi-lab-{id}
# Only if VPN is required for this event:
kubectl apply -f networkpolicy-vpn-ingress.yaml -n orchi-lab-{id}
```

### Step 4 — Operator creates DNS ConfigMap
```bash
kubectl apply -f lab-dns-configmap.yaml -n orchi-lab-{id}
```

### Step 5 — Verify
```bash
# Check NetworkPolicies
kubectl -n orchi-lab-example get networkpolicies

# Check DNS resolution inside a lab pod
kubectl -n orchi-lab-example exec -it challenge-sql-injection-xxx -- nslookup xss-basic.lab.orchi.local

# Check WireGuard
kubectl -n orchi-system exec -it wireguard-xxx -- wg show

# Check Ingress
kubectl -n orchi-system get ingress
curl -k https://orchi.cicibogaz.com/healthz
```

## Rollback Strategy

| Step | Action |
|------|--------|
| 1 | Remove Ingress: `kubectl delete ingress orchi-ingress -n orchi-system` |
| 2 | Re-enable legacy port bindings in daemon config |
| 3 | Remove NetworkPolicies from lab namespaces (all traffic allowed by default) |
| 4 | Re-enable iptables rules via `IPTables.createRejectRule()` |
| 5 | Scale down K8s WireGuard: `kubectl scale deploy wireguard -n orchi-system --replicas=0` |
| 6 | Re-enable external WireGuard gRPC service |

## Risks and Mitigation

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| CNI plugin doesn't enforce NetworkPolicy | Critical | Low | Verify CNI supports NetworkPolicy (Calico, Cilium, Weave — not Flannel) |
| Default-deny blocks legitimate traffic | High | Medium | Test each policy individually; add policies incrementally |
| WireGuard NET_ADMIN capability is a security concern | Medium | Low | Isolate WireGuard pod; apply strict RBAC; use dedicated node pool |
| CoreDNS ConfigMap not picked up by cluster DNS | Medium | Low | Use `kubebuilder` or custom CoreDNS plugin to watch ConfigMaps |
| Ingress controller WebSocket timeout for Guacamole | Medium | Medium | Set `proxy-read-timeout: 3600` annotation |
| LoadBalancer IP changes for WireGuard | Medium | Low | Use static IP annotation or DNS with short TTL |
| DHCP removal breaks legacy VMs | Low | Low | DHCP is only needed for VBox VMs; K8s pods use CNI IPAM |
