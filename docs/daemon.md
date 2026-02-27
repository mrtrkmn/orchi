# Operator

The orchi operator is the core service of the platform. It watches Kubernetes Custom Resources (Events, Labs, Teams, Challenges) and reconciles the cluster state accordingly.

## Configuration

The operator reads its configuration from a Kubernetes ConfigMap and Secret, mounted as environment variables. See [`k8s/base/orchi-operator-deployment.yaml`](../k8s/base/orchi-operator-deployment.yaml) for the full specification.

Key configuration values:

| Environment Variable | Source | Description |
|---|---|---|
| `ORCHI_HOST_HTTP` | ConfigMap | HTTP host for the platform |
| `ORCHI_HOST_GRPC` | ConfigMap | gRPC host for the platform |
| `ORCHI_PRODUCTION_MODE` | ConfigMap | Enable production mode |
| `ORCHI_REGISTRY` | ConfigMap | Container image registry |
| `ORCHI_SIGNING_KEY` | Secret | JWT signing key |
| `ORCHI_RECAPTCHA_KEY` | Secret | reCAPTCHA key |
| `STORE_AUTH_KEY` | Secret | Store authentication key |
| `STORE_SIGN_KEY` | Secret | Store signing key |

## Challenge Configuration

Challenges are defined as Kubernetes Custom Resources (Challenge CRDs). See [`k8s/crds/challenge-crd.yaml`](../k8s/crds/challenge-crd.yaml) for the schema.

Example challenge:
```yaml
apiVersion: orchi.cicibogaz.com/v1alpha1
kind: Challenge
metadata:
  name: sql-injection
  namespace: orchi-lab-example
spec:
  tag: sql-injection
  name: "SQL Injection"
  image: registry.cicibogaz.com/orchi/challenges/sql-injection:1.0.0
  memoryMB: 256
  cpu: 0.5
  flags:
    - tag: sqli-1
      name: "Find the admin password"
      envVar: APP_FLAG
      points: 100
  records:
    - type: A
      name: sqli.lab
```

## CLI Interaction

The legacy CLI (`hkn`) can still interact with the operator for administrative tasks. See [`client/readme.md`](../client/readme.md) for CLI documentation.
