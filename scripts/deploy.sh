#!/usr/bin/env bash
# ==============================================================================
# Orchi Platform — Deployment Script
# ==============================================================================
# Single deployment script for all environments: prod, staging, and dev.
# Handles cluster authentication, manifest validation, and rollout verification.
#
# Usage:
#   ./scripts/deploy.sh -e <environment> [-p <provider>] [-d] [-r] [-h]
#
# Examples:
#   ./scripts/deploy.sh -e dev                          # Deploy to dev (kubeconfig)
#   ./scripts/deploy.sh -e staging -p aws               # Deploy to staging via AWS EKS
#   ./scripts/deploy.sh -e prod -p aws                  # Deploy to prod via AWS EKS
#   ./scripts/deploy.sh -e prod -p aws -d               # Dry run (diff only)
#   ./scripts/deploy.sh -e prod -p kubeconfig            # Deploy to prod via kubeconfig
#   ./scripts/deploy.sh -e prod -p aws -r               # Remove ALL orchi resources
# ==============================================================================

set -euo pipefail

# ==============================================================================
# Configuration — Per-Environment Settings
# ==============================================================================
# These values mirror the Kustomize overlays in k8s/overlays/<env>/

# --- Production ---
PROD_HOST_HTTP="cyberorch.com"
PROD_HOST_GRPC="grpc.cyberorch.com"
PROD_PRODUCTION_MODE="true"
PROD_REGISTRY="ghcr.io/mrtrkmn/orchi"
PROD_AMIGO_MIN_REPLICAS=2
PROD_AMIGO_MAX_REPLICAS=10
PROD_STORE_PVC_SIZE="20Gi"
PROD_BACKUPS_ENABLED="true"

# --- Staging ---
STAGING_HOST_HTTP="staging.cyberorch.com"
STAGING_HOST_GRPC="grpc.staging.cyberorch.com"
STAGING_PRODUCTION_MODE="true"
STAGING_REGISTRY="ghcr.io/mrtrkmn/orchi/staging"
STAGING_AMIGO_MIN_REPLICAS=1
STAGING_AMIGO_MAX_REPLICAS=5
STAGING_STORE_PVC_SIZE="5Gi"
STAGING_BACKUPS_ENABLED="false"

# --- Dev ---
DEV_HOST_HTTP="localhost"
DEV_HOST_GRPC="localhost"
DEV_PRODUCTION_MODE="false"
DEV_REGISTRY="localhost:5000/orchi"
DEV_AMIGO_MIN_REPLICAS=1
DEV_AMIGO_MAX_REPLICAS=2
DEV_STORE_PVC_SIZE="1Gi"
DEV_BACKUPS_ENABLED="false"

# --- Shared ---
NAMESPACE="orchi-system"
KUSTOMIZE_BASE="k8s/overlays"
ROLLOUT_TIMEOUT="120s"

# --- AWS EKS defaults (override via environment variables) ---
AWS_REGION="${AWS_REGION:-eu-north-1}"
EKS_CLUSTER_NAME="${EKS_CLUSTER_NAME:-orchi-cluster}"

# ==============================================================================
# Functions
# ==============================================================================

usage() {
    cat <<EOF
Usage: $(basename "$0") -e <environment> [-p <provider>] [-d] [-r] [-h]

Deploy the Orchi platform to the specified environment.

Options:
  -e <environment>   Target environment: prod, staging, or dev (required)
  -p <provider>      Cluster provider: kubeconfig or aws (default: kubeconfig)
  -d                 Dry run — show diff only, do not apply
  -r                 Remove ALL orchi resources (namespace, CRDs, Helm releases)
  -h                 Show this help message

Environment variables (for AWS provider):
  AWS_REGION          AWS region (default: eu-north-1)
  EKS_CLUSTER_NAME    EKS cluster name (default: orchi-cluster)
  AWS_ROLE_ARN        IAM role ARN for OIDC authentication (optional)
  KUBECONFIG          Path to kubeconfig file (for kubeconfig provider)

Environment config summary:
$(printf "  %-10s %-25s replicas: %-6s PVC: %-6s backups: %s\n" \
    "prod"    "${PROD_HOST_HTTP}"    "${PROD_AMIGO_MIN_REPLICAS}-${PROD_AMIGO_MAX_REPLICAS}" \
    "${PROD_STORE_PVC_SIZE}" "${PROD_BACKUPS_ENABLED}")
$(printf "  %-10s %-25s replicas: %-6s PVC: %-6s backups: %s\n" \
    "staging"  "${STAGING_HOST_HTTP}" "${STAGING_AMIGO_MIN_REPLICAS}-${STAGING_AMIGO_MAX_REPLICAS}" \
    "${STAGING_STORE_PVC_SIZE}" "${STAGING_BACKUPS_ENABLED}")
$(printf "  %-10s %-25s replicas: %-6s PVC: %-6s backups: %s\n" \
    "dev"      "${DEV_HOST_HTTP}"     "${DEV_AMIGO_MIN_REPLICAS}-${DEV_AMIGO_MAX_REPLICAS}" \
    "${DEV_STORE_PVC_SIZE}" "${DEV_BACKUPS_ENABLED}")
EOF
    exit 0
}

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
    exit 1
}

