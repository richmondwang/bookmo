#!/usr/bin/env bash
# scripts/cluster.sh — manage the local minikube dev cluster for Kadto
#
# Usage:
#   ./scripts/cluster.sh setup    # start minikube, create namespace, launch skaffold dev
#   ./scripts/cluster.sh teardown # stop skaffold, delete namespace, delete minikube cluster
#   ./scripts/cluster.sh status   # show cluster, pod, and port-forward status
#   ./scripts/cluster.sh logs     # tail logs from all Kadto pods
#
# Prerequisites: minikube, docker, kubectl, skaffold

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

MINIKUBE_PROFILE="kadto"
MINIKUBE_DRIVER="docker"
MINIKUBE_CPUS="4"
MINIKUBE_MEMORY="4096"        # MB — needs room for Postgres, Redis, and two Go toolchain images
MINIKUBE_KUBERNETES_VERSION="stable"
NAMESPACE="kadto-local"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()    { echo -e "${CYAN}[cluster]${NC} $*"; }
success() { echo -e "${GREEN}[cluster]${NC} $*"; }
warn()    { echo -e "${YELLOW}[cluster]${NC} $*"; }
die()     { echo -e "${RED}[cluster] ERROR:${NC} $*" >&2; exit 1; }

# ── Platform detection ────────────────────────────────────────────────────────

is_native_linux() {
  [[ "$(uname -s)" == "Linux" ]] && ! grep -qi "microsoft" /proc/version 2>/dev/null
}

# ── Dependency check ──────────────────────────────────────────────────────────

