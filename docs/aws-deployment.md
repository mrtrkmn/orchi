# AWS EKS Deployment Guide

This guide walks through deploying the Orchi platform on Amazon EKS (Elastic Kubernetes Service) and configuring GitHub Actions for automated deployment and event management. It covers every step from creating the cluster through to running your first CTF event.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Prerequisites](#prerequisites)
- [Step 1 — Create an EKS Cluster](#step-1--create-an-eks-cluster)
- [Step 2 — Configure IAM for GitHub Actions (OIDC)](#step-2--configure-iam-for-github-actions-oidc)
- [Step 3 — Install Cluster Dependencies](#step-3--install-cluster-dependencies)
- [Step 4 — Create Operator Secrets](#step-4--create-operator-secrets)
- [Step 5 — Configure GitHub Repository Secrets](#step-5--configure-github-repository-secrets)
- [Step 6 — Deploy Orchi via GitHub Actions](#step-6--deploy-orchi-via-github-actions)
- [Step 7 — Verify the Deployment](#step-7--verify-the-deployment)
- [Step 8 — Create an Event via GitHub Actions](#step-8--create-an-event-via-github-actions)
- [Step 9 — DNS and Ingress Configuration](#step-9--dns-and-ingress-configuration)
- [Manual Deployment (without GitHub Actions)](#manual-deployment-without-github-actions)
- [Environment Overlays (Dev / Staging / Prod)](#environment-overlays-dev--staging--prod)
- [Security Hardening](#security-hardening)
- [Monitoring and Observability](#monitoring-and-observability)
- [Backup and Disaster Recovery](#backup-and-disaster-recovery)
- [Upgrades and Rollbacks](#upgrades-and-rollbacks)
- [Cost Optimization](#cost-optimization)
- [Troubleshooting](#troubleshooting)
- [Quick-Reference Cheat Sheet](#quick-reference-cheat-sheet)

---

## Architecture Overview

Before starting, understand how the components fit together on EKS:

```
                         Internet
                            │
                            ▼
              ┌─────────────────────────┐
              │   Ingress (nginx)       │
              │   TLS via cert-manager  │
              │   *.cyberorch.com       │
              └──────┬──────────┬───────┘
                     │          │
          ┌──────────▼──┐  ┌───▼──────────────┐
          │  Frontend   │  │  API Gateway      │
          │  (nginx SPA)│  │  (Go, chi router) │
          │  2 replicas │  │  3 replicas       │
          └─────────────┘  └──────┬────────────┘
                                  │ gRPC
                    ┌─────────────┼─────────────┐
                    │             │              │
              ┌─────▼─────┐ ┌────▼─────┐  ┌─────▼─────┐
              │ Store     │ │ Operator │  │ Amigo     │
              │ StatefulSet│ │ (K8s    │  │ (challenge│
              │ +PVC      │ │ controller│ │ frontend) │
              └───────────┘ │ watches  │  │ 2–10 pods │
                            │ CRDs)    │  └───────────┘
                            └────┬─────┘
                                 │ reconciles
                    ┌────────────┼────────────┐
                    │            │             │
              ┌─────▼─────┐ ┌───▼──────┐ ┌───▼───────┐
              │ Guacamole │ │ WireGuard│ │ VNC Proxy │
              │ (remote   │ │ VPN      │ │ (desktop  │
              │ desktop)  │ │ UDP:51820│ │ access)   │
              └───────────┘ └──────────┘ └───────────┘
```

**Namespaces:**

| Namespace | Purpose |
|---|---|
| `orchi-system` | Core services: Operator, API Gateway, Amigo, Guacamole, WireGuard, VNC Proxy |
| `orchi-store` | Store StatefulSet and backup jobs |
| `orchi-frontend` | Frontend SPA (nginx) |
| `orchi-monitoring` | Prometheus, Grafana, Loki, AlertManager |
| `orchi-ingress` | Ingress controller |
| `orchi-lab-<event-id>` | Per-event lab namespaces (auto-created by operator) |

**Custom Resource Definitions (CRDs):**

| CRD | Scope | Description |
|---|---|---|
| `events.orchi.cyberorch.com` | Cluster | Top-level event (owns labs, teams, challenges) |
| `labs.orchi.cyberorch.com` | Cluster | Lab environments with challenge pods |
| `teams.orchi.cyberorch.com` | Namespaced | Team registrations within an event |
| `challenges.orchi.cyberorch.com` | Namespaced | Individual challenge instances |

**Kubernetes Manifests Layout (`k8s/`):**

```
k8s/
├── kustomization.yaml              # Root kustomization (aggregates everything)
├── base/
│   ├── namespace.yaml              # Lab namespace template
│   ├── orchi-operator-deployment.yaml  # Operator + ConfigMap + Secret
│   ├── orchi-operator-rbac.yaml    # ServiceAccount, ClusterRole, ClusterRoleBinding
│   ├── external-secrets.yaml       # AWS Secrets Manager integration (prod only)
│   ├── feature-flags.yaml          # Gradual feature rollout flags
│   ├── pod-security.yaml           # Pod Security Admission policies
│   ├── poddisruptionbudget.yaml    # PDBs for all workloads
│   └── resource-quotas.yaml        # Namespace resource limits
├── crds/
│   ├── event-crd.yaml
│   ├── lab-crd.yaml
│   ├── team-crd.yaml
│   └── challenge-crd.yaml
├── networking/
│   ├── cert-manager.yaml           # ClusterIssuers + wildcard Certificate
│   ├── ingress.yaml                # Main Ingress (Amigo + Guacamole)
│   ├── ingress-decoupled.yaml      # Decoupled API Gateway ingress
│   ├── lab-dns-configmap.yaml      # CoreDNS overrides for labs
│   ├── networkpolicy-default-deny.yaml
│   ├── networkpolicy-api-frontend.yaml
│   ├── networkpolicy-store-access.yaml
│   ├── networkpolicy-operator-access.yaml
│   ├── networkpolicy-guacamole-access.yaml
│   ├── networkpolicy-intra-lab.yaml
│   ├── networkpolicy-vnc-proxy.yaml
│   └── networkpolicy-vpn-ingress.yaml
├── observability/
│   ├── grafana-dashboard-configmap.yaml
│   ├── prometheus-rules.yaml       # 12 alerting rules
│   └── prometheus-servicemonitor.yaml
├── workloads/
│   ├── amigo-deployment.yaml       # Challenge submission frontend
│   ├── api-gateway-deployment.yaml # REST/gRPC API gateway
│   ├── frontend-deployment.yaml    # SPA frontend (nginx)
│   ├── guacamole-deployment.yaml   # Remote desktop (guacd + web + MySQL)
│   ├── store-statefulset.yaml      # Persistent data store
│   ├── wireguard-deployment.yaml   # VPN (LoadBalancer UDP:51820)
│   ├── vnc-proxy-deployment.yaml   # Desktop access proxy
│   ├── hpa.yaml                    # HorizontalPodAutoscalers
│   ├── backup-cronjob.yaml         # S3 backup every 6h (prod only)
│   ├── velero-schedule.yaml        # Velero backup schedules (prod only)
│   ├── migration-job.yaml          # Data migration job
│   ├── challenge-deployment.yaml   # Challenge pod template
│   ├── challenge-configmap.yaml
│   ├── challenge-secret.yaml
│   └── challenge-service.yaml
└── overlays/
    ├── dev/kustomization.yaml      # Reduced resources, local registry
    ├── staging/kustomization.yaml  # Staging hostnames, moderate scaling
    └── prod/kustomization.yaml     # Full capacity, backups, external secrets
```

---

## Prerequisites

Install and verify every tool **before** proceeding.

### Required Tools

| Tool | Minimum Version | Install | Verify |
|---|---|---|---|
| AWS CLI | v2.x | [Install guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) | `aws --version` |
| eksctl | 0.170+ | [Install guide](https://eksctl.io/installation/) | `eksctl version` |
| kubectl | 1.28+ | [Install guide](https://kubernetes.io/docs/tasks/tools/) | `kubectl version --client` |
| Helm | 3.x | [Install guide](https://helm.sh/docs/intro/install/) | `helm version` |
| Git | 2.x | Pre-installed on most systems | `git --version` |

### macOS Quick Install (Homebrew)

```bash
brew install awscli eksctl kubectl helm git
```

### AWS Account Requirements

Your AWS account (or IAM user) needs permissions to:

- **EKS:** Create/manage clusters and node groups
- **IAM:** Create roles, policies, and OIDC providers
- **EC2:** Create VPCs, subnets, security groups, and EBS volumes
- **S3:** Create buckets (for backups, optional)

Verify your AWS identity:

```bash
aws sts get-caller-identity
```

Expected output:

```json
{
    "UserId": "AIDXXXXXXXXXXXXXXXXX",
    "Account": "123456789012",
    "Arn": "arn:aws:iam::123456789012:user/your-username"
}
```

> **Tip:** If you see an error, run `aws configure` and enter your Access Key, Secret Key, and default region.

### Clone the Repository

```bash
git clone https://github.com/mrtrkmn/orchi.git
cd orchi
```

---

## Step 1 — Create an EKS Cluster

### Option A: Using eksctl (recommended)

This is the fastest path — it creates the VPC, subnets, security groups, NAT gateways, and node group automatically.

**1.1** Create a cluster configuration file `eks-cluster.yaml`:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: orchi-cluster
  region: eu-north-1     # change to your preferred region
  version: "1.31"

iam:
  withOIDC: true          # required for IRSA and GitHub Actions OIDC auth

managedNodeGroups:
  - name: orchi-nodes
    instanceType: t3.large  # 2 vCPU, 8 GiB RAM — see Cost Optimization for alternatives
    desiredCapacity: 3
    minSize: 2
    maxSize: 6
    volumeSize: 50          # GB of EBS per node
    labels:
      role: orchi
    tags:
      project: orchi
    iam:
      withAddonPolicies:
        ebs: true           # required for PVC (store StatefulSet)
        albIngress: true    # required for AWS Load Balancer Controller
        cloudWatch: true    # optional, for CloudWatch logs
```

**1.2** Create the cluster:

```bash
eksctl create cluster -f eks-cluster.yaml
```

> **Wait time:** 15–20 minutes. The command blocks until the cluster is ready.
> You can watch progress in the CloudFormation console: [CloudFormation Stacks](https://console.aws.amazon.com/cloudformation/).

**1.3** Verify the cluster was created:

```bash
# Check cluster status
aws eks describe-cluster --name orchi-cluster --region eu-north-1 --query 'cluster.status'
# Expected: "ACTIVE"

# Check nodes are ready
kubectl get nodes -o wide
# Expected: 3 nodes in Ready state
```

### Option B: Using AWS CLI

Use this method if you already have a VPC and subnets, or need more control.

**1.1** Create the IAM roles (if they don't exist):

```bash
# Create the EKS cluster service role
aws iam create-role \
  --role-name eks-cluster-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {"Service": "eks.amazonaws.com"},
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy \
  --role-name eks-cluster-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy

# Create the node group role
aws iam create-role \
  --role-name eks-node-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {"Service": "ec2.amazonaws.com"},
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy --role-name eks-node-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
aws iam attach-role-policy --role-name eks-node-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
aws iam attach-role-policy --role-name eks-node-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
aws iam attach-role-policy --role-name eks-node-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy
```

**1.2** Create the cluster:

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

aws eks create-cluster \
  --name orchi-cluster \
  --region eu-north-1 \
  --kubernetes-version 1.31 \
  --role-arn arn:aws:iam::${ACCOUNT_ID}:role/eks-cluster-role \
  --resources-vpc-config subnetIds=subnet-xxx,subnet-yyy,securityGroupIds=sg-zzz
```

> **Replace** `subnet-xxx`, `subnet-yyy`, `sg-zzz` with your actual VPC subnet and security group IDs. You need at least 2 subnets in different availability zones.

**1.3** Wait for the cluster to become active:

```bash
echo "Waiting for cluster to be ACTIVE (this can take 10-15 minutes)..."
aws eks wait cluster-active --name orchi-cluster --region eu-north-1
echo "Cluster is ACTIVE!"
```

**1.4** Create the node group:

```bash
aws eks create-nodegroup \
  --cluster-name orchi-cluster \
  --nodegroup-name orchi-nodes \
  --node-role arn:aws:iam::${ACCOUNT_ID}:role/eks-node-role \
  --instance-types t3.large \
  --scaling-config minSize=2,maxSize=6,desiredSize=3 \
  --disk-size 50 \
  --region eu-north-1

echo "Waiting for node group to be ACTIVE..."
aws eks wait nodegroup-active \
  --cluster-name orchi-cluster \
  --nodegroup-name orchi-nodes \
  --region eu-north-1
echo "Node group is ACTIVE!"
```

### Configure kubectl

Regardless of which method you used, configure kubectl to talk to your cluster:

```bash
# Update kubeconfig
aws eks update-kubeconfig --name orchi-cluster --region eu-north-1

# Verify connectivity
kubectl get nodes
```

Expected output:

```
NAME                                        STATUS   ROLES    AGE   VERSION
ip-10-0-1-100.eu-north-1.compute.internal   Ready    <none>   5m    v1.31.x
ip-10-0-2-200.eu-north-1.compute.internal   Ready    <none>   5m    v1.31.x
ip-10-0-3-300.eu-north-1.compute.internal   Ready    <none>   5m    v1.31.x
```

> **Checkpoint:** You should now have a running EKS cluster with 3 healthy nodes. If `kubectl get nodes` shows no nodes or errors, see [Troubleshooting](#kubectl-cannot-access-the-cluster).

---

## Step 2 — Configure IAM for GitHub Actions (OIDC)

GitHub Actions authenticates to AWS using OIDC (OpenID Connect), eliminating the need for long-lived AWS access keys. This is the recommended approach for CI/CD pipelines.

**How it works:**
1. GitHub Actions requests a short-lived OIDC token from GitHub's identity provider.
2. The workflow exchanges this token with AWS STS for temporary credentials.
3. The temporary credentials grant access to the EKS cluster.
4. Credentials expire automatically after the workflow completes.

### 2.1 Create the GitHub OIDC Identity Provider

If you used `eksctl` with `withOIDC: true`, the **EKS** OIDC provider already exists. For **GitHub Actions**, you need an additional OIDC identity provider:

```bash
# Check if it already exists
aws iam list-open-id-connect-providers | grep token.actions.githubusercontent.com

# If not, create it
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

> **Note:** As of 2024, AWS validates the GitHub Actions certificate chain automatically. The thumbprint is accepted for API compatibility but is not used for verification.

Verify it was created:

```bash
aws iam list-open-id-connect-providers
# Look for: arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com
```

### 2.2 Create an IAM Role for GitHub Actions

**2.2.1** Create a trust policy file `github-actions-trust-policy.json`:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:mrtrkmn/orchi:*"
        }
      }
    }
  ]
}
```

> **Security:** The `sub` condition restricts this role to workflows running in the `mrtrkmn/orchi` repository only. Change `mrtrkmn/orchi` to your `<org>/<repo>`.
> To restrict further, use `repo:mrtrkmn/orchi:ref:refs/heads/master` (only master branch) or `repo:mrtrkmn/orchi:environment:prod` (only prod environment).

**2.2.2** Replace `ACCOUNT_ID` with your actual AWS account ID:

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
sed -i '' "s/ACCOUNT_ID/${ACCOUNT_ID}/g" github-actions-trust-policy.json
# On Linux (without macOS sed): sed -i "s/ACCOUNT_ID/${ACCOUNT_ID}/g" github-actions-trust-policy.json
```

**2.2.3** Create the IAM role:

```bash
aws iam create-role \
  --role-name orchi-github-actions \
  --assume-role-policy-document file://github-actions-trust-policy.json \
  --description "IAM role for Orchi GitHub Actions CI/CD pipeline"
```

**2.2.4** Attach the required policies:

```bash
# EKS cluster policy (required for eks:DescribeCluster)
aws iam attach-role-policy \
  --role-name orchi-github-actions \
  --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy

# Create a minimal custom policy for kubectl access
cat > eks-kubectl-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EKSAccess",
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListClusters"
      ],
      "Resource": "*"
    }
  ]
}
EOF

aws iam create-policy \
  --policy-name orchi-eks-kubectl \
  --policy-document file://eks-kubectl-policy.json

aws iam attach-role-policy \
  --role-name orchi-github-actions \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/orchi-eks-kubectl
```

**2.2.5** Verify the role:

```bash
aws iam get-role --role-name orchi-github-actions --query 'Role.Arn' --output text
# Expected: arn:aws:iam::<ACCOUNT_ID>:role/orchi-github-actions
```

### 2.3 Grant the IAM Role Access to the Cluster

The IAM role needs Kubernetes RBAC permissions. EKS supports two methods:

#### Option A: EKS Access Entries (recommended, EKS 1.30+)

EKS Access Entries are the modern, API-native way to manage cluster access without
editing the `aws-auth` ConfigMap directly. Since this guide uses EKS 1.31, Access Entries are fully supported:

**Step 1** — Create an access entry for the GitHub Actions role:

```bash
aws eks create-access-entry \
  --cluster-name orchi-cluster \
  --principal-arn arn:aws:iam::${ACCOUNT_ID}:role/orchi-github-actions \
  --type STANDARD \
  --region eu-north-1
```

**Step 2** — Associate an access policy (ClusterAdmin for deployments):

```bash
aws eks associate-access-policy \
  --cluster-name orchi-cluster \
  --principal-arn arn:aws:iam::${ACCOUNT_ID}:role/orchi-github-actions \
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy \
  --access-scope type=cluster \
  --region eu-north-1
```

**Step 3** — Verify the access entry:

```bash
aws eks list-access-entries --cluster-name orchi-cluster --region eu-north-1
aws eks describe-access-entry \
  --cluster-name orchi-cluster \
  --principal-arn arn:aws:iam::${ACCOUNT_ID}:role/orchi-github-actions \
  --region eu-north-1
```

> **Production tip:** Use `AmazonEKSAdminPolicy` (namespace-scoped) instead of `AmazonEKSClusterAdminPolicy` and limit access to `orchi-system`, `orchi-store`, and `orchi-frontend` namespaces only.

#### Option B: aws-auth ConfigMap (legacy, all EKS versions)

Add an entry to the `aws-auth` ConfigMap:

```bash
# Check current auth map
kubectl -n kube-system get configmap aws-auth -o yaml

# Add the GitHub Actions role
eksctl create iamidentitymapping \
  --cluster orchi-cluster \
  --region eu-north-1 \
  --arn arn:aws:iam::${ACCOUNT_ID}:role/orchi-github-actions \
  --group system:masters \
  --username github-actions

# Verify the mapping
eksctl get iamidentitymapping --cluster orchi-cluster --region eu-north-1
```

> **Production tip:** Use a more restrictive Kubernetes RBAC role instead of `system:masters`. See the [Kubernetes RBAC docs](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

> **Checkpoint:** IAM is now configured. GitHub Actions workflows will be able to authenticate and deploy to your cluster.

---

## Step 3 — Install Cluster Dependencies

Orchi requires several cluster-level components before you can deploy the platform. Install them in order.

### 3.1 Install a CNI with NetworkPolicy Support

EKS ships with the Amazon VPC CNI, which does **not** support NetworkPolicy enforcement. Orchi uses 7 different NetworkPolicies (default-deny, API/frontend access, store access, operator access, guacamole access, intra-lab isolation, VPN ingress), so a CNI that enforces policies is **mandatory**.

#### Option A: Calico (simpler setup)

```bash
# Install Calico for NetworkPolicy enforcement
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.29.0/manifests/calico-vxlan.yaml

# Verify Calico pods are running
kubectl -n kube-system get pods -l k8s-app=calico-node
```

Wait until all Calico pods show `Running` (1–2 minutes):

```
NAME                READY   STATUS    RESTARTS   AGE
calico-node-xxxxx   1/1     Running   0          60s
calico-node-yyyyy   1/1     Running   0          60s
calico-node-zzzzz   1/1     Running   0          60s
```

#### Option B: Cilium (advanced, with eBPF observability)

```bash
helm repo add cilium https://helm.cilium.io/
helm repo update

helm install cilium cilium/cilium --version 1.16.5 \
  --namespace kube-system \
  --set eni.enabled=true \
  --set ipam.mode=eni \
  --set egressMasqueradeInterfaces=eth0 \
  --set routingMode=native

# Verify Cilium pods are running
kubectl -n kube-system get pods -l k8s-app=cilium
```

### 3.2 Install the EBS CSI Driver (for PVCs)

The `orchi-store` StatefulSet requires EBS-backed persistent volumes. The dev overlay uses 1 GiB, production uses 20 GiB.

**3.2.1** Install the EBS CSI driver as an EKS Add-on:

```bash
eksctl create addon \
  --cluster orchi-cluster \
  --name aws-ebs-csi-driver \
  --region eu-north-1 \
  --force
```

**3.2.2** Verify the driver is running:

```bash
kubectl -n kube-system get pods -l app.kubernetes.io/name=aws-ebs-csi-driver
```

Expected: 2+ pods in `Running` state (controller + node daemonset).

**3.2.3** Create a `gp3` StorageClass and set it as default:

```bash
kubectl apply -f - <<'EOF'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gp3
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  fsType: ext4
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
reclaimPolicy: Retain
EOF
```

**3.2.4** Verify the StorageClass:

```bash
kubectl get storageclass
```

Expected:

```
NAME            PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
gp2             kubernetes.io/aws-ebs   Delete    WaitForFirstConsumer   false                  20m
gp3 (default)   ebs.csi.aws.com        Retain    WaitForFirstConsumer   true                   5s
```

> **Why `gp3`?** It provides 3,000 IOPS baseline for free (gp2 only scales IOPS with volume size), and is ~20% cheaper at the same performance.

### 3.3 Install an Ingress Controller

Orchi's Ingress resources use `ingressClassName: nginx`. Choose one option:

#### Option A: NGINX Ingress Controller (default)

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/aws-load-balancer-type"=nlb \
  --set controller.service.annotations."service\.beta\.kubernetes\.io/aws-load-balancer-scheme"=internet-facing

# Wait for the LoadBalancer to get an external address
echo "Waiting for LoadBalancer external hostname..."
kubectl -n ingress-nginx get svc ingress-nginx-controller -w
```

Note the `EXTERNAL-IP` (an AWS NLB hostname like `a1b2c3d4e5f6g7.elb.eu-north-1.amazonaws.com`). You'll need it for DNS in [Step 9](#step-9--dns-and-ingress-configuration).

#### Option B: AWS Load Balancer Controller (ALB-based)

```bash
helm repo add eks https://aws.github.io/eks-charts
helm repo update

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=orchi-cluster \
  --set serviceAccount.create=true
```

> **Note:** If using ALB, you need to change the Ingress annotations from `nginx` to `alb` in the manifests.

### 3.4 Install cert-manager (for TLS)

cert-manager automates TLS certificate issuance and renewal via Let's Encrypt.

**3.4.1** Install cert-manager:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.16.3 \
  --set crds.enabled=true
```

**3.4.2** Verify cert-manager is running:

```bash
kubectl -n cert-manager get pods
```

Expected: 3 pods in `Running` state (cert-manager, cainjector, webhook).

```
NAME                                      READY   STATUS    RESTARTS   AGE
cert-manager-xxxxxxxxx-xxxxx             1/1     Running   0          60s
cert-manager-cainjector-xxxxxxx-xxxxx    1/1     Running   0          60s
cert-manager-webhook-xxxxxxx-xxxxx       1/1     Running   0          60s
```

**3.4.3** Verify cert-manager CRDs are installed:

```bash
kubectl get crds | grep cert-manager
# Expected: 6 CRDs (certificaterequests, certificates, challenges, clusterissuers, issuers, orders)
```

### 3.5 Configure Cloudflare DNS for Let's Encrypt Wildcard Certificates

Orchi uses a wildcard TLS certificate (`*.cyberorch.com`) issued by Let's Encrypt.
Wildcard certificates require DNS-01 challenge validation, which is handled by
cert-manager using the Cloudflare API.

The certificate covers:
- `cyberorch.com` — main landing page
- `*.cyberorch.com` — API, staging, desktop, dynamic event subdomains

#### 3.5.1 Create a Cloudflare API Token

1. Go to [Cloudflare Dashboard → Profile → API Tokens](https://dash.cloudflare.com/profile/api-tokens)
2. Click **Create Token**
3. Select the **Edit zone DNS** template
4. Configure permissions exactly as follows:
   - **Permissions:**
     - Zone → DNS → **Edit**
     - Zone → Zone → **Read**
   - **Zone Resources:**
     - Include → **Specific zone** → `cyberorch.com`
5. (Optional) Set an IP filtering rule and expiration date for extra security
6. Click **Continue to summary** → **Create Token**
7. **Copy the token value immediately** — it is shown only once

> **Security:** Use a scoped API Token, not a Global API Key. The token only needs DNS edit permissions for one zone.

#### 3.5.2 Create the Cloudflare Secret in Kubernetes

```bash
kubectl create secret generic cloudflare-api-token \
  --namespace cert-manager \
  --from-literal=api-token=<YOUR_CLOUDFLARE_API_TOKEN>
```

Verify:

```bash
kubectl -n cert-manager get secret cloudflare-api-token
# Should show: cloudflare-api-token   Opaque   1   <age>
```

> **Warning:** The template Secret in `k8s/networking/cert-manager.yaml` contains a placeholder value (`REPLACE_WITH_CLOUDFLARE_API_TOKEN`). Always create the secret manually via `kubectl` or use External Secrets Operator in production — **never** commit real tokens to version control.

#### 3.5.3 Deploy the ClusterIssuers and Certificate

The ClusterIssuers (Let's Encrypt prod + staging) and the wildcard Certificate are deployed automatically when you apply the Kustomize manifests in [Step 6](#step-6--deploy-orchi-via-github-actions). After deploying, verify:

```bash
# Check that both ClusterIssuers are Ready
kubectl get clusterissuers
# Expected:
# NAME                  READY   AGE
# letsencrypt-prod      True    60s
# letsencrypt-staging   True    60s

# Check the wildcard certificate status
kubectl -n orchi-system get certificates
kubectl -n orchi-system describe certificate cyberorch-wildcard-cert

# Check the TLS secret was created (may take 1-3 minutes for initial issuance)
kubectl -n orchi-system get secret cyberorch-tls-cert
```

**Testing with the staging issuer first (recommended):**

Let's Encrypt has [rate limits](https://letsencrypt.org/docs/rate-limits/) — 5 duplicate certificates per week. Use the staging issuer first to verify everything works:

```bash
# Switch to staging issuer
kubectl -n orchi-system patch certificate cyberorch-wildcard-cert \
  --type merge -p '{"spec":{"issuerRef":{"name":"letsencrypt-staging"}}}'

# Wait for the staging cert, then switch back to prod
kubectl -n orchi-system patch certificate cyberorch-wildcard-cert \
  --type merge -p '{"spec":{"issuerRef":{"name":"letsencrypt-prod"}}}'
```

> **Checkpoint:** All cluster dependencies are installed. You have: CNI with NetworkPolicy support, EBS CSI driver, Ingress controller, cert-manager with Cloudflare DNS-01.

---

## Step 4 — Create Operator Secrets

The Orchi operator requires secrets for signing keys, API credentials, and reCAPTCHA keys. These **must** be created before deploying.

### 4.1 Create the Operator Secret

```bash
kubectl create namespace orchi-system 2>/dev/null || true

kubectl create secret generic orchi-operator-secrets \
  --namespace orchi-system \
  --from-literal=ORCHI_SIGNING_KEY="$(openssl rand -hex 32)" \
  --from-literal=ORCHI_RECAPTCHA_KEY="your-recaptcha-site-key" \
  --from-literal=ORCHI_API_USERNAME="admin" \
  --from-literal=ORCHI_API_PASSWORD="$(openssl rand -base64 24)"
```

> **Important:** The Secret template in `k8s/base/orchi-operator-deployment.yaml` contains placeholder values (`REPLACE_BEFORE_DEPLOY`). The manually created secret above takes precedence because the template has `optional: true`.

### 4.2 Verify the Secret

```bash
kubectl -n orchi-system get secret orchi-operator-secrets
# Verify keys exist (values are hidden):
kubectl -n orchi-system get secret orchi-operator-secrets -o jsonpath='{.data}' | python3 -c "import sys,json; print('\n'.join(json.loads(sys.stdin.read()).keys()))"
```

### 4.3 (Production) Use External Secrets Operator

For production, never store secrets directly in the cluster. Use AWS Secrets Manager via External Secrets Operator:

```bash
# Install External Secrets Operator
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets \
  --namespace external-secrets \
  --create-namespace

# The production overlay (k8s/overlays/prod) includes ExternalSecret resources
# that sync from AWS Secrets Manager automatically.
```

Store your secrets in AWS Secrets Manager:

```bash
aws secretsmanager create-secret \
  --name orchi/operator-secrets \
  --region eu-north-1 \
  --secret-string '{
    "ORCHI_SIGNING_KEY": "'$(openssl rand -hex 32)'",
    "ORCHI_RECAPTCHA_KEY": "your-recaptcha-key",
    "ORCHI_API_USERNAME": "admin",
    "ORCHI_API_PASSWORD": "'$(openssl rand -base64 24)'"
  }'
```

---

## Step 5 — Configure GitHub Repository Secrets

The GitHub Actions workflows (`deploy-prod.yml`, `create-event.yml`, `stop-event.yml`) read credentials from repository or environment secrets.

### 5.1 Add Repository-Level Secrets

Go to your GitHub repository → **Settings** → **Secrets and variables** → **Actions** → **New repository secret**:

| Secret Name | Value | How to Obtain |
|---|---|---|
| `AWS_ROLE_ARN` | `arn:aws:iam::<ACCOUNT_ID>:role/orchi-github-actions` | From Step 2: `aws iam get-role --role-name orchi-github-actions --query 'Role.Arn' --output text` |
| `AWS_REGION` | `eu-north-1` | The region where your EKS cluster runs |
| `EKS_CLUSTER_NAME` | `orchi-cluster` | The name you chose in Step 1 |

### 5.2 (Optional) Add Environment-Level Secrets

For multi-environment setups (dev/staging/prod on different clusters), create GitHub Environments:

1. Go to **Settings** → **Environments** → **New environment**
2. Create three environments: `dev`, `staging`, `prod`
3. Add per-environment secrets (they override repository secrets):

| Environment | `AWS_REGION` | `EKS_CLUSTER_NAME` | `AWS_ROLE_ARN` |
|---|---|---|---|
| `dev` | `eu-north-1` | `orchi-dev` | `arn:aws:iam::<ID>:role/orchi-github-actions-dev` |
| `staging` | `eu-north-1` | `orchi-staging` | `arn:aws:iam::<ID>:role/orchi-github-actions-staging` |
| `prod` | `eu-north-1` | `orchi-cluster` | `arn:aws:iam::<ID>:role/orchi-github-actions` |

### 5.3 (Optional) Add Protection Rules for Production

For the `prod` environment, add protection rules:

1. Go to **Settings** → **Environments** → `prod`
2. Enable **Required reviewers** — add at least 1 team member
3. Enable **Wait timer** — set to 5 minutes (grace period to cancel)
4. Under **Deployment branches**, select **Selected branches** → add `master` and `v*.*.*` tags

### 5.4 Verify Secrets Are Configured

The deployment workflow checks for these secrets at the "Configure AWS credentials" step. If any are missing, the workflow will fail with a clear error message.

> **Checkpoint:** GitHub Actions is configured to authenticate to your EKS cluster via OIDC. No long-lived credentials are stored.

---

## Step 6 — Deploy Orchi via GitHub Actions

### 6.1 First Deployment (Dry Run)

1. Go to your GitHub repository → **Actions** tab
2. In the left sidebar, click **Deploy Orchi Platform**
3. Click **Run workflow** (top right)
4. Fill in the parameters:
   - **Target environment:** `dev` (start with dev for first deployment)
   - **Cluster provider:** `aws`
   - **Dry run (diff only, no apply):** ✅ `true`
5. Click **Run workflow**

The dry run renders all Kustomize manifests and runs `kubectl diff` to show exactly what would be created/changed — without actually applying anything. Review the diff in the workflow logs.

### 6.2 Apply the Deployment

Once you've reviewed the dry-run output:

1. Click **Run workflow** again
2. Same parameters, but set **Dry run:** `false`
3. Click **Run workflow**

The workflow executes these steps in order:

```
1. Checkout                   → clones the repository
2. Set environment            → determines target environment
3. Configure kubectl          → installs kubectl
4. Configure AWS credentials  → exchanges OIDC token for AWS credentials
5. Set up kubeconfig          → runs aws eks update-kubeconfig
6. Validate manifests         → runs kubectl kustomize (syntax check)
7. Deploy                     → runs kubectl apply -k k8s/overlays/<env>
8. Wait for rollout           → waits for operator and amigo to be ready
9. Verify deployment          → prints pods, services, and CRDs
```

### 6.3 What Gets Deployed

The Kustomize manifests deploy all these resources:

| Resource | Type | Namespace | Replicas |
|---|---|---|---|
| Orchi Operator | Deployment | `orchi-system` | 1 (leader election) |
| Amigo | Deployment + HPA | `orchi-system` | 2–10 |
| API Gateway | Deployment + HPA | `orchi-system` | 3–20 |
| Frontend | Deployment + HPA | `orchi-frontend` | 2–10 |
| Guacamole | Deployment + HPA | `orchi-system` | 1–5 |
| VNC Proxy | Deployment + HPA | `orchi-system` | 2–20 |
| WireGuard | Deployment | `orchi-system` | 1 (LoadBalancer) |
| Store | StatefulSet | `orchi-store` | 1 (with PVC) |
| 4 CRDs | CRD | cluster-scoped | — |
| 7 NetworkPolicies | NetworkPolicy | `orchi-system` | — |
| Ingress | Ingress | `orchi-system` | — |
| cert-manager resources | ClusterIssuer, Certificate | various | — |
| PodDisruptionBudgets | PDB | `orchi-system` | — |
| ResourceQuotas | ResourceQuota | lab namespaces | — |

### 6.4 Automatic Deployment on Version Tags

Pushing a Git tag matching `v*.*.*` or `*.*.*` triggers an automatic production deployment:

```bash
# Tag and push
git tag v2.1.0
git push origin v2.1.0
```

The workflow automatically sets `environment=prod` and `cluster_provider=aws` for tag-triggered runs.

> **Checkpoint:** Orchi is deployed to your cluster. Proceed to verification.

---

## Step 7 — Verify the Deployment

Run these commands locally to verify everything is healthy.

### 7.1 Configure Local kubectl

```bash
aws eks update-kubeconfig --name orchi-cluster --region eu-north-1
```

### 7.2 Check CRDs

```bash
kubectl get crds | grep orchi
```

Expected:

```
challenges.orchi.cyberorch.com     2025-01-01T00:00:00Z
events.orchi.cyberorch.com         2025-01-01T00:00:00Z
labs.orchi.cyberorch.com           2025-01-01T00:00:00Z
teams.orchi.cyberorch.com          2025-01-01T00:00:00Z
```

### 7.3 Check Pods

```bash
kubectl -n orchi-system get pods
```

All pods should show `Running` with `READY` matching the expected count:

```
NAME                              READY   STATUS    RESTARTS   AGE
orchi-operator-xxxxxxxxx-xxxxx    1/1     Running   0          5m
amigo-xxxxxxxxx-xxxxx             1/1     Running   0          5m
amigo-xxxxxxxxx-yyyyy             1/1     Running   0          5m
guacamole-xxxxxxxxx-xxxxx         3/3     Running   0          5m
wireguard-xxxxxxxxx-xxxxx         1/1     Running   0          5m
vnc-proxy-xxxxxxxxx-xxxxx         1/1     Running   0          5m
vnc-proxy-xxxxxxxxx-yyyyy         1/1     Running   0          5m
```

### 7.4 Check Services

```bash
kubectl -n orchi-system get svc
```

The WireGuard service should have an external IP (AWS NLB address):

```
NAME         TYPE           CLUSTER-IP      EXTERNAL-IP                                     PORT(S)
amigo        ClusterIP      10.100.x.x      <none>                                          80/TCP
guacamole    ClusterIP      10.100.x.x      <none>                                          8080/TCP
wireguard    LoadBalancer   10.100.x.x      a1b2c3.elb.eu-north-1.amazonaws.com            51820:xxxxx/UDP
```

### 7.5 Check Ingress

```bash
kubectl -n orchi-system get ingress
```

The Ingress should show the configured host and the ingress controller's address:

```
NAME            CLASS   HOSTS           ADDRESS                                     PORTS     AGE
orchi-ingress   nginx   cyberorch.com   a1b2c3.elb.eu-north-1.amazonaws.com        80, 443   5m
```

### 7.6 Check TLS Certificate

```bash
kubectl -n orchi-system get certificate
```

```
NAME                       READY   SECRET                AGE
cyberorch-wildcard-cert    True    cyberorch-tls-cert    5m
```

If `READY` is `False`, check the certificate events:

```bash
kubectl -n orchi-system describe certificate cyberorch-wildcard-cert
```

### 7.7 Check Operator Health

```bash
# Readiness and liveness probes
kubectl -n orchi-system exec deploy/orchi-operator -- wget -qO- http://localhost:8081/healthz
kubectl -n orchi-system exec deploy/orchi-operator -- wget -qO- http://localhost:8081/readyz

# Operator logs
kubectl -n orchi-system logs deploy/orchi-operator --tail=50
```

### 7.8 Full Status Summary

```bash
echo "=== Nodes ==="
kubectl get nodes -o wide
echo ""
echo "=== CRDs ==="
kubectl get crds | grep orchi
echo ""
echo "=== Operator Pod ==="
kubectl -n orchi-system get pods -l app.kubernetes.io/name=orchi-operator
echo ""
echo "=== All Pods ==="
kubectl -n orchi-system get pods
echo ""
echo "=== Services ==="
kubectl -n orchi-system get svc
echo ""
echo "=== Ingress ==="
kubectl -n orchi-system get ingress
echo ""
echo "=== Certificates ==="
kubectl -n orchi-system get certificates
echo ""
echo "=== HPAs ==="
kubectl -n orchi-system get hpa
echo ""
echo "=== PDBs ==="
kubectl -n orchi-system get pdb
echo ""
echo "=== PVCs ==="
kubectl get pvc -A
echo ""
echo "=== Events (last 10) ==="
kubectl -n orchi-system get events --sort-by=.lastTimestamp | tail -10
```

> **Checkpoint:** If all pods are running, CRDs exist, and the Ingress has an address, the deployment is successful.

---

## Step 8 — Create an Event via GitHub Actions

An **Event** is the top-level resource in Orchi. It creates a lab namespace, deploys challenge pods, configures VPN access, and manages team registrations.

### 8.1 Create an Event

1. Go to **Actions** → **Create Event** → **Run workflow**
2. Fill in the parameters:

| Parameter | Example Value | Description |
|---|---|---|
| **Event tag** | `ctf-2025` | Unique identifier — lowercase, alphanumeric, hyphens allowed, 2–63 chars |
| **Event display name** | `CTF Competition 2025` | Human-readable name shown in the UI |
| **Maximum number of teams** | `50` | Capacity limit for team registration |
| **Exercise tags** | `sql-injection,xss-basic,buffer-overflow` | Comma-separated list of exercise tags to include |
| **Frontend VM image** | `ghcr.io/mrtrkmn/orchi/frontends/kali:latest` | *(Optional)* Guacamole RDP target image |
| **Frontend VM memory** | `4096` | *(Optional)* Memory in MB for frontend VMs |
| **Frontend VM CPU** | `2` | *(Optional)* CPU cores for frontend VMs |
| **Target environment** | `prod` | Must match the environment where Orchi is deployed |
| **Cluster provider** | `aws` | Use `aws` for EKS |

3. Click **Run workflow**

### 8.2 What Happens Behind the Scenes

The workflow:

1. **Validates inputs** — checks that the event tag is valid (lowercase, 2–63 chars, alphanumeric with optional hyphens)
2. **Generates an Event CR manifest** — creates a YAML file with the Event custom resource
3. **Applies the Event CR** — `kubectl apply -f /tmp/event.yaml`
4. **Waits for the event to be ready** — polls the event status every 10 seconds for up to 5 minutes
5. **Prints event details** — shows the event, labs, and teams

The **operator** then reconciles the Event CR into:

```
Event CR (ctf-2025)
├── Namespace: orchi-lab-ctf-2025
│   ├── Challenge pods (sql-injection, xss-basic, buffer-overflow)
│   ├── Challenge services (per-exercise ClusterIP)
│   ├── NetworkPolicies (inter-challenge isolation)
│   └── ResourceQuota (limits total CPU/memory)
├── Lab CR (references the event)
├── DNS records (via CoreDNS ConfigMap overrides)
└── VPN config (WireGuard peer entries)
```

### 8.3 Verify the Event

```bash
# Check the event
kubectl get events.orchi.cyberorch.com
# Expected:
# NAME        NAME                    TAG        CAPACITY   AVAILABLE   PHASE     AGE
# ctf-2025    CTF Competition 2025    ctf-2025   50         50          Running   2m

# Check the lab
kubectl get labs.orchi.cyberorch.com
# Expected: lab associated with ctf-2025

# Check the lab namespace
kubectl get ns orchi-lab-ctf-2025
# Expected: Active namespace

# Check challenge pods in the lab namespace
kubectl -n orchi-lab-ctf-2025 get pods
# Expected: one pod per exercise

# Check teams (empty until teams register)
kubectl get teams.orchi.cyberorch.com -l orchi.cyberorch.com/event=ctf-2025
```

### 8.4 Inspect Event Details

```bash
# Full event YAML
kubectl get events.orchi.cyberorch.com/ctf-2025 -o yaml

# Event conditions (shows reconciliation progress)
kubectl get events.orchi.cyberorch.com/ctf-2025 -o jsonpath='{.status.conditions}' | python3 -m json.tool
```

### 8.5 Stop / Delete an Event

**Via GitHub Actions:**

1. Go to **Actions** → **Stop Event** → **Run workflow**
2. Enter:
   - **Event tag:** `ctf-2025`
   - **Target environment:** `prod`
   - **Cluster provider:** `aws`
3. Click **Run workflow**

**Via kubectl (manual):**

```bash
kubectl delete events.orchi.cyberorch.com/ctf-2025
```

The operator will automatically:
- Delete the lab namespace (`orchi-lab-ctf-2025`) and all its contents
- Clean up VPN peer configurations
- Remove DNS records
- Update the event phase to `Closed`

Verify cleanup:

```bash
kubectl get ns orchi-lab-ctf-2025
# Expected: NotFound

kubectl get events.orchi.cyberorch.com
# Expected: ctf-2025 is no longer listed
```

---

## Step 9 — DNS and Ingress Configuration

### 9.1 Get the Ingress External Address

```bash
# For NGINX Ingress Controller
kubectl -n ingress-nginx get svc ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'

# Or check the Ingress resource directly
kubectl -n orchi-system get ingress orchi-ingress -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'
```

### 9.2 Configure DNS Records in Cloudflare

1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com/) → select `cyberorch.com`
2. Go to **DNS** → **Records**
3. Add the following records:

| Type | Name | Content | Proxy |
|---|---|---|---|
| CNAME | `@` (root) | `<NLB hostname from above>` | Proxied (orange cloud) |
| CNAME | `*` (wildcard) | `<NLB hostname from above>` | DNS only (grey cloud) |
| CNAME | `staging` | `<NLB hostname from above>` | Proxied |
| CNAME | `api` | `<NLB hostname from above>` | Proxied |

> **Note:** Wildcard (`*`) subdomains should use **DNS only** (grey cloud) because Cloudflare's free plan doesn't proxy wildcard records. The specific subdomains (root, staging, api) can use Cloudflare proxying for DDoS protection.

### 9.3 Verify DNS Resolution

```bash
# Check DNS resolves
dig cyberorch.com +short
dig staging.cyberorch.com +short
dig api.cyberorch.com +short

# Check HTTPS works (may take a few minutes for cert issuance)
curl -I https://cyberorch.com
# Expected: HTTP/2 200 (or 301/302 depending on auth)
```

### 9.4 WireGuard VPN DNS

The WireGuard service gets its own LoadBalancer IP. Get it and add a DNS record:

```bash
kubectl -n orchi-system get svc wireguard -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'
```

Add a DNS record in Cloudflare:

| Type | Name | Content | Proxy |
|---|---|---|---|
| CNAME | `vpn` | `<WireGuard NLB hostname>` | DNS only (grey cloud) |

> **Important:** VPN (UDP:51820) must use **DNS only** — Cloudflare proxy does not support UDP.

---

## Manual Deployment (without GitHub Actions)

If you prefer to deploy manually or don't use GitHub Actions:

### Full Manual Deployment

```bash
# 1. Configure kubectl
aws eks update-kubeconfig --name orchi-cluster --region eu-north-1

# 2. Create the operator secret (if not already created)
kubectl create namespace orchi-system 2>/dev/null || true
kubectl create secret generic orchi-operator-secrets \
  --namespace orchi-system \
  --from-literal=ORCHI_SIGNING_KEY="$(openssl rand -hex 32)" \
  --from-literal=ORCHI_RECAPTCHA_KEY="your-recaptcha-key" \
  --from-literal=ORCHI_API_USERNAME="admin" \
  --from-literal=ORCHI_API_PASSWORD="$(openssl rand -base64 24)" \
  --dry-run=client -o yaml | kubectl apply -f -

# 3. Preview what will be deployed
kubectl kustomize k8s/overlays/dev

# 4. Deploy (choose your environment)
kubectl apply -k k8s/overlays/dev       # development
kubectl apply -k k8s/overlays/staging   # staging
kubectl apply -k k8s/overlays/prod      # production

# 5. Wait for rollout
echo "Waiting for operator..."
kubectl -n orchi-system rollout status deployment/orchi-operator --timeout=120s
echo "Waiting for amigo..."
kubectl -n orchi-system rollout status deployment/amigo --timeout=120s
echo "Deployment complete!"

# 6. Verify
kubectl -n orchi-system get pods
kubectl -n orchi-system get svc
kubectl get crds | grep orchi
```

### Manual Event Creation

```bash
kubectl apply -f - <<'EOF'
apiVersion: orchi.cyberorch.com/v1alpha1
kind: Event
metadata:
  name: ctf-2025
  labels:
    app.kubernetes.io/managed-by: manual
    app.kubernetes.io/part-of: orchi
spec:
  tag: ctf-2025
  name: "CTF Competition 2025"
  capacity: 50
  createdBy: admin
  lab:
    exercises:
      - sql-injection
      - xss-basic
      - buffer-overflow
  vpn:
    required: false
EOF

# Wait for the event to reach Running phase
echo "Waiting for event..."
for i in $(seq 1 30); do
  PHASE=$(kubectl get events.orchi.cyberorch.com/ctf-2025 -o jsonpath='{.status.phase}' 2>/dev/null || echo "Pending")
  echo "  Attempt $i/30: phase=$PHASE"
  if [ "$PHASE" = "Running" ]; then
    echo "Event is running!"
    break
  fi
  if [ "$PHASE" = "Failed" ]; then
    echo "Event failed!"
    kubectl get events.orchi.cyberorch.com/ctf-2025 -o yaml
    break
  fi
  sleep 10
done

# Verify
kubectl get events.orchi.cyberorch.com
kubectl get labs.orchi.cyberorch.com
```

### Manual Event with Frontend VMs

```bash
kubectl apply -f - <<'EOF'
apiVersion: orchi.cyberorch.com/v1alpha1
kind: Event
metadata:
  name: pentest-workshop
spec:
  tag: pentest-workshop
  name: "Penetration Testing Workshop"
  capacity: 20
  createdBy: admin
  lab:
    exercises:
      - nmap-basics
      - metasploit-intro
    frontends:
      - image: ghcr.io/mrtrkmn/orchi/frontends/kali:latest
        memoryMB: 4096
        cpu: 2
  vpn:
    required: true
EOF
```

---

## Environment Overlays (Dev / Staging / Prod)

Orchi uses Kustomize overlays to customize deployments per environment. The base manifests live in `k8s/` and overlays patch them.

### Comparison

| Setting | Dev | Staging | Prod |
|---|---|---|---|
| **Operator CPU** | 50m–250m | 100m–500m (base) | 100m–500m (base) |
| **Operator Memory** | 64Mi–256Mi | 128Mi–512Mi (base) | 128Mi–512Mi (base) |
| **Amigo replicas** | 1–2 | 1–5 | 2–10 |
| **Store PVC** | 1 GiB | 5 GiB (base) | 20 GiB |
| **Lab resource quota (pods)** | 10 | 50 (base) | 50 (base) |
| **Lab resource quota (CPU)** | 2–4 cores | 8–16 cores (base) | 8–16 cores (base) |
| **Lab resource quota (memory)** | 2–4 GiB | 16–32 GiB (base) | 16–32 GiB (base) |
| **Domain** | `localhost` | `staging.cyberorch.com` | `cyberorch.com` |
| **Container registry** | `localhost:5000/orchi` | `ghcr.io/mrtrkmn/orchi/staging` | `ghcr.io/mrtrkmn/orchi` |
| **Production mode** | `false` | `true` | `true` |
| **External Secrets** | No | No | Yes |
| **S3 Backup CronJob** | No | No | Yes (every 6h) |
| **Velero Schedules** | No | No | Yes (daily full + 4h CRD) |

### How to Deploy a Specific Overlay

```bash
# Preview the rendered manifests for any overlay
kubectl kustomize k8s/overlays/dev
kubectl kustomize k8s/overlays/staging
kubectl kustomize k8s/overlays/prod

# Apply
kubectl apply -k k8s/overlays/<environment>
```

---

## Security Hardening

Orchi implements defense-in-depth across multiple layers:

### Pod Security

- **Pod Security Admission** is enforced on all namespaces:
  - `orchi-system`: `baseline` (allows necessary capabilities)
  - Lab namespaces (`orchi-lab-*`): `restricted` (strictest level)
- All containers run as **non-root** (`runAsNonRoot: true`)
- All containers use **read-only root filesystems**
- All containers drop **ALL** Linux capabilities
- **seccomp** profile `RuntimeDefault` is applied to all pods

### Network Isolation

Seven NetworkPolicies enforce traffic flow:

| Policy | Purpose |
|---|---|
| `default-deny` | Denies all ingress/egress by default |
| `api-frontend` | Allows frontend → API Gateway traffic |
| `store-access` | Allows operator/API → Store traffic |
| `operator-access` | Allows operator → Kubernetes API |
| `guacamole-access` | Allows ingress → Guacamole |
| `intra-lab` | Allows inter-challenge traffic within a lab |
| `vpn-ingress` | Allows VPN client → lab network |

### Secrets Management

- **Never** commit real secrets to version control
- The `orchi-operator-secrets` template has placeholder values
- Production uses **External Secrets Operator** with AWS Secrets Manager
- Alternative: **Sealed Secrets** (Bitnami)

### RBAC

- The operator runs with a dedicated `ServiceAccount`
- A `ClusterRole` grants only the permissions the operator needs (CRD CRUD, namespace management, pod lifecycle)
- GitHub Actions uses a scoped IAM role with OIDC (no long-lived credentials)

---

## Monitoring and Observability

### Option A: CloudWatch Container Insights (AWS-native)

```bash
# Enable Container Insights via EKS add-on
aws eks create-addon \
  --cluster-name orchi-cluster \
  --addon-name amazon-cloudwatch-observability \
  --region eu-north-1

# Verify the add-on is active
aws eks describe-addon \
  --cluster-name orchi-cluster \
  --addon-name amazon-cloudwatch-observability \
  --region eu-north-1 \
  --query 'addon.status'
```

View metrics in the [CloudWatch Container Insights console](https://console.aws.amazon.com/cloudwatch/home#container-insights:infrastructure).

### Option B: Prometheus + Grafana Stack (self-hosted)

The Orchi manifests include `ServiceMonitor`, `PrometheusRule`, and `Grafana Dashboard ConfigMap` resources in `k8s/observability/`.

**Install the Prometheus stack:**

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.enabled=true \
  --set grafana.adminPassword=admin \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false
```

> **Note:** `serviceMonitorSelectorNilUsesHelmValues=false` tells Prometheus to discover all ServiceMonitors in the cluster, including Orchi's.

**Verify the stack:**

```bash
kubectl -n monitoring get pods
# Expected: prometheus, grafana, alertmanager pods running
```

**Access Grafana locally:**

```bash
kubectl -n monitoring port-forward svc/kube-prometheus-stack-grafana 3000:80
# Open http://localhost:3000 (admin/admin)
```

The Orchi dashboard (`k8s/observability/grafana-dashboard-configmap.yaml`) is auto-provisioned and shows:
- Operator reconciliation latency and error rate
- Pod counts per component
- Event lifecycle metrics
- Store read/write latency
- Resource utilization per namespace

### Alerting Rules

Orchi ships with 12 PrometheusRules (`k8s/observability/prometheus-rules.yaml`):

| Alert | Condition | Severity |
|---|---|---|
| OperatorDown | Operator pod not running for 5 min | critical |
| StoreDown | Store pod not running for 5 min | critical |
| HighReconcileErrors | >5% reconcile error rate for 10 min | warning |
| PVCAlmostFull | Store PVC >80% capacity | warning |
| EventStuck | Event in Pending phase for >10 min | warning |
| CertificateExpiringSoon | TLS cert expires in <14 days | warning |
| ... | ... | ... |

---

## Backup and Disaster Recovery

### Store Backup to S3 (Production Only)

The production overlay includes a CronJob (`k8s/workloads/backup-cronjob.yaml`) that backs up the Store PVC to S3 every 6 hours.

**Create the S3 bucket:**

```bash
aws s3 mb s3://orchi-backups-${ACCOUNT_ID} --region eu-north-1

# Enable versioning for extra protection
aws s3api put-bucket-versioning \
  --bucket orchi-backups-${ACCOUNT_ID} \
  --versioning-configuration Status=Enabled

# Set lifecycle policy to expire old backups after 30 days
aws s3api put-bucket-lifecycle-configuration \
  --bucket orchi-backups-${ACCOUNT_ID} \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "ExpireOldBackups",
      "Status": "Enabled",
      "Filter": {"Prefix": "store-backups/"},
      "Expiration": {"Days": 30}
    }]
  }'
```

**Grant the node IAM role S3 access:**

```bash
# Get the node role name
NODE_ROLE=$(aws eks describe-nodegroup \
  --cluster-name orchi-cluster \
  --nodegroup-name orchi-nodes \
  --region eu-north-1 \
  --query 'nodegroup.nodeRole' --output text | awk -F/ '{print $NF}')

# Create a scoped S3 policy (not AmazonS3FullAccess)
cat > orchi-s3-backup-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["s3:PutObject", "s3:GetObject", "s3:ListBucket"],
    "Resource": [
      "arn:aws:s3:::orchi-backups-${ACCOUNT_ID}",
      "arn:aws:s3:::orchi-backups-${ACCOUNT_ID}/*"
    ]
  }]
}
EOF

aws iam create-policy \
  --policy-name orchi-s3-backup \
  --policy-document file://orchi-s3-backup-policy.json

aws iam attach-role-policy \
  --role-name ${NODE_ROLE} \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/orchi-s3-backup
```

### Velero for Full Cluster Backup

[Velero](https://velero.io/) backs up the entire cluster state (or selected namespaces/resources).

**Install Velero:**

```bash
# Install the Velero CLI
brew install velero  # macOS
# Or: https://velero.io/docs/main/basic-install/

# Install Velero in the cluster
velero install \
  --provider aws \
  --plugins velero/velero-plugin-for-aws:v1.11.0 \
  --bucket orchi-backups-${ACCOUNT_ID} \
  --prefix velero \
  --backup-location-config region=eu-north-1 \
  --snapshot-location-config region=eu-north-1 \
  --use-node-agent
```

The production overlay includes two Velero schedules (`k8s/workloads/velero-schedule.yaml`):

| Schedule | Frequency | Retention | What |
|---|---|---|---|
| `orchi-full-backup` | Daily at 02:00 UTC | 7 days | Full cluster backup |
| `orchi-crd-backup` | Every 4 hours | 3 days | CRDs only (Events, Labs, Teams, Challenges) |

**Manually trigger a backup:**

```bash
velero backup create manual-backup-$(date +%Y%m%d)
velero backup describe manual-backup-$(date +%Y%m%d)
```

**Restore from backup:**

```bash
# List available backups
velero backup get

# Restore a specific backup
velero restore create --from-backup orchi-full-backup-20250228020000
```

---

## Upgrades and Rollbacks

### Upgrade the Orchi Platform

**Via GitHub Actions (recommended):**

1. Merge your changes to `master`
2. Tag a new version:
   ```bash
   git tag v2.2.0
   git push origin v2.2.0
   ```
3. The deployment workflow runs automatically

**Via kubectl:**

```bash
# Pull latest manifests
git pull origin master

# Preview changes
kubectl diff -k k8s/overlays/prod

# Apply
kubectl apply -k k8s/overlays/prod

# Monitor rollout
kubectl -n orchi-system rollout status deployment/orchi-operator --timeout=120s
```

### Rollback

**Rollback a specific deployment:**

```bash
# Check rollout history
kubectl -n orchi-system rollout history deployment/orchi-operator

# Rollback to the previous revision
kubectl -n orchi-system rollout undo deployment/orchi-operator

# Rollback to a specific revision
kubectl -n orchi-system rollout undo deployment/orchi-operator --to-revision=3
```

**Rollback the entire Kustomize overlay:**

```bash
# Checkout the previous version
git checkout v2.1.0

# Re-apply the old manifests
kubectl apply -k k8s/overlays/prod

# Go back to latest
git checkout master
```

### Upgrade EKS Cluster Version

```bash
# Check current version
aws eks describe-cluster --name orchi-cluster --region eu-north-1 --query 'cluster.version'

# Upgrade the control plane (one minor version at a time)
aws eks update-cluster-version \
  --name orchi-cluster \
  --kubernetes-version 1.32 \
  --region eu-north-1

# Wait for upgrade (15–20 minutes)
aws eks wait cluster-active --name orchi-cluster --region eu-north-1

# Upgrade the node group
aws eks update-nodegroup-version \
  --cluster-name orchi-cluster \
  --nodegroup-name orchi-nodes \
  --region eu-north-1
```

---

## Cost Optimization

### Resource Recommendations per Environment

| Resource | Dev | Staging | Prod |
|---|---|---|---|
| Node instance type | `t3.medium` (2 vCPU, 4 GiB) | `t3.large` (2 vCPU, 8 GiB) | `m5.xlarge` (4 vCPU, 16 GiB) |
| Node count | 2 | 2–3 | 3–6 |
| EBS volume type | `gp3` | `gp3` | `gp3` |
| Store PVC size | 1 GiB | 5 GiB | 20 GiB |
| NAT Gateway | Single AZ | Single AZ | Multi-AZ |

### Spot Instances for Challenge Pods

Add a spot node group for lab namespaces — challenge pods can tolerate interruptions:

```yaml
# Add to eks-cluster.yaml
managedNodeGroups:
  - name: orchi-spot
    instanceTypes: [t3.large, t3a.large, m5.large]
    spot: true
    desiredCapacity: 0        # scales from 0
    minSize: 0
    maxSize: 20
    labels:
      role: lab
      lifecycle: spot
    taints:
      - key: lifecycle
        value: spot
        effect: NoSchedule
    tags:
      project: orchi
```

Challenge pods need a toleration in the operator config to schedule on spot nodes.

### Scale to Zero When Idle

```bash
# Scale the node group to 0 when not running events
eksctl scale nodegroup \
  --cluster orchi-cluster \
  --name orchi-nodes \
  --nodes 0 --nodes-min 0 \
  --region eu-north-1

# Scale back up before deploying
eksctl scale nodegroup \
  --cluster orchi-cluster \
  --name orchi-nodes \
  --nodes 3 --nodes-min 2 \
  --region eu-north-1
```

### Install Cluster Autoscaler or Karpenter

#### Option A: Karpenter (recommended for EKS)

[Karpenter](https://karpenter.sh/) is the AWS-native node autoscaler that provisions
right-sized nodes faster than Cluster Autoscaler:

```bash
helm repo add karpenter https://charts.karpenter.sh/
helm repo update

helm install karpenter karpenter/karpenter \
  --namespace kube-system \
  --set settings.clusterName=orchi-cluster \
  --set settings.clusterEndpoint=$(aws eks describe-cluster --name orchi-cluster --query "cluster.endpoint" --output text --region eu-north-1)
```

#### Option B: Cluster Autoscaler

```bash
helm repo add autoscaler https://kubernetes.github.io/autoscaler
helm repo update

helm install cluster-autoscaler autoscaler/cluster-autoscaler \
  --namespace kube-system \
  --set autoDiscovery.clusterName=orchi-cluster \
  --set awsRegion=eu-north-1
```

### Estimated Monthly Costs

| Component | Dev (~$) | Prod (~$) |
|---|---|---|
| EKS control plane | 73 | 73 |
| EC2 nodes (3 × t3.large) | 180 | 180 |
| EC2 nodes (3 × m5.xlarge) | — | 415 |
| EBS volumes (3 × 50 GiB gp3) | 12 | 12 |
| NAT Gateway (single AZ) | 32 | 64 (multi-AZ) |
| Load Balancers (NLB × 2) | 36 | 36 |
| S3 (backups) | <1 | <5 |
| **Total** | **~$333** | **~$605** |

> These are rough estimates for the `eu-north-1` region. Actual costs depend on usage, data transfer, and spot savings.

---

## Troubleshooting

### GitHub Actions Cannot Authenticate to AWS

**Symptom:** Workflow fails at "Configure AWS credentials" with `Error: Could not assume role with OIDC`.

**Diagnosis steps:**

```bash
# 1. Verify the OIDC provider exists
aws iam list-open-id-connect-providers | grep token.actions.githubusercontent.com

# 2. Verify the role trust policy
aws iam get-role --role-name orchi-github-actions --query 'Role.AssumeRolePolicyDocument'

# 3. Check the trust policy references your exact repository
# The "sub" condition must match: repo:<org>/<repo>:*
```

**Common fixes:**
- The role trust policy has a typo in the repository name
- Missing `id-token: write` permission in the workflow file
- The OIDC provider ARN in the trust policy doesn't match the one in IAM
- The `aud` condition must be `sts.amazonaws.com`

### kubectl Cannot Access the Cluster

**Symptom:** `error: You must be logged in to the server (Unauthorized)`

**Diagnosis steps:**

```bash
# 1. Verify your current identity
aws sts get-caller-identity

# 2. Re-generate kubeconfig
aws eks update-kubeconfig --name orchi-cluster --region eu-north-1

# 3. Check if using EKS Access Entries
aws eks list-access-entries --cluster-name orchi-cluster --region eu-north-1

# 4. Or check aws-auth ConfigMap
kubectl -n kube-system get configmap aws-auth -o yaml
```

**Fix (Access Entries):**

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
CURRENT_ROLE=$(aws sts get-caller-identity --query Arn --output text)

aws eks create-access-entry \
  --cluster-name orchi-cluster \
  --principal-arn ${CURRENT_ROLE} \
  --type STANDARD \
  --region eu-north-1

aws eks associate-access-policy \
  --cluster-name orchi-cluster \
  --principal-arn ${CURRENT_ROLE} \
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy \
  --access-scope type=cluster \
  --region eu-north-1
```

**Fix (aws-auth):**

```bash
eksctl create iamidentitymapping \
  --cluster orchi-cluster \
  --region eu-north-1 \
  --arn arn:aws:iam::${ACCOUNT_ID}:role/orchi-github-actions \
  --group system:masters \
  --username github-actions
```

### PVCs Stuck in Pending

**Symptom:** `orchi-store` pod stuck in `Pending`, PVC not bound.

**Diagnosis steps:**

```bash
# 1. Check PVC status
kubectl get pvc -A

# 2. Check PVC events for error messages
kubectl describe pvc -n orchi-store

# 3. Verify the EBS CSI driver is running
kubectl -n kube-system get pods -l app.kubernetes.io/name=aws-ebs-csi-driver

# 4. Verify a default StorageClass exists
kubectl get storageclass

# 5. Check the node IAM role has EBS permissions
NODE_ROLE=$(aws eks describe-nodegroup --cluster-name orchi-cluster \
  --nodegroup-name orchi-nodes --region eu-north-1 \
  --query 'nodegroup.nodeRole' --output text | awk -F/ '{print $NF}')
aws iam list-attached-role-policies --role-name ${NODE_ROLE}
```

**Common fixes:**
- EBS CSI driver not installed → run `eksctl create addon --name aws-ebs-csi-driver ...`
- No default StorageClass → create the `gp3` StorageClass from [Step 3.2](#32-install-the-ebs-csi-driver-for-pvcs)
- Node IAM role missing `AmazonEBSCSIDriverPolicy` → attach the policy

### NetworkPolicies Not Enforced

**Symptom:** Pods can communicate despite deny-all policies.

**Diagnosis:**

```bash
# Check if a NetworkPolicy-capable CNI is running
kubectl -n kube-system get pods | grep -E "calico|cilium"
```

**Fix:** The Amazon VPC CNI does not enforce NetworkPolicy. Install Calico or Cilium — see [Step 3.1](#31-install-a-cni-with-networkpolicy-support).

### LoadBalancer Service Has No External IP

**Symptom:** WireGuard service stays in `<pending>` state for more than 5 minutes.

**Diagnosis:**

```bash
# 1. Check service events
kubectl -n orchi-system describe svc wireguard

# 2. Verify subnets are tagged for load balancer discovery
aws ec2 describe-subnets \
  --filters "Name=tag:kubernetes.io/cluster/orchi-cluster,Values=shared,owned" \
  --query 'Subnets[].{ID:SubnetId,AZ:AvailabilityZone,Tags:Tags}' \
  --region eu-north-1
```

**Fix:**
- Public subnets need the tag `kubernetes.io/role/elb: 1`
- Private subnets need the tag `kubernetes.io/role/internal-elb: 1`
- If using `eksctl`, these tags are set automatically

```bash
# Tag a public subnet for external load balancers
aws ec2 create-tags --resources subnet-xxx \
  --tags Key=kubernetes.io/role/elb,Value=1
```

### Operator Pod CrashLooping

**Symptom:** Operator pod is in `CrashLoopBackOff`.

**Diagnosis:**

```bash
# Check logs
kubectl -n orchi-system logs deploy/orchi-operator --previous --tail=100

# Check events
kubectl -n orchi-system get events --sort-by=.lastTimestamp | grep operator

# Check the operator secret exists
kubectl -n orchi-system get secret orchi-operator-secrets
```

**Common fixes:**
- Missing `orchi-operator-secrets` → create it per [Step 4](#step-4--create-operator-secrets)
- Secrets contain placeholder values (`REPLACE_BEFORE_DEPLOY`) → recreate with real values
- Missing RBAC permissions → verify the ClusterRole and ClusterRoleBinding exist

### Certificate Not Issuing

**Symptom:** Certificate shows `READY: False` for more than 5 minutes.

**Diagnosis:**

```bash
# Check the certificate status
kubectl -n orchi-system describe certificate cyberorch-wildcard-cert

# Check cert-manager logs
kubectl -n cert-manager logs deploy/cert-manager --tail=50

# Check ACME challenges
kubectl get challenges -A

# Check ACME orders
kubectl get orders -A
```

**Common fixes:**
- Cloudflare API token missing or invalid → recreate the secret in Step 3.5.2
- Wrong DNS zone permissions → verify the token has Zone:DNS:Edit for `cyberorch.com`
- Let's Encrypt rate limit hit → use the staging issuer first, wait for rate limit reset (1 week)

### Event Stuck in Pending Phase

**Symptom:** Event CR created but never reaches `Running` phase.

**Diagnosis:**

```bash
# Check event status
kubectl get events.orchi.cyberorch.com/<event-tag> -o yaml

# Check operator logs for reconciliation errors
kubectl -n orchi-system logs deploy/orchi-operator --tail=100 | grep -i "error\|fail\|reconcile"

# Check if the lab namespace was created
kubectl get ns | grep orchi-lab
```

**Common fixes:**
- Operator not running → check operator pod status
- Exercise tags don't match available exercises → verify exercise tags exist
- Insufficient cluster resources → check if nodes have available CPU/memory

---

## Quick-Reference Cheat Sheet

### Common kubectl Commands

```bash
# Cluster access
aws eks update-kubeconfig --name orchi-cluster --region eu-north-1

# View all Orchi resources
kubectl -n orchi-system get all
kubectl get crds | grep orchi
kubectl get events.orchi.cyberorch.com
kubectl get labs.orchi.cyberorch.com
kubectl get teams.orchi.cyberorch.com -A
kubectl get challenges.orchi.cyberorch.com -A

# Operator logs
kubectl -n orchi-system logs deploy/orchi-operator -f

# Restart a deployment
kubectl -n orchi-system rollout restart deployment/orchi-operator
kubectl -n orchi-system rollout restart deployment/amigo

# Scale a deployment
kubectl -n orchi-system scale deployment/amigo --replicas=5

# Get events for debugging
kubectl -n orchi-system get events --sort-by=.lastTimestamp

# Port-forward for local access
kubectl -n orchi-system port-forward svc/amigo 8080:80
kubectl -n orchi-system port-forward svc/guacamole 8081:8080
```

### GitHub Actions Workflows

| Workflow | Trigger | Purpose |
|---|---|---|
| **Deploy Orchi Platform** | Manual dispatch or `v*.*.*` tag push | Deploy/update the platform |
| **Create Event** | Manual dispatch | Create a new CTF/lab event |
| **Stop Event** | Manual dispatch | Delete an event and clean up |
| **Test** | Push/PR to master, develop, feature/\*, hotfix/\* | Run unit tests |
| **Formalities** | Any push | Validate branch naming conventions |

### Important Paths

| Path | Description |
|---|---|
| `k8s/overlays/<env>/` | Environment-specific Kustomize overlays |
| `k8s/crds/` | Custom Resource Definitions |
| `k8s/networking/cert-manager.yaml` | TLS certificate configuration |
| `k8s/networking/ingress.yaml` | Ingress routing rules |
| `k8s/workloads/` | All Deployment/StatefulSet/Job manifests |
| `k8s/observability/` | Prometheus rules and Grafana dashboards |
| `.github/workflows/` | CI/CD pipeline definitions |

For more general troubleshooting, see [troubleshooting.md](troubleshooting.md).
