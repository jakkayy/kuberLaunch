#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="kuberlauncher"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()    { echo -e "${GREEN}[+]${NC} $*"; }
warn()    { echo -e "${YELLOW}[!]${NC} $*"; }
error()   { echo -e "${RED}[x]${NC} $*"; exit 1; }

# --- Check dependencies ---
check_dep() {
  command -v "$1" &>/dev/null || error "$1 not found. Install it first."
}

check_dep kind
check_dep kubectl
check_dep docker

# Install Helm if missing (user-local, no sudo required)
if ! command -v helm &>/dev/null; then
  info "Installing Helm to ~/.local/bin..."
  mkdir -p ~/.local/bin
  curl -fsSL https://get.helm.sh/helm-v3.17.3-linux-amd64.tar.gz | tar xz -C /tmp
  mv /tmp/linux-amd64/helm ~/.local/bin/helm
  export PATH="$HOME/.local/bin:$PATH"
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
fi

# --- Create Kind cluster ---
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  warn "Cluster '${CLUSTER_NAME}' already exists — skipping create"
else
  info "Creating Kind cluster '${CLUSTER_NAME}'..."
  kind create cluster --config "$ROOT_DIR/k8s/kind-config.yaml"
fi

kubectl config use-context "kind-${CLUSTER_NAME}"

# --- Install nginx ingress controller ---
info "Installing nginx ingress controller..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

info "Waiting for ingress controller to be ready (up to 3 min)..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s

# --- Done ---
echo ""
info "Cluster ready!"
echo ""
echo "  Context : kind-${CLUSTER_NAME}"
echo "  HTTP    : http://localhost:8090"
echo "  HTTPS   : https://localhost:8443"
echo ""
echo "  Next: run scripts/test-deploy.sh <project-id> to deploy a generated project"