check_deps() {
  local missing=()
  for cmd in minikube docker kubectl skaffold; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
  done
  [[ ${#missing[@]} -eq 0 ]] || die "Missing required tools: ${missing[*]}"
}

# ── Docker service management ─────────────────────────────────────────────────
# Only manages Docker on native Linux. On WSL2 and macOS, Docker Desktop
# is self-managing — attempting systemctl on those platforms would fail.

start_docker() {
  if docker info &>/dev/null; then
    info "Docker is already running."
    return
  fi
  if ! is_native_linux; then
    die "Docker is not running. Start Docker Desktop and try again."
  fi
  info "Starting Docker service..."
  systemctl --user start docker.service
  local attempts=0
  until docker info &>/dev/null; do
    (( attempts++ ))
    [[ $attempts -lt 15 ]] || die "Docker did not become ready after ${attempts} attempts."
    info "Waiting for Docker daemon... (${attempts}/15)"
    sleep 2
  done
  success "Docker is running."
}

# ── Minikube helpers ──────────────────────────────────────────────────────────

minikube_is_running() {
  minikube status -p "$MINIKUBE_PROFILE" --format='{{.Host}}' 2>/dev/null | grep -q "Running"
}

# ── SETUP ─────────────────────────────────────────────────────────────────────

cmd_setup() {
  check_deps

  # 1. Ensure Docker is running
  start_docker

  # 2. Start (or reuse) minikube cluster
  if minikube_is_running; then
    info "Minikube profile '${MINIKUBE_PROFILE}' is already running."
  else
    info "Starting minikube..."
    info "  profile  = ${MINIKUBE_PROFILE}"
    info "  driver   = ${MINIKUBE_DRIVER}"
    info "  cpus     = ${MINIKUBE_CPUS}"
    info "  memory   = ${MINIKUBE_MEMORY}MB"
    minikube start \
      -p "$MINIKUBE_PROFILE" \
      --driver="$MINIKUBE_DRIVER" \
      --cpus="$MINIKUBE_CPUS" \
      --memory="$MINIKUBE_MEMORY" \
      --kubernetes-version="$MINIKUBE_KUBERNETES_VERSION"
    success "Minikube started."
  fi

  # 3. Point Docker CLI at minikube's daemon so Skaffold can push images
  #    directly into the cluster without a registry.
  info "Configuring Docker environment for minikube..."
  eval "$(minikube docker-env -p "$MINIKUBE_PROFILE")"

  # 4. Switch kubectl context to this cluster
  info "Switching kubectl context to '${MINIKUBE_PROFILE}'..."
  kubectl config use-context "$MINIKUBE_PROFILE"

  # 5. Create namespace (idempotent)
  info "Ensuring namespace '${NAMESPACE}' exists..."
  kubectl create namespace "$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

  # 6. Launch skaffold dev (exec replaces this shell; Ctrl-C stops everything)
  success "Cluster ready. Launching skaffold dev..."
  echo ""
  info "  API      → http://localhost:8080"
  info "  Postgres → localhost:5432   (user: kadto  db: booking_platform)"
  info "  Redis    → localhost:6379"
  echo ""
  info "Migrations run automatically via init container before the API pod starts."
  info "Press Ctrl-C to stop. Run './scripts/cluster.sh teardown' to destroy."
  echo ""

  cd "$REPO_ROOT"
  exec skaffold dev \
    --kube-context="$MINIKUBE_PROFILE" \
    --namespace="$NAMESPACE" \
    --status-check
}

# ── TEARDOWN ──────────────────────────────────────────────────────────────────

cmd_teardown() {
  check_deps

  if minikube_is_running; then
    # Point Docker CLI at minikube's daemon so skaffold can reach the cluster
    eval "$(minikube docker-env -p "$MINIKUBE_PROFILE" 2>/dev/null || true)"

    # 1. Let Skaffold clean up its own resources (Deployments, Services, Jobs, etc.)
    info "Running skaffold delete..."
    cd "$REPO_ROOT"
    skaffold delete \
      --kube-context="$MINIKUBE_PROFILE" \
      --namespace="$NAMESPACE" 2>/dev/null || true

    # 2. Delete the namespace — catches anything Skaffold missed
    info "Deleting namespace '${NAMESPACE}'..."
    kubectl delete namespace "$NAMESPACE" \
      --ignore-not-found=true \
      --context="$MINIKUBE_PROFILE" || true
  fi

  # 3. Delete the minikube cluster entirely
  if minikube status -p "$MINIKUBE_PROFILE" 2>/dev/null | grep -q "host:"; then
    info "Deleting minikube profile '${MINIKUBE_PROFILE}'..."
    minikube delete -p "$MINIKUBE_PROFILE"
    success "Minikube cluster deleted."
  else
    info "Minikube profile '${MINIKUBE_PROFILE}' does not exist or is already stopped."
  fi

  success "Teardown complete."
}

# ── STATUS ────────────────────────────────────────────────────────────────────

cmd_status() {
  check_deps
  echo ""

  info "=== Minikube (profile: ${MINIKUBE_PROFILE}) ==="
  if minikube_is_running; then
    minikube status -p "$MINIKUBE_PROFILE"
    echo ""

    info "=== Pods (${NAMESPACE}) ==="
    kubectl get pods -n "$NAMESPACE" \
      --context="$MINIKUBE_PROFILE" \
      -o wide 2>/dev/null \
      || warn "Namespace '${NAMESPACE}' not found — run: ./scripts/cluster.sh setup"
    echo ""

    info "=== Services (${NAMESPACE}) ==="
    kubectl get services -n "$NAMESPACE" \
      --context="$MINIKUBE_PROFILE" 2>/dev/null || true
    echo ""

    info "=== Port Forwards (when skaffold dev is running) ==="
    echo "  API      → http://localhost:8080"
    echo "  Postgres → localhost:5432"
    echo "  Redis    → localhost:6379"
  else
    warn "Minikube profile '${MINIKUBE_PROFILE}' is not running."
    echo "  Run: ./scripts/cluster.sh setup"
  fi
  echo ""
}

# ── LOGS ──────────────────────────────────────────────────────────────────────

cmd_logs() {
  check_deps
  if ! minikube_is_running; then
    die "Minikube is not running. Run: ./scripts/cluster.sh setup"
  fi
  info "Tailing logs from all Kadto pods in '${NAMESPACE}' (Ctrl-C to stop)..."
  kubectl logs \
    --context="$MINIKUBE_PROFILE" \
    -n "$NAMESPACE" \
    -l app.kubernetes.io/part-of=kadto \
    -f \
    --max-log-requests=10 \
    --prefix=true
}

# ── Entrypoint ────────────────────────────────────────────────────────────────

case "${1:-}" in
  setup)    cmd_setup    ;;
  teardown) cmd_teardown ;;
  status)   cmd_status   ;;
  logs)     cmd_logs     ;;
  *)
    echo "Usage: $0 {setup|teardown|status|logs}"
    echo ""
    echo "  setup    — start minikube, create namespace, launch skaffold dev"
    echo "  teardown — delete k8s resources, namespace, and minikube cluster"
    echo "  status   — show cluster, pod, and service status"
    echo "  logs     — tail logs from all Kadto pods"
    exit 1
    ;;
esac