# Print environment-specific config for the selected environment
print_config() {
    local env="$1"
    case "${env}" in
        prod)
            log "Config: host=${PROD_HOST_HTTP} grpc=${PROD_HOST_GRPC} mode=${PROD_PRODUCTION_MODE} registry=${PROD_REGISTRY}"
            log "Config: amigo replicas=${PROD_AMIGO_MIN_REPLICAS}-${PROD_AMIGO_MAX_REPLICAS} store PVC=${PROD_STORE_PVC_SIZE} backups=${PROD_BACKUPS_ENABLED}"
            ;;
        staging)
            log "Config: host=${STAGING_HOST_HTTP} grpc=${STAGING_HOST_GRPC} mode=${STAGING_PRODUCTION_MODE} registry=${STAGING_REGISTRY}"
            log "Config: amigo replicas=${STAGING_AMIGO_MIN_REPLICAS}-${STAGING_AMIGO_MAX_REPLICAS} store PVC=${STAGING_STORE_PVC_SIZE} backups=${STAGING_BACKUPS_ENABLED}"
            ;;
        dev)
            log "Config: host=${DEV_HOST_HTTP} grpc=${DEV_HOST_GRPC} mode=${DEV_PRODUCTION_MODE} registry=${DEV_REGISTRY}"
            log "Config: amigo replicas=${DEV_AMIGO_MIN_REPLICAS}-${DEV_AMIGO_MAX_REPLICAS} store PVC=${DEV_STORE_PVC_SIZE} backups=${DEV_BACKUPS_ENABLED}"
            ;;
    esac
}

# ==============================================================================
# Step 1: Check Prerequisites
# ==============================================================================
check_prerequisites() {
    log "Checking prerequisites..."

    if ! command -v kubectl &>/dev/null; then
        error "kubectl is not installed. See https://kubernetes.io/docs/tasks/tools/"
    fi

    if [ "${PROVIDER}" = "aws" ]; then
        if ! command -v aws &>/dev/null; then
            error "AWS CLI is not installed. See https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html"
        fi
    fi

    # Verify kustomize overlay directory exists
    if [ ! -d "${KUSTOMIZE_BASE}/${ENVIRONMENT}" ]; then
        error "Kustomize overlay not found: ${KUSTOMIZE_BASE}/${ENVIRONMENT}"
    fi

    log "Prerequisites OK"
}

# ==============================================================================
# Step 2: Configure Cluster Access
# ==============================================================================
configure_cluster() {
    log "Configuring cluster access (provider: ${PROVIDER})..."

    case "${PROVIDER}" in
        kubeconfig)
            if [ -n "${KUBECONFIG:-}" ]; then
                log "Using KUBECONFIG=${KUBECONFIG}"
            elif [ -f "${HOME}/.kube/config" ]; then
                log "Using default kubeconfig at ${HOME}/.kube/config"
            else
                error "No kubeconfig found. Set KUBECONFIG or place config at ${HOME}/.kube/config"
            fi
            ;;
        aws)
            log "Configuring AWS EKS access (cluster: ${EKS_CLUSTER_NAME}, region: ${AWS_REGION})..."
            aws eks update-kubeconfig \
                --name "${EKS_CLUSTER_NAME}" \
                --region "${AWS_REGION}"
            ;;
        *)
            error "Unknown provider: ${PROVIDER}. Use 'kubeconfig' or 'aws'."
            ;;
    esac

    # Verify connectivity
    if ! kubectl cluster-info &>/dev/null; then
        error "Cannot connect to Kubernetes cluster. Check your credentials."
    fi

    log "Cluster access configured"
}

