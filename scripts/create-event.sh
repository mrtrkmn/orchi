#!/usr/bin/env bash
# ==============================================================================
# Orchi Platform — Create Event Script
# ==============================================================================
# Creates an Event custom resource on the cluster.
# Generates the Event CR YAML from flags and applies it via kubectl.
#
# Usage:
#   ./scripts/create-event.sh -t <tag> -n <name> -c <capacity> -x <exercises> [options]
#
# Examples:
#   ./scripts/create-event.sh -t ctf-2026 -n "Spring CTF 2026" -c 30 -x sql-injection,xss-basic
#   ./scripts/create-event.sh -t workshop-1 -n "Security Workshop" -c 10 -x buffer-overflow -p aws
#   ./scripts/create-event.sh -t ctf-2026 -n "CTF" -c 50 -x sql-injection --vpn --frontend kali:latest
#   ./scripts/create-event.sh -t ctf-2026 --delete                # Delete an existing event
#   ./scripts/create-event.sh --list                               # List all events
# ==============================================================================

set -euo pipefail

# ==============================================================================
# Defaults
# ==============================================================================
EVENT_TAG=""
EVENT_NAME=""
CAPACITY=""
EXERCISES=""
PROVIDER="kubeconfig"
CREATED_BY="${USER:-operator}"
VPN_REQUIRED="false"
FRONTEND_IMAGE=""
FRONTEND_MEMORY=4096
FRONTEND_CPU=2
START_TIME=""
END_TIME=""
DRY_RUN="false"
DELETE_MODE="false"
LIST_MODE="false"

AWS_REGION="${AWS_REGION:-eu-north-1}"
EKS_CLUSTER_NAME="${EKS_CLUSTER_NAME:-orchi-cluster}"

# ==============================================================================
# Functions
# ==============================================================================

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $*" >&2
    exit 1
}

usage() {
    cat <<'EOF'
Usage: create-event.sh -t <tag> -n <name> -c <capacity> -x <exercises> [options]

Create an Orchi Event custom resource on the Kubernetes cluster.

Required:
  -t, --tag <tag>           Event tag (unique ID, lowercase alphanumeric + hyphens, 2-63 chars)
  -n, --name <name>         Human-readable event name
  -c, --capacity <number>   Maximum number of teams
  -x, --exercises <list>    Comma-separated exercise tags (e.g. sql-injection,xss-basic)

Optional:
  -p, --provider <provider> Cluster provider: kubeconfig or aws (default: kubeconfig)
  -b, --created-by <user>   Creator username (default: $USER)
      --vpn                 Require VPN for participants
      --frontend <image>    Frontend VM image (e.g. ghcr.io/mrtrkmn/orchi/frontends/kali:latest)
      --frontend-memory <MB> Frontend VM memory in MB (default: 4096)
      --frontend-cpu <cores> Frontend VM CPU cores (default: 2)
      --start <datetime>    Start time in ISO 8601 (e.g. 2026-03-01T09:00:00Z)
      --end <datetime>      End time in ISO 8601 (e.g. 2026-03-01T18:00:00Z)
  -d, --dry-run             Print the generated YAML without applying
      --delete              Delete the event specified by -t <tag>
      --list                List all existing events
  -h, --help                Show this help message

Environment variables:
  AWS_REGION          AWS region (default: eu-north-1)
  EKS_CLUSTER_NAME    EKS cluster name (default: orchi-cluster)

Examples:
  # Create a basic event
  create-event.sh -t ctf-2026 -n "Spring CTF 2026" -c 30 -x sql-injection,xss-basic

  # Create event on AWS EKS with VPN and frontend
  create-event.sh -t workshop-1 -n "Security Workshop" -c 10 \
    -x buffer-overflow,format-string -p aws --vpn \
    --frontend ghcr.io/mrtrkmn/orchi/frontends/kali:latest

  # Dry run — show YAML without applying
  create-event.sh -t ctf-2026 -n "CTF" -c 50 -x sql-injection --dry-run

  # Delete an event
  create-event.sh -t ctf-2026 --delete

  # List all events
  create-event.sh --list
EOF
    exit 0
}

