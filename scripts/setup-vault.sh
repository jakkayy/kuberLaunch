#!/usr/bin/env bash
# Configure Vault for kuberLaunch:
#   - KV v2 at secret/
#   - Kubernetes auth so pods log in with their service account
#   - a scoped policy + role for the kuberLaunch API itself
# Per-project policies/roles are created by the API (vault.EnsureProjectAccess).
#
# Usage: VAULT_ADDR=http://vault.localhost:8090 VAULT_TOKEN=<admin token> ./scripts/setup-vault.sh
set -euo pipefail

: "${VAULT_ADDR:?set VAULT_ADDR}"
: "${VAULT_TOKEN:?set VAULT_TOKEN (admin token, only needed for this setup)}"
API_NAMESPACE="${API_NAMESPACE:-kuberlaunch}"
API_SERVICE_ACCOUNT="${API_SERVICE_ACCOUNT:-kuberlaunch-api}"

command -v vault >/dev/null || { echo "vault CLI not found"; exit 1; }

vault secrets list -format=json | grep -q '"secret/"' || vault secrets enable -path=secret kv-v2
vault auth list -format=json | grep -q '"kubernetes/"' || vault auth enable kubernetes

# Vault runs in-cluster, so it can use its own service account to review tokens.
vault write auth/kubernetes/config kubernetes_host="https://kubernetes.default.svc:443"

vault policy write kuberlaunch-api - <<'POLICY'
# Manage project secrets.
path "secret/data/kuberlauncher/*"     { capabilities = ["create", "read", "update", "delete"] }
path "secret/metadata/kuberlauncher/*" { capabilities = ["read", "list", "delete"] }

# Create read-only policies and Kubernetes roles for each project.
path "sys/policies/acl/kuberlaunch-*" {
  capabilities = ["create", "read", "update", "delete"]
}
path "auth/kubernetes/role/kuberlaunch-*" {
  capabilities = ["create", "read", "update", "delete"]
}
POLICY

# Note: the API may write kuberlaunch-* policies, so a project named "api"
# would collide with this role. The API rejects that slug (see reserved slugs).
vault write auth/kubernetes/role/kuberlaunch-api \
  bound_service_account_names="$API_SERVICE_ACCOUNT" \
  bound_service_account_namespaces="$API_NAMESPACE" \
  token_policies=kuberlaunch-api \
  token_ttl=1h

# Read-only access to the platform's own secrets for External Secrets
# (k8s/platform/kuberlaunch-secrets.yaml).
vault policy write kuberlaunch-platform-secrets - <<'POLICY'
path "secret/data/platform/kuberlaunch" { capabilities = ["read"] }
POLICY
vault write auth/kubernetes/role/kuberlaunch-platform-secrets \
  bound_service_account_names=kuberlaunch-secrets \
  bound_service_account_namespaces="$API_NAMESPACE" \
  token_policies=kuberlaunch-platform-secrets \
  token_ttl=1h

echo "Vault configured. Run the API with:"
echo "  VAULT_AUTH_METHOD=kubernetes VAULT_K8S_ROLE=kuberlaunch-api"