# ==============================================================================
# Step 3: Validate Manifests
# ==============================================================================
validate_manifests() {
    log "Validating Kustomize manifests for '${ENVIRONMENT}'..."

    if ! kubectl kustomize "${KUSTOMIZE_BASE}/${ENVIRONMENT}" >/dev/null; then
        error "Manifest validation failed for ${KUSTOMIZE_BASE}/${ENVIRONMENT}"
    fi

    log "Manifests valid"
}

# ==============================================================================
# Step 4: Diff / Dry Run
# ==============================================================================
diff_manifests() {
    log "Showing diff for '${ENVIRONMENT}' (dry run)..."

    # kubectl diff exit codes: 0 = no diff, 1 = diff found, >1 = error
    local rc=0
    kubectl diff -k "${KUSTOMIZE_BASE}/${ENVIRONMENT}" || rc=$?

    if [ "${rc}" -eq 0 ]; then
        log "No differences found — cluster is up to date"
    elif [ "${rc}" -eq 1 ]; then
        log "Diff complete — differences shown above, no changes applied"
    else
        error "kubectl diff failed (exit code ${rc}). Check cluster connectivity and credentials."
    fi
}

# ==============================================================================
# Step 5: Apply Manifests
# ==============================================================================
apply_manifests() {
    log "Deploying to '${ENVIRONMENT}'..."

    kubectl apply -k "${KUSTOMIZE_BASE}/${ENVIRONMENT}"

    log "Manifests applied"
}

# ==============================================================================
# Step 6: Wait for Rollout
# ==============================================================================
wait_for_rollout() {
    log "Waiting for rollout to complete (timeout: ${ROLLOUT_TIMEOUT})..."

    log "Waiting for orchi-operator..."
    kubectl -n "${NAMESPACE}" rollout status deployment/orchi-operator --timeout="${ROLLOUT_TIMEOUT}"

    log "Waiting for amigo..."
    kubectl -n "${NAMESPACE}" rollout status deployment/amigo --timeout="${ROLLOUT_TIMEOUT}"

    log "Rollout complete"
}

# ==============================================================================
# Step 7: Remove All
# ==============================================================================
remove_all() {
    log "============================================"
    log "  REMOVING ALL ORCHI RESOURCES"
    log "============================================"
    echo ""
    log "WARNING: This will delete:"
    log "  - All resources in namespace '${NAMESPACE}'"
    log "  - Orchi CRDs and all custom resources"
    log "  - Helm releases (external-secrets, kube-prometheus-stack, velero)"
    log "  - Persistent volumes and data"
    echo ""
    read -r -p "Type 'yes-remove-everything' to confirm: " confirm
    if [ "${confirm}" != "yes-remove-everything" ]; then
        log "Aborted. No changes made."
        exit 0
    fi

    echo ""

    # ── Step 1: Delete Kustomize-managed resources ──
    log "[1/6] Deleting Kustomize-managed resources..."
    if kubectl kustomize "${KUSTOMIZE_BASE}/${ENVIRONMENT}" &>/dev/null; then
        kubectl delete -k "${KUSTOMIZE_BASE}/${ENVIRONMENT}" --ignore-not-found --wait=false || true
    fi
    log "  Kustomize resources deleted"

    # ── Step 2: Delete remaining workloads in namespace ──
    log "[2/6] Cleaning up remaining workloads in ${NAMESPACE}..."
    for kind in deployment statefulset daemonset job cronjob; do
        kubectl -n "${NAMESPACE}" delete "${kind}" --all --ignore-not-found --wait=false 2>/dev/null || true
    done
    kubectl -n "${NAMESPACE}" delete pods --all --force --grace-period=0 2>/dev/null || true
    log "  Workloads cleaned up"

    # ── Step 3: Delete services, configmaps, secrets, PVCs ──
    log "[3/6] Deleting services, configs, secrets, and PVCs..."
    for kind in service configmap secret pvc ingress networkpolicy; do
        kubectl -n "${NAMESPACE}" delete "${kind}" --all --ignore-not-found 2>/dev/null || true
    done
    log "  Namespace resources deleted"

    # ── Step 4: Delete Orchi CRDs ──
    log "[4/6] Deleting Orchi CRDs..."
    local crds
    crds=$(kubectl get crds -o name 2>/dev/null | grep orchi || true)
    if [ -n "${crds}" ]; then
        echo "${crds}" | xargs kubectl delete --ignore-not-found --wait=false
        log "  CRDs deleted: $(echo "${crds}" | wc -l | tr -d ' ') CRD(s)"
    else
        log "  No Orchi CRDs found"
    fi

    # ── Step 5: Uninstall Helm releases ──
    log "[5/6] Uninstalling Helm releases..."
    if command -v helm &>/dev/null; then
        for release_ns in "external-secrets:external-secrets" "kube-prometheus-stack:monitoring" "velero:velero"; do
            local release="${release_ns%%:*}"
            local ns="${release_ns##*:}"
            if helm status "${release}" -n "${ns}" &>/dev/null; then
                helm uninstall "${release}" -n "${ns}" --wait 2>/dev/null || true
                log "  Uninstalled Helm release: ${release} (ns: ${ns})"
            fi
        done
        # Clean up Helm namespaces
        for ns in external-secrets monitoring velero; do
            kubectl delete namespace "${ns}" --ignore-not-found --wait=false 2>/dev/null || true
        done
    else
        log "  helm not found — skipping Helm cleanup"
        log "  Manually run: helm uninstall <release> -n <namespace>"
    fi

    # ── Step 6: Delete namespace ──
    log "[6/6] Deleting namespace ${NAMESPACE}..."
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found --wait=false 2>/dev/null || true
    log "  Namespace ${NAMESPACE} deletion initiated"

    echo ""
    log "============================================"
    log "  REMOVAL COMPLETE"
    log "============================================"
    log "All Orchi resources have been removed."
    log ""
    log "Note: PersistentVolumes may take a moment to be reclaimed."
    log "Check with: kubectl get pv | grep orchi"
    log ""
    log "To verify clean state:"
    log "  kubectl get all -n ${NAMESPACE}"
    log "  kubectl get crds | grep orchi"
    log "  kubectl get ns ${NAMESPACE}"
}

