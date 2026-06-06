#!/usr/bin/env bash
# Test deploy a generated project onto the local Kind cluster.
# Usage: ./scripts/test-deploy.sh <project-id> [namespace]
set -euo pipefail

PROJECT_ID="${1:?Usage: $0 <project-id>}"
API_URL="${API_URL:-http://localhost:8080}"
WORK_DIR="/tmp/kuberlauncher-deploy-${PROJECT_ID}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'
info() { echo -e "${GREEN}[+]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }

# --- Check tools ---
command -v kubectl &>/dev/null || { echo "kubectl not found"; exit 1; }
command -v helm    &>/dev/null || { echo "helm not found — run scripts/setup-kind.sh first"; exit 1; }

# --- Fetch project slug from API ---
info "Fetching project info..."
PROJECT_JSON=$(curl -fsSL "${API_URL}/api/v1/projects/${PROJECT_ID}")
SLUG=$(echo "$PROJECT_JSON" | grep -o '"slug":"[^"]*"' | cut -d'"' -f4)
NAMESPACE="${SLUG}-dev"

info "Project: ${SLUG} → namespace: ${NAMESPACE}"

# --- Download generated templates ---
info "Downloading templates..."
mkdir -p "$WORK_DIR"
curl -fsSL "${API_URL}/api/v1/projects/${PROJECT_ID}/download" -o "${WORK_DIR}/templates.zip"
cd "$WORK_DIR" && unzip -qo templates.zip

# --- Switch to Kind cluster ---
kubectl config use-context "kind-kuberlauncher" 2>/dev/null || \
  warn "Could not switch context — using current context"

# --- Create namespace ---
info "Creating namespace '${NAMESPACE}'..."
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

# --- Helm install ---
info "Deploying '${SLUG}' into namespace '${NAMESPACE}'..."
helm upgrade --install "$SLUG" ./helm \
  --namespace "$NAMESPACE" \
  --set ingress.host="${SLUG}.local" \
  --wait --timeout 2m

# --- Show status ---
echo ""
info "Deploy complete!"
kubectl get pods -n "$NAMESPACE"
echo ""
echo "  Add to /etc/hosts:  127.0.0.1  ${PROJECT_ID}.local"
echo "  Access via:         http://${PROJECT_ID}.local:8090"
echo ""
echo "  Cleanup: helm uninstall ${SLUG} -n ${NAMESPACE}"
