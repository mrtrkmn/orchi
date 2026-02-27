# Deployment Guide — Kubernetes Native

## Overview

This guide covers the Kubernetes deployment model for the decoupled Orchi
platform. Each component is independently deployable with its own scaling
policy, health checks, and resource constraints.

---

## Namespace Strategy

```
orchi-system/           Core services (API Gateway, Auth, Core, Lab)
orchi-frontend/         Frontend static SPA
orchi-store/            Store StatefulSet and backups
orchi-monitoring/       Prometheus, Grafana, Loki, AlertManager
orchi-ingress/          Ingress controller (Traefik)
orchi-lab-<event-id>/   Per-event lab namespaces (auto-created by operator)
```

Each namespace has:
- ResourceQuota limiting total CPU/memory
- LimitRange enforcing per-pod defaults
- NetworkPolicy with default-deny
- RBAC restricting service account permissions

---

## Architecture Diagram

```
                         Internet
                            │
                            ▼
              ┌─────────────────────────┐
              │   Traefik Ingress       │
              │   (orchi-ingress)       │
              │   TLS, Rate Limit       │
              └──────┬──────────┬───────┘
                     │          │
          ┌──────────▼──┐  ┌───▼──────────────┐
          │  Frontend   │  │  API Gateway      │
          │  (nginx)    │  │  (Go, chi router) │
          │  orchi-     │  │  orchi-system      │
          │  frontend   │  │                   │
          └─────────────┘  └──────┬────────────┘
                                  │ gRPC
                    ┌─────────────┼─────────────┐
                    │             │             │
              ┌─────▼─────┐ ┌────▼────┐  ┌─────▼─────┐
              │ Store     │ │ Daemon  │  │ Exercise  │
              │ (gRPC)    │ │(Operator│  │ (gRPC)    │
              │ orchi-    │ │ K8s CRD)│  │ orchi-    │
              │ store     │ │ orchi-  │  │ system    │
              └───────────┘ │ system  │  └───────────┘
                            └─────────┘
```

---

## Frontend Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orchi-frontend
  namespace: orchi-frontend
  labels:
    app.kubernetes.io/name: orchi-frontend
    app.kubernetes.io/part-of: orchi
    app.kubernetes.io/component: frontend
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: orchi-frontend
  template:
    metadata:
      labels:
        app.kubernetes.io/name: orchi-frontend
    spec:
      containers:
        - name: frontend
          image: ghcr.io/mrtrkmn/orchi-frontend:latest
          ports:
            - containerPort: 80
              name: http
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          securityContext:
            runAsNonRoot: true
            runAsUser: 101  # nginx user
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
          volumeMounts:
            - name: tmp
              mountPath: /tmp
            - name: cache
              mountPath: /var/cache/nginx
            - name: run
              mountPath: /var/run
      volumes:
        - name: tmp
          emptyDir: {}
        - name: cache
          emptyDir: {}
        - name: run
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: orchi-frontend
  namespace: orchi-frontend
spec:
  selector:
    app.kubernetes.io/name: orchi-frontend
  ports:
    - port: 80
      targetPort: http
      name: http
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: orchi-frontend
  namespace: orchi-frontend
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: orchi-frontend
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

---

## API Gateway Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orchi-api-gateway
  namespace: orchi-system
  labels:
    app.kubernetes.io/name: orchi-api-gateway
    app.kubernetes.io/part-of: orchi
    app.kubernetes.io/component: api-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app.kubernetes.io/name: orchi-api-gateway
  template:
    metadata:
      labels:
        app.kubernetes.io/name: orchi-api-gateway
    spec:
      serviceAccountName: orchi-api-gateway
      containers:
        - name: api-gateway
          image: ghcr.io/mrtrkmn/orchi-api:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8081
              name: metrics
          env:
            - name: STORE_ADDR
              value: "orchi-store.orchi-store.svc.cluster.local:5454"
            - name: JWT_PUBLIC_KEY
              valueFrom:
                secretKeyRef:
                  name: orchi-jwt-keys
                  key: public-key.pem
            - name: JWT_PRIVATE_KEY
              valueFrom:
                secretKeyRef:
                  name: orchi-jwt-keys
                  key: private-key.pem
            - name: CORS_ORIGINS
              value: "https://cyberorch.com,https://app.cyberorch.com"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 15
          readinessProbe:
            httpGet:
              path: /readyz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
          securityContext:
            runAsNonRoot: true
            runAsUser: 1000
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
---
apiVersion: v1
kind: Service
metadata:
  name: orchi-api-gateway
  namespace: orchi-system
spec:
  selector:
    app.kubernetes.io/name: orchi-api-gateway
  ports:
    - port: 8080
      targetPort: http
      name: http
    - port: 8081
      targetPort: metrics
      name: metrics
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: orchi-api-gateway
  namespace: orchi-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: orchi-api-gateway
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second
        target:
          type: AverageValue
          averageValue: "100"
```

---

## Ingress Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: orchi-ingress
  namespace: orchi-ingress
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
    cert-manager.io/cluster-issuer: letsencrypt-prod
    traefik.ingress.kubernetes.io/router.middlewares: >-
      orchi-ingress-rate-limit@kubernetescrd,
      orchi-ingress-security-headers@kubernetescrd
spec:
  ingressClassName: traefik
  tls:
    - hosts:
        - cyberorch.com
        - api.cyberorch.com
      secretName: orchi-tls
  rules:
    - host: cyberorch.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: orchi-frontend.orchi-frontend
                port:
                  number: 80
    - host: api.cyberorch.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: orchi-api-gateway.orchi-system
                port:
                  number: 8080
```

---

## Observability Stack

### Prometheus ServiceMonitor
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: orchi-api-gateway
  namespace: orchi-monitoring
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: orchi-api-gateway
  namespaceSelector:
    matchNames:
      - orchi-system
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
```

### Key Metrics
- `orchi_api_requests_total` — Total API requests by endpoint, method, status
- `orchi_api_request_duration_seconds` — Request latency histogram
- `orchi_ws_connections_active` — Active WebSocket connections
- `orchi_flags_submitted_total` — Flag submissions by result
- `orchi_labs_active` — Number of active lab environments
- `orchi_auth_login_total` — Login attempts by result

### Grafana Dashboards
- **API Gateway**: Request rate, latency p50/p95/p99, error rate
- **Scoreboard**: Active connections, update frequency
- **Labs**: Active labs, resource utilization
- **Security**: Failed logins, rate limit triggers, blocked IPs

### Logging with Loki
- Structured JSON logging from all services
- Log levels: DEBUG, INFO, WARN, ERROR
- Request ID propagation for tracing
- Retention: 30 days

---

## Resource Quotas

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: orchi-system-quota
  namespace: orchi-system
spec:
  hard:
    requests.cpu: "4"
    requests.memory: 8Gi
    limits.cpu: "8"
    limits.memory: 16Gi
    pods: "50"
    services: "10"

---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: orchi-lab-quota
  namespace: orchi-lab-template  # Applied to each lab namespace
spec:
  hard:
    requests.cpu: "2"
    requests.memory: 4Gi
    limits.cpu: "4"
    limits.memory: 8Gi
    pods: "10"
```