# ==============================================================================
# Parse Arguments
# ==============================================================================
parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            -t|--tag)        EVENT_TAG="$2"; shift 2 ;;
            -n|--name)       EVENT_NAME="$2"; shift 2 ;;
            -c|--capacity)   CAPACITY="$2"; shift 2 ;;
            -x|--exercises)  EXERCISES="$2"; shift 2 ;;
            -p|--provider)   PROVIDER="$2"; shift 2 ;;
            -b|--created-by) CREATED_BY="$2"; shift 2 ;;
            --vpn)           VPN_REQUIRED="true"; shift ;;
            --frontend)      FRONTEND_IMAGE="$2"; shift 2 ;;
            --frontend-memory) FRONTEND_MEMORY="$2"; shift 2 ;;
            --frontend-cpu)  FRONTEND_CPU="$2"; shift 2 ;;
            --start)         START_TIME="$2"; shift 2 ;;
            --end)           END_TIME="$2"; shift 2 ;;
            -d|--dry-run)    DRY_RUN="true"; shift ;;
            --delete)        DELETE_MODE="true"; shift ;;
            --list)          LIST_MODE="true"; shift ;;
            -h|--help)       usage ;;
            *)               error "Unknown option: $1. Use --help for usage." ;;
        esac
    done
}

# ==============================================================================
# Validate Inputs
# ==============================================================================
validate_inputs() {
    if [ "${LIST_MODE}" = "true" ]; then
        return
    fi

    if [ -z "${EVENT_TAG}" ]; then
        error "Event tag is required (-t <tag>). Use --help for usage."
    fi

    # Validate tag format
    if ! echo "${EVENT_TAG}" | grep -qE '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'; then
        error "Event tag must be lowercase alphanumeric with optional hyphens (e.g. ctf-2026)"
    fi
    if [ ${#EVENT_TAG} -lt 2 ] || [ ${#EVENT_TAG} -gt 63 ]; then
        error "Event tag must be between 2 and 63 characters"
    fi

    if [ "${DELETE_MODE}" = "true" ]; then
        return
    fi

    if [ -z "${EVENT_NAME}" ]; then
        error "Event name is required (-n <name>). Use --help for usage."
    fi
    if [ -z "${CAPACITY}" ]; then
        error "Capacity is required (-c <number>). Use --help for usage."
    fi
    if ! echo "${CAPACITY}" | grep -qE '^[0-9]+$' || [ "${CAPACITY}" -lt 1 ]; then
        error "Capacity must be a positive integer"
    fi
    if [ -z "${EXERCISES}" ]; then
        error "Exercises are required (-x <list>). Use --help for usage."
    fi
}

# ==============================================================================
# Configure Cluster Access
# ==============================================================================
configure_cluster() {
    log "Configuring cluster access (provider: ${PROVIDER})..."

    case "${PROVIDER}" in
        kubeconfig)
            if [ -n "${KUBECONFIG:-}" ]; then
                log "Using KUBECONFIG=${KUBECONFIG}"
            elif [ -f "${HOME}/.kube/config" ]; then
                log "Using default kubeconfig"
            else
                error "No kubeconfig found. Set KUBECONFIG or use -p aws."
            fi
            ;;
        aws)
            if ! command -v aws &>/dev/null; then
                error "AWS CLI is not installed."
            fi
            aws eks update-kubeconfig \
                --name "${EKS_CLUSTER_NAME}" \
                --region "${AWS_REGION}" \
                --quiet
            ;;
        *)
            error "Unknown provider: ${PROVIDER}. Use 'kubeconfig' or 'aws'."
            ;;
    esac

    if ! kubectl cluster-info &>/dev/null; then
        error "Cannot connect to Kubernetes cluster."
    fi
    log "Cluster access OK"
}