# ==============================================================================
# Step 8: Verify Deployment
# ==============================================================================
verify_deployment() {
    log "Verifying deployment..."

    echo ""
    echo "=== Pods ==="
    kubectl -n "${NAMESPACE}" get pods

    echo ""
    echo "=== Services ==="
    kubectl -n "${NAMESPACE}" get svc

    echo ""
    echo "=== CRDs ==="
    kubectl get crds | grep orchi || true

    echo ""
    log "Deployment to '${ENVIRONMENT}' verified successfully"
}

# ==============================================================================
# Main
# ==============================================================================

ENVIRONMENT=""
PROVIDER="kubeconfig"
DRY_RUN="false"
REMOVE_ALL="false"

while getopts "e:p:drh" opt; do
    case "${opt}" in
        e) ENVIRONMENT="${OPTARG}" ;;
        p) PROVIDER="${OPTARG}" ;;
        d) DRY_RUN="true" ;;
        r) REMOVE_ALL="true" ;;
        h) usage ;;
        *) usage ;;
    esac
done

# Validate environment
if [ -z "${ENVIRONMENT}" ]; then
    error "Environment is required. Use -e prod, -e staging, or -e dev."
fi

case "${ENVIRONMENT}" in
    prod|staging|dev) ;;
    *) error "Invalid environment '${ENVIRONMENT}'. Must be one of: prod, staging, dev." ;;
esac

# Validate provider
case "${PROVIDER}" in
    kubeconfig|aws) ;;
    *) error "Invalid provider '${PROVIDER}'. Must be one of: kubeconfig, aws." ;;
esac

log "=========================================="
log "Orchi Platform Deployment"
log "=========================================="
log "Environment : ${ENVIRONMENT}"
log "Provider    : ${PROVIDER}"
log "Dry run     : ${DRY_RUN}"
log "Remove all  : ${REMOVE_ALL}"
log "Namespace   : ${NAMESPACE}"
log "=========================================="

print_config "${ENVIRONMENT}"

check_prerequisites
configure_cluster
validate_manifests

if [ "${REMOVE_ALL}" = "true" ]; then
    remove_all
elif [ "${DRY_RUN}" = "true" ]; then
    diff_manifests
    log "Dry run complete. No changes were applied."
else
    apply_manifests
    wait_for_rollout
    verify_deployment
fi

log "Done."
