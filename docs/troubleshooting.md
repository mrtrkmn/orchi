# Troubleshooting

Common issues and solutions for the orchi platform on Kubernetes.

## Operator Issues

### Operator Pod Not Starting

```bash
kubectl -n orchi-system describe pod -l app.kubernetes.io/name=orchi-operator
kubectl -n orchi-system logs -l app.kubernetes.io/name=orchi-operator
```

Common causes:
- Missing ConfigMap or Secret
- RBAC permissions not applied (`kubectl apply -f k8s/base/orchi-operator-rbac.yaml`)
- Image pull errors (check registry credentials)

### CRDs Not Recognized

```bash
kubectl get crds | grep orchi
```

If missing, install them:
```bash
kubectl apply -f k8s/crds/
```

## Lab Issues

### Challenge Pods Not Starting

```bash
kubectl -n orchi-lab-{id} get pods
kubectl -n orchi-lab-{id} describe pod {pod-name}
kubectl -n orchi-lab-{id} logs {pod-name}
```

Common causes:
- Resource quota exceeded (`kubectl -n orchi-lab-{id} describe resourcequota`)
- Image pull errors
- Security context violations (non-root requirement)

### Network Connectivity Issues

Verify NetworkPolicies:
```bash
kubectl -n orchi-lab-{id} get networkpolicies
```

Ensure your CNI plugin supports NetworkPolicy (Calico, Cilium — not Flannel).

### DNS Resolution Failures

Check the lab DNS ConfigMap:
```bash
kubectl -n orchi-lab-{id} get configmap lab-dns-records -o yaml
```

## Amigo Issues

### Flag Submission Not Working

```bash
kubectl -n orchi-system logs -l app.kubernetes.io/name=amigo
```

Check that the Amigo pod can reach the operator:
```bash
kubectl -n orchi-system exec -it deploy/amigo -- wget -qO- http://orchi-operator:8080/healthz
```

## Store Issues

### Store Pod CrashLooping

```bash
kubectl -n orchi-system describe statefulset orchi-store
kubectl -n orchi-system logs statefulset/orchi-store
```

Check PVC status:
```bash
kubectl -n orchi-system get pvc
```

## Guacamole Issues

### Remote Desktop Not Connecting

```bash
kubectl -n orchi-system logs -l app.kubernetes.io/name=guacamole -c guacd
kubectl -n orchi-system logs -l app.kubernetes.io/name=guacamole -c guacamole-web
```

Check that the guacd pod can reach challenge frontend pods via NetworkPolicy.

## VPN Issues

### WireGuard Not Accepting Connections

```bash
kubectl -n orchi-system logs -l app.kubernetes.io/name=wireguard
kubectl -n orchi-system get svc wireguard
```

Verify the LoadBalancer has an external IP:
```bash
kubectl -n orchi-system get svc wireguard -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

## Observability

### No Metrics in Prometheus

Verify ServiceMonitors are discovered:
```bash
kubectl -n orchi-system get servicemonitors
```

Check that prometheus-operator is installed and the ServiceMonitor label selectors match.

## Filing Issues

If none of the above resolve your problem, [create an issue](https://github.com/mrtrkmn/orchi/issues/new).