# ==============================================================================
# List Events
# ==============================================================================
list_events() {
    log "Listing all Orchi events..."
    echo ""

    if ! kubectl get events.orchi.cyberorch.com &>/dev/null; then
        log "Event CRD not found. Is the platform deployed?"
        exit 1
    fi

    kubectl get events.orchi.cyberorch.com -o wide 2>/dev/null || echo "No events found."
    echo ""

    # Show team counts per event
    local events
    events=$(kubectl get events.orchi.cyberorch.com -o jsonpath='{range .items[*]}{.spec.tag}{"\t"}{.spec.name}{"\t"}{.status.phase}{"\t"}{.status.teamCount}/{.spec.capacity}{"\n"}{end}' 2>/dev/null || true)

    if [ -n "${events}" ]; then
        echo "Event Summary:"
        printf "  %-20s %-30s %-12s %s\n" "TAG" "NAME" "PHASE" "TEAMS"
        printf "  %-20s %-30s %-12s %s\n" "---" "----" "-----" "-----"
        while IFS=$'\t' read -r tag name phase teams; do
            printf "  %-20s %-30s %-12s %s\n" "${tag}" "${name}" "${phase:-Pending}" "${teams}"
        done <<< "${events}"
    fi
}

# ==============================================================================
# Delete Event
# ==============================================================================
delete_event() {
    log "Deleting event '${EVENT_TAG}'..."

    if ! kubectl get events.orchi.cyberorch.com/"${EVENT_TAG}" &>/dev/null; then
        error "Event '${EVENT_TAG}' not found."
    fi

    echo ""
    kubectl get events.orchi.cyberorch.com/"${EVENT_TAG}" -o wide
    echo ""

    read -r -p "Delete event '${EVENT_TAG}' and all associated resources? (yes/no): " confirm
    if [ "${confirm}" != "yes" ]; then
        log "Aborted."
        exit 0
    fi

    kubectl delete events.orchi.cyberorch.com/"${EVENT_TAG}" --wait=true
    log "Event '${EVENT_TAG}' deleted."

    # Clean up lab namespace if it exists
    local lab_ns="orchi-lab-${EVENT_TAG}"
    if kubectl get namespace "${lab_ns}" &>/dev/null; then
        log "Cleaning up lab namespace '${lab_ns}'..."
        kubectl delete namespace "${lab_ns}" --wait=false
        log "Lab namespace deletion initiated."
    fi
}

# ==============================================================================
# Generate Event CR YAML
# ==============================================================================
generate_event_yaml() {
    local tmpfile
    tmpfile=$(mktemp /tmp/orchi-event-XXXXXXXX).yaml

    # Build exercises list
    local exercises_yaml=""
    IFS=',' read -ra TAGS <<< "${EXERCISES}"
    for tag in "${TAGS[@]}"; do
        tag=$(echo "${tag}" | xargs)  # trim whitespace
        exercises_yaml="${exercises_yaml}      - ${tag}"$'\n'
    done

    # Start building the YAML
    {
        echo "apiVersion: orchi.cyberorch.com/v1alpha1"
        echo "kind: Event"
        echo "metadata:"
        echo "  name: ${EVENT_TAG}"
        echo "  labels:"
        echo "    app.kubernetes.io/managed-by: orchi-create-event"
        echo "    app.kubernetes.io/part-of: orchi"
        echo "    orchi.cyberorch.com/created-by: ${CREATED_BY}"
        echo "spec:"
        echo "  tag: ${EVENT_TAG}"
        echo "  name: \"${EVENT_NAME}\""
        echo "  capacity: ${CAPACITY}"
        echo "  createdBy: ${CREATED_BY}"

        # Add optional start/end times
        if [ -n "${START_TIME}" ]; then
            echo "  startedAt: \"${START_TIME}\""
        fi
        if [ -n "${END_TIME}" ]; then
            echo "  finishExpected: \"${END_TIME}\""
        fi

        # Add VPN config if enabled
        if [ "${VPN_REQUIRED}" = "true" ]; then
            echo "  vpn:"
            echo "    required: true"
        fi

        # Add lab spec
        echo "  lab:"
        echo "    exercises:"
        printf '%s' "${exercises_yaml}"

        # Add frontend if specified
        if [ -n "${FRONTEND_IMAGE}" ]; then
            echo "    frontends:"
            echo "      - image: ${FRONTEND_IMAGE}"
            echo "        memoryMB: ${FRONTEND_MEMORY}"
            echo "        cpu: ${FRONTEND_CPU}"
        fi
    } > "${tmpfile}"

    echo "${tmpfile}"
}

