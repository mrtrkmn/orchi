# AWS EKS Deployment Guide

This guide walks through deploying the Orchi platform on Amazon EKS and configuring GitHub Actions for automated deployment and event management.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Step 1 — Create an EKS Cluster](#step-1--create-an-eks-cluster)
- [Step 2 — Configure IAM for GitHub Actions (OIDC)](#step-2--configure-iam-for-github-actions-oidc)
- [Step 3 — Install Cluster Dependencies](#step-3--install-cluster-dependencies)
- [Step 4 — Configure GitHub Repository Secrets](#step-4--configure-github-repository-secrets)
- [Step 5 — Deploy Orchi via GitHub Actions](#step-5--deploy-orchi-via-github-actions)
- [Step 6 — Create an Event via GitHub Actions](#step-6--create-an-event-via-github-actions)
- [Step 7 — Verify the Deployment](#step-7--verify-the-deployment)
- [Manual Deployment (without GitHub Actions)](#manual-deployment-without-github-actions)
- [Monitoring and Observability](#monitoring-and-observability)
- [Backup and Disaster Recovery](#backup-and-disaster-recovery)
- [Cost Optimization](#cost-optimization)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- An AWS account with permissions to create EKS clusters, IAM roles, and VPCs
- [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) configured with credentials
- [eksctl](https://eksctl.io/installation/) for cluster creation
- [kubectl](https://kubernetes.io/docs/tasks/tools/) 1.28+
- [Helm](https://helm.sh/docs/intro/install/) 3.x (for cluster dependencies)
- A GitHub repository with Actions enabled

## Step 1 — Create an EKS Cluster

### Option A: Using eksctl (recommended)

Create a cluster configuration file `eks-cluster.yaml`:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: orchi-cluster
  region: eu-west-1    # change to your preferred region
  version: "1.31"

iam:
  withOIDC: true       # required for GitHub Actions OIDC auth

managedNodeGroups:
  - name: orchi-nodes
    instanceType: t3.large
    desiredCapacity: 3
    minSize: 2
    maxSize: 6
    volumeSize: 50
    labels:
      role: orchi
    tags:
      project: orchi
    iam:
      withAddonPolicies:
        ebs: true          # for PVC (store StatefulSet)
        albIngress: true   # for AWS Load Balancer Controller
        cloudWatch: true   # for logging
```

Create the cluster:

```bash
eksctl create cluster -f eks-cluster.yaml
```

This takes 15–20 minutes. It creates the VPC, subnets, security groups, and node group automatically.

### Option B: Using AWS CLI

```bash
# Create the cluster
aws eks create-cluster \
  --name orchi-cluster \
  --region eu-west-1 \
  --kubernetes-version 1.31 \
  --role-arn arn:aws:iam::ACCOUNT_ID:role/eks-cluster-role \
  --resources-vpc-config subnetIds=subnet-xxx,subnet-yyy,securityGroupIds=sg-zzz

# Wait for cluster to be active
aws eks wait cluster-active --name orchi-cluster --region eu-west-1

# Create a managed node group
aws eks create-nodegroup \
  --cluster-name orchi-cluster \
  --nodegroup-name orchi-nodes \
  --node-role arn:aws:iam::ACCOUNT_ID:role/eks-node-role \
  --instance-types t3.large \
  --scaling-config minSize=2,maxSize=6,desiredSize=3 \
  --disk-size 50 \
  --region eu-west-1
```

### Configure kubectl

```bash
aws eks update-kubeconfig --name orchi-cluster --region eu-west-1
kubectl get nodes   # verify connectivity
```

## Step 2 — Configure IAM for GitHub Actions (OIDC)

GitHub Actions authenticates to AWS using OIDC (OpenID Connect), eliminating the need for long-lived AWS access keys.

### 2.1 Create the OIDC Identity Provider

If you used `eksctl` with `withOIDC: true`, the EKS OIDC provider is already created. For GitHub Actions, you need a separate OIDC provider:

```bash
# Create GitHub OIDC provider in IAM
# Note: As of 2024, AWS no longer requires the --thumbprint-list parameter
# for GitHub Actions OIDC. AWS validates the certificate chain automatically.
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

### 2.2 Create an IAM Role for GitHub Actions

Create a trust policy file `github-actions-trust-policy.json`:

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

> **Security:** The `sub` condition restricts this role to workflows running in the `mrtrkmn/orchi` repository. Adjust to your org/repo.

Create the role and attach EKS permissions:

```bash
# Create the role
aws iam create-role \
  --role-name orchi-github-actions \
  --assume-role-policy-document file://github-actions-trust-policy.json

# Attach EKS access policy
aws iam attach-role-policy \
  --role-name orchi-github-actions \
  --policy-arn arn:aws:iam::aws:policy/AmazonEKSClusterPolicy

# Create and attach a custom policy for kubectl access
cat > eks-kubectl-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
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
  --policy-arn arn:aws:iam::ACCOUNT_ID:policy/orchi-eks-kubectl
```

### 2.3 Grant the IAM Role Access to the Cluster

The IAM role needs Kubernetes RBAC permissions. EKS supports two methods:

#### Option A: EKS Access Entries (recommended, EKS 1.30+)

EKS Access Entries are the modern, API-native way to manage cluster access without
editing the `aws-auth` ConfigMap directly. The cluster version 1.31 used in this
guide fully supports Access Entries:

```bash
# Create an access entry for the GitHub Actions role
aws eks create-access-entry \
  --cluster-name orchi-cluster \
  --principal-arn arn:aws:iam::ACCOUNT_ID:role/orchi-github-actions \
  --type STANDARD \
  --region eu-west-1

# Associate a policy (ClusterAdmin for deployments)
aws eks associate-access-policy \
  --cluster-name orchi-cluster \
  --principal-arn arn:aws:iam::ACCOUNT_ID:role/orchi-github-actions \
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy \
  --access-scope type=cluster \
  --region eu-west-1
```

> **Note:** For production, consider `AmazonEKSAdminPolicy` (namespace-scoped) instead
> of `AmazonEKSClusterAdminPolicy` and scope access to the `orchi-system` namespace.

#### Option B: aws-auth ConfigMap (legacy, all EKS versions)

Add an entry to the `aws-auth` ConfigMap:

```bash
# Check current auth map
kubectl -n kube-system get configmap aws-auth -o yaml

# Add the GitHub Actions role
eksctl create iamidentitymapping \
  --cluster orchi-cluster \
  --region eu-west-1 \
  --arn arn:aws:iam::ACCOUNT_ID:role/orchi-github-actions \
  --group system:masters \
  --username github-actions
```

> **Note:** For production, consider using a more restrictive Kubernetes RBAC role instead of `system:masters`. See the [Kubernetes RBAC docs](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

## Step 3 — Install Cluster Dependencies

Orchi requires a CNI with NetworkPolicy support and optionally a CSI driver for persistent volumes.

### 3.1 Install a CNI with NetworkPolicy Support

EKS ships with the Amazon VPC CNI, which does **not** support NetworkPolicy. Install Calico for policy enforcement:

```bash
# Install Calico for NetworkPolicy support
kubectl apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.29.0/manifests/calico-vxlan.yaml
```

Alternatively, use Cilium:

```bash
helm repo add cilium https://helm.cilium.io/
helm install cilium cilium/cilium --version 1.16.5 \
  --namespace kube-system \
  --set eni.enabled=true \
  --set ipam.mode=eni \
  --set egressMasqueradeInterfaces=eth0 \
  --set routingMode=native
```

### 3.2 Install the EBS CSI Driver (for PVCs)

The orchi-store StatefulSet requires persistent volumes:

```bash
# Install the EBS CSI driver add-on
eksctl create addon \
  --cluster orchi-cluster \
  --name aws-ebs-csi-driver \
  --region eu-west-1 \
  --force

# Create a gp3 StorageClass
kubectl apply -f - <<EOF
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

### 3.3 Install the AWS Load Balancer Controller (optional, for Ingress)

If using AWS ALB for ingress instead of nginx:

```bash
helm repo add eks https://aws.github.io/eks-charts
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=orchi-cluster \
  --set serviceAccount.create=true
```

### 3.4 Install cert-manager (for TLS)

```bash
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true
```

### 3.5 Configure Cloudflare DNS for Let's Encrypt Wildcard Certificates

Orchi uses a wildcard TLS certificate (`*.cyberorch.com`) issued by Let's Encrypt.
Wildcard certificates require DNS-01 challenge validation, which is handled by
cert-manager using the Cloudflare API.

#### 3.5.1 Create a Cloudflare API Token

1. Go to [Cloudflare Dashboard](https://dash.cloudflare.com/profile/api-tokens)
2. Click **Create Token**
3. Use the **Edit zone DNS** template
4. Configure permissions:
   - **Zone → DNS → Edit**
   - **Zone → Zone → Read**
5. Zone Resources: **Include → Specific zone → cyberorch.com**
6. Click **Create Token** and copy the token value

#### 3.5.2 Create the Cloudflare Secret in Kubernetes

```bash
kubectl create secret generic cloudflare-api-token \
  --namespace cert-manager \
  --from-literal=api-token=<YOUR_CLOUDFLARE_API_TOKEN>
```

> **Security:** The token only needs DNS edit permissions for the `cyberorch.com` zone.
> Do not use a Global API Key. The secret template in `k8s/networking/cert-manager.yaml`
> is a placeholder — use `kubectl create secret` or External Secrets Operator in production.

#### 3.5.3 Deploy the ClusterIssuer and Certificate

The cert-manager resources are included in the Kustomize base (`k8s/networking/cert-manager.yaml`).
After deploying Orchi, verify the certificate is issued:

```bash
# Check ClusterIssuers
kubectl get clusterissuers

# Check certificate status
kubectl -n orchi-system get certificates
kubectl -n orchi-system describe certificate cyberorch-wildcard-cert

# Check the TLS secret was created
kubectl -n orchi-system get secret cyberorch-tls-cert
```

If using staging first (recommended for testing):

```bash
# Temporarily switch to staging issuer to avoid rate limits
kubectl -n orchi-system patch certificate cyberorch-wildcard-cert \
  --type merge -p '{"spec":{"issuerRef":{"name":"letsencrypt-staging"}}}'
```

## Step 4 — Configure GitHub Repository Secrets

Go to your repository **Settings → Secrets and variables → Actions** and add:

| Secret | Value | Description |
|---|---|---|
| `AWS_ROLE_ARN` | `arn:aws:iam::ACCOUNT_ID:role/orchi-github-actions` | IAM role ARN from Step 2 |
| `AWS_REGION` | `eu-west-1` | AWS region where the cluster runs |
| `EKS_CLUSTER_NAME` | `orchi-cluster` | EKS cluster name |
| `CLOUDFLARE_API_TOKEN` | `<token>` | *(Optional)* Cloudflare API token — only needed if using CI/CD to create the K8s secret. Otherwise create manually per Step 3.5 |

For environment-specific secrets, create GitHub Environments (`dev`, `staging`, `prod`) under **Settings → Environments** and add the secrets per environment. This allows different clusters per environment.

## Step 5 — Deploy Orchi via GitHub Actions

1. Go to **Actions → Deploy Orchi Platform → Run workflow**
2. Select:
   - **Environment:** `dev`, `staging`, or `prod`
   - **Cluster provider:** `aws`
   - **Dry run:** `true` (first time, to preview changes)
3. Click **Run workflow**

The dry-run shows what would be applied without making changes. Once satisfied:

4. Run again with **Dry run:** `false` to deploy

The workflow will:
1. Authenticate to AWS using OIDC (no credentials stored)
2. Run `aws eks update-kubeconfig` to configure kubectl
3. Run `kubectl apply -k k8s/overlays/<environment>` to deploy all manifests
4. Wait for the operator and Amigo deployments to roll out
5. Print pod, service, and CRD status

### Automatic Deployment on Tags

Pushing a version tag triggers an automatic production deployment:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Step 6 — Create an Event via GitHub Actions

1. Go to **Actions → Create Event → Run workflow**
2. Fill in:
   - **Event tag:** `ctf-2024` (unique, lowercase, hyphens allowed)
   - **Event name:** `CTF Competition 2024`
   - **Capacity:** `50`
   - **Exercises:** `sql-injection,xss-basic,buffer-overflow`
   - **Frontend image:** *(optional)* `ghcr.io/mrtrkmn/orchi/frontends/kali:latest`
   - **Environment:** `prod`
   - **Cluster provider:** `aws`
3. Click **Run workflow**

The workflow generates an Event custom resource and applies it to the cluster. The operator then reconciles the event into:
- A lab namespace (`orchi-lab-ctf-2024`)
- Challenge pods for each exercise
- NetworkPolicies for isolation
- DNS records for the lab

### Stop an Event

Go to **Actions → Stop Event → Run workflow**, enter the event tag and cluster provider (`aws`), and run.

## Step 7 — Verify the Deployment

```bash
# Configure kubectl for local verification
aws eks update-kubeconfig --name orchi-cluster --region eu-west-1

# Check CRDs
kubectl get crds | grep orchi

# Check operator and workloads
kubectl -n orchi-system get pods

# Check events
kubectl get events.orchi.cyberorch.com

# Check labs
kubectl get labs.orchi.cyberorch.com

# Check ingress
kubectl -n orchi-system get ingress
```

## Manual Deployment (without GitHub Actions)

If you prefer to deploy without GitHub Actions:

```bash
# 1. Configure kubectl
aws eks update-kubeconfig --name orchi-cluster --region eu-west-1

# 2. Deploy (choose environment)
kubectl apply -k k8s/overlays/dev      # development
kubectl apply -k k8s/overlays/staging   # staging
kubectl apply -k k8s/overlays/prod      # production

# 3. Wait for rollout
kubectl -n orchi-system rollout status deployment/orchi-operator --timeout=120s
kubectl -n orchi-system rollout status deployment/amigo --timeout=120s

# 4. Create an event
kubectl apply -f - <<EOF
apiVersion: orchi.cyberorch.com/v1alpha1
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
EOF

# 5. Verify
kubectl get events.orchi.cyberorch.com
kubectl -n orchi-system get pods
```

## Monitoring and Observability

### CloudWatch Container Insights (optional)

```bash
# Enable Container Insights for the cluster
aws eks create-addon \
  --cluster-name orchi-cluster \
  --addon-name amazon-cloudwatch-observability \
  --region eu-west-1
```

### Prometheus and Grafana

The Orchi manifests include ServiceMonitors and PrometheusRules. Install the Prometheus stack:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.enabled=true
```

The Orchi Grafana dashboard (`k8s/observability/grafana-dashboard-configmap.yaml`) is automatically provisioned when the stack is installed.

## Backup and Disaster Recovery

### Store Backup to S3

The production overlay includes a CronJob that backs up the store PVC to S3 every 6 hours. Ensure the node IAM role has S3 write permissions:

```bash
aws iam attach-role-policy \
  --role-name eksctl-orchi-cluster-nodegroup-NodeInstanceRole-XXXXX \
  --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess
```

### Velero for Cluster Backup

```bash
# Install Velero with AWS plugin
velero install \
  --provider aws \
  --plugins velero/velero-plugin-for-aws:v1.11.0 \
  --bucket orchi-backups \
  --backup-location-config region=eu-west-1 \
  --snapshot-location-config region=eu-west-1 \
  --use-node-agent

# The production overlay includes Velero schedules:
# - orchi-full-backup: daily, 7-day retention
# - orchi-crd-backup: every 4h, 3-day retention
```

## Cost Optimization

| Resource | Recommendation |
|---|---|
| Node instances | Use `t3.large` for dev, `m5.xlarge` for prod |
| Spot instances | Add a spot node group for challenge pods (can tolerate interruptions) |
| Cluster autoscaler | Install to scale nodes based on demand |
| EBS volumes | Use `gp3` (cheaper than `gp2` at same performance) |
| NAT Gateway | Use a single NAT Gateway for dev; multi-AZ for prod |
| Idle clusters | Scale node group to 0 when not running events |

### Install Cluster Autoscaler or Karpenter

#### Option A: Karpenter (recommended for EKS)

[Karpenter](https://karpenter.sh/) is the AWS-native node autoscaler that provisions
right-sized nodes faster than Cluster Autoscaler:

```bash
helm repo add karpenter https://charts.karpenter.sh/
helm install karpenter karpenter/karpenter \
  --namespace kube-system \
  --set settings.clusterName=orchi-cluster \
  --set settings.clusterEndpoint=$(aws eks describe-cluster --name orchi-cluster --query "cluster.endpoint" --output text)
```

#### Option B: Cluster Autoscaler

```bash
helm repo add autoscaler https://kubernetes.github.io/autoscaler
helm install cluster-autoscaler autoscaler/cluster-autoscaler \
  --namespace kube-system \
  --set autoDiscovery.clusterName=orchi-cluster \
  --set awsRegion=eu-west-1
```

## Troubleshooting

### GitHub Actions Cannot Authenticate to AWS

**Symptom:** Workflow fails at "Configure AWS credentials" step.

**Fix:**
1. Verify the OIDC provider exists: `aws iam list-open-id-connect-providers`
2. Verify the role trust policy references the correct repository
3. Check that `id-token: write` permission is set in the workflow

### kubectl Cannot Access the Cluster

**Symptom:** `error: You must be logged in to the server (Unauthorized)`

**Fix:**
1. Verify the IAM role is mapped in `aws-auth`:
   ```bash
   kubectl -n kube-system get configmap aws-auth -o yaml
   ```
2. Re-add the mapping:
   ```bash
   eksctl create iamidentitymapping \
     --cluster orchi-cluster \
     --arn arn:aws:iam::ACCOUNT_ID:role/orchi-github-actions \
     --group system:masters \
     --username github-actions
   ```

### PVCs Stuck in Pending

**Symptom:** `orchi-store` pod stuck in `Pending`, PVC not bound.

**Fix:**
1. Verify the EBS CSI driver is installed: `kubectl get pods -n kube-system -l app.kubernetes.io/name=aws-ebs-csi-driver`
2. Verify a default StorageClass exists: `kubectl get storageclass`
3. Check the node IAM role has EBS permissions

### NetworkPolicies Not Enforced

**Symptom:** Pods can communicate despite deny-all policies.

**Fix:** The Amazon VPC CNI does not enforce NetworkPolicy. Install Calico or Cilium (see [Step 3.1](#31-install-a-cni-with-networkpolicy-support)).

### LoadBalancer Service Has No External IP

**Symptom:** WireGuard service stays in `<pending>` state.

**Fix:**
1. Verify subnets are tagged for load balancer discovery:
   ```bash
   aws ec2 describe-subnets --filters "Name=tag:kubernetes.io/cluster/orchi-cluster,Values=shared,owned" \
     --query 'Subnets[].{ID:SubnetId,AZ:AvailabilityZone,Tags:Tags}'
   ```
2. Public subnets need the tag `kubernetes.io/role/elb: 1`
3. Private subnets need `kubernetes.io/role/internal-elb: 1`

For more general troubleshooting, see [troubleshooting.md](troubleshooting.md).
