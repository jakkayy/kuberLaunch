#!/usr/bin/env bash
# End-to-end test of the kuberLaunch golden path, without external services:
#   1. start Postgres + the API, sign in with a test session token
#   2. create a project through the API and download its generated files
#   3. build the service image with the generated Dockerfile
#   4. helm install the generated chart (dev values) into a Kind cluster
#   5. wait for rollout and call the service, including its provisioned Redis
#
# GitHub, ArgoCD, Vault and the operators (ESO, CNPG, Prometheus) are not
# installed, so the chart features that need them are switched off.
#
# Usage: scripts/e2e.sh            (KEEP=1 keeps the cluster and containers)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER="${CLUSTER:-kuberlaunch-e2e}"
WORK="$(mktemp -d)"
SECRET="e2e-secret-0123456789abcdef0123456789"
API_PORT="${API_PORT:-18181}"
PG_PORT="${PG_PORT:-55433}"
SLUG="e2e-service"

info() { echo -e "\033[0;32m[e2e]\033[0m $*"; }
fail() { echo -e "\033[0;31m[e2e] FAIL:\033[0m $*" >&2; exit 1; }

cleanup() {
  [ -n "${API_PID:-}" ] && kill "$API_PID" 2>/dev/null || true
  if [ "${KEEP:-0}" != "1" ]; then
    docker rm -f kl-e2e-pg >/dev/null 2>&1 || true
    kind delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
    rm -rf "$WORK"
  fi
}
trap cleanup EXIT

for dep in docker kind kubectl helm go python3 curl unzip; do
  command -v "$dep" >/dev/null || fail "$dep not found"
done

# --- session token signed like the web app does (HS256, iss/aud/exp) ---
token() {
  python3 - "$SECRET" <<'PY'
import base64, hashlib, hmac, json, sys, time
b64 = lambda b: base64.urlsafe_b64encode(b).rstrip(b"=").decode()
header = b64(json.dumps({"alg": "HS256", "typ": "JWT"}).encode())
claims = b64(json.dumps({"sub": "4242", "login": "e2e-bot", "name": "e2e",
                         "iss": "kuberlaunch-web", "aud": "kuberlaunch-api",
                         "exp": int(time.time()) + 3600}).encode())
sig = hmac.new(sys.argv[1].encode(), f"{header}.{claims}".encode(), hashlib.sha256).digest()
print(f"{header}.{claims}.{b64(sig)}")
PY
}

info "starting Postgres"
docker rm -f kl-e2e-pg >/dev/null 2>&1 || true
docker run -d --name kl-e2e-pg -e POSTGRES_USER=kuberlauncher -e POSTGRES_PASSWORD=kuberlauncher \
  -e POSTGRES_DB=kuberlauncher -p "$PG_PORT:5432" postgres:16-alpine >/dev/null
for _ in $(seq 30); do docker exec kl-e2e-pg pg_isready -U kuberlauncher >/dev/null 2>&1 && break; sleep 1; done

info "starting API"
(cd "$ROOT/api" && go build -o "$WORK/api" ./cmd/server)
env -i PATH="$PATH" HOME="$HOME" PORT="$API_PORT" DB_PORT="$PG_PORT" ENV=test \
  AUTH_JWT_SECRET="$SECRET" KUBECONFIG=/nonexistent "$WORK/api" >"$WORK/api.log" 2>&1 &
API_PID=$!
API="http://localhost:$API_PORT"
for _ in $(seq 30); do curl -fs "$API/readyz" >/dev/null 2>&1 && break; sleep 1; done
curl -fs "$API/readyz" >/dev/null || { cat "$WORK/api.log"; fail "API not ready"; }

AUTH="Authorization: Bearer $(token)"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$API/api/v1/projects")" = 401 ] || fail "API accepted an unauthenticated request"

info "creating project through the API"
PROJECT_ID=$(curl -fs -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"E2E Service","runtime":"go","database":"none","cache":"redis"}' \
  "$API/api/v1/projects" | python3 -c 'import sys,json; print(json.load(sys.stdin)["project"]["id"])')
[ -n "$PROJECT_ID" ] || fail "no project id"
curl -fs -H "$AUTH" "$API/api/v1/projects/$PROJECT_ID/download" -o "$WORK/files.zip"
mkdir -p "$WORK/app"
unzip -qo "$WORK/files.zip" -d "$WORK/app"
for f in Dockerfile helm/values.yaml helm/values-dev.yaml helm/templates/redis.yaml argocd/application-prod.yaml; do
  [ -f "$WORK/app/$f" ] || fail "generated file missing: $f"
done

info "building the service image from the generated Dockerfile"
cp -r "$ROOT/scripts/e2e/sample-go/." "$WORK/app/"
IMAGE="kuberlaunch-e2e/$SLUG:e2e"
docker build -q -t "$IMAGE" "$WORK/app" >/dev/null

info "creating Kind cluster $CLUSTER"
kind get clusters | grep -qx "$CLUSTER" || kind create cluster --name "$CLUSTER" --wait 120s >/dev/null
kubectl config use-context "kind-$CLUSTER" >/dev/null
kind load docker-image "$IMAGE" --name "$CLUSTER" >/dev/null

info "installing the generated chart (dev)"
NS="$SLUG-dev"
helm upgrade --install "$SLUG" "$WORK/app/helm" -n "$NS" --create-namespace \
  -f "$WORK/app/helm/values-dev.yaml" \
  --set image.repository="kuberlaunch-e2e/$SLUG" --set image.tag=e2e --set image.pullSecret= \
  --set externalSecret.enabled=false --set monitoring.serviceMonitor=false \
  --set ingress.enabled=false --set networkPolicy.enabled=false \
  --wait --timeout 3m >/dev/null || { kubectl -n "$NS" get pods; kubectl -n "$NS" describe pods | tail -40; fail "helm install"; }

info "calling the service"
kubectl -n "$NS" port-forward "svc/$SLUG" 18282:8080 >/dev/null 2>&1 &
PF=$!
sleep 3
BODY=$(curl -fs localhost:18282/ || true)
kill $PF 2>/dev/null || true
echo "$BODY" | grep -q "hello from kuberLaunch e2e" || fail "unexpected response: $BODY"
echo "$BODY" | grep -q "REDIS_URL=redis://$SLUG-redis:6379" || fail "REDIS_URL not injected: $BODY"
kubectl -n "$NS" rollout status "deploy/$SLUG-redis" --timeout 60s >/dev/null || fail "redis not ready"

info "checking the audit trail"
curl -fs -H "$AUTH" "$API/api/v1/projects/$PROJECT_ID/audit" | grep -q '"project.create"' || fail "project.create not audited"

info "PASS"