# ==============================================================================
# Create Event
# ==============================================================================
create_event() {
    log "Creating event '${EVENT_TAG}'..."
    echo ""
    log "  Tag:        ${EVENT_TAG}"
    log "  Name:       ${EVENT_NAME}"
    log "  Capacity:   ${CAPACITY}"
    log "  Exercises:  ${EXERCISES}"
    log "  VPN:        ${VPN_REQUIRED}"
    log "  Created by: ${CREATED_BY}"
    [ -n "${START_TIME}" ] && log "  Start:      ${START_TIME}"
    [ -n "${END_TIME}" ]   && log "  End:        ${END_TIME}"
    [ -n "${FRONTEND_IMAGE}" ] && log "  Frontend:   ${FRONTEND_IMAGE} (${FRONTEND_MEMORY}MB, ${FRONTEND_CPU} CPU)"
    echo ""

    # Generate YAML
    local yaml_file
    yaml_file=$(generate_event_yaml)

    echo "=== Generated Event CR ==="
    cat "${yaml_file}"
    echo "==========================="
    echo ""

    if [ "${DRY_RUN}" = "true" ]; then
        log "Dry run — YAML printed above, no changes applied."
        log "YAML saved to: ${yaml_file}"
        return
    fi

    # Check if event already exists
    if kubectl get events.orchi.cyberorch.com/"${EVENT_TAG}" &>/dev/null; then
        echo ""
        read -r -p "Event '${EVENT_TAG}' already exists. Update it? (yes/no): " confirm
        if [ "${confirm}" != "yes" ]; then
            log "Aborted."
            rm -f "${yaml_file}"
            exit 0
        fi
    fi

    # Apply
    kubectl apply -f "${yaml_file}"
    log "Event CR applied."

    # Wait for event to be ready
    log "Waiting for event to reach Running phase (timeout: ~3 min)..."
    local phase=""
    for i in $(seq 1 18); do
        phase=$(kubectl get events.orchi.cyberorch.com/"${EVENT_TAG}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Pending")
        echo "  Attempt ${i}/18: phase=${phase}"

        if [ "${phase}" = "Running" ]; then
            echo ""
            log "Event '${EVENT_TAG}' is running!"
            break
        fi
        if [ "${phase}" = "Failed" ]; then
            echo ""
            log "Event failed to start. Details:"
            kubectl get events.orchi.cyberorch.com/"${EVENT_TAG}" -o yaml
            rm -f "${yaml_file}"
            exit 1
        fi
        sleep 10
    done

    if [ "${phase}" != "Running" ]; then
        log "Event has not reached Running phase yet (current: ${phase})."
        log "It may still be initializing. Check with:"
        log "  kubectl get events.orchi.cyberorch.com/${EVENT_TAG} -o wide"
    fi

    # Print summary
    echo ""
    echo "=== Event ==="
    kubectl get events.orchi.cyberorch.com/"${EVENT_TAG}" -o wide 2>/dev/null || true
    echo ""
    echo "=== Labs ==="
    kubectl get labs.orchi.cyberorch.com -l "orchi.cyberorch.com/event=${EVENT_TAG}" 2>/dev/null || echo "No labs yet"
    echo ""
    echo "=== Teams ==="
    kubectl get teams.orchi.cyberorch.com -l "orchi.cyberorch.com/event=${EVENT_TAG}" 2>/dev/null || echo "No teams yet"
    echo ""

    rm -f "${yaml_file}"
    log "Done."
}

# ==============================================================================
# Main
# ==============================================================================
parse_args "$@"
validate_inputs
configure_cluster

if [ "${LIST_MODE}" = "true" ]; then
    list_events
elif [ "${DELETE_MODE}" = "true" ]; then
    delete_event
else
    create_event
fi
