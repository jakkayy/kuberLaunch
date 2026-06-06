# kuberLaunch

Internal Developer Platform (IDP) — กรอกฟอร์มครั้งเดียว ได้ Dockerfile + Helm chart + GitHub Actions CI + ArgoCD GitOps + Grafana dashboard + Vault secrets พร้อมใช้งานทันที

## Architecture

```
Developer → Web UI (Next.js)
              ↓
           API (Go / Gin)
              ↓
    ┌─────────┼──────────┬──────────┐
    │         │          │          │
 GitHub    ArgoCD     Grafana    Vault
 (CI/CD)  (GitOps)  (Monitor)  (Secrets)
              ↓
         Kind (K8s)
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| API | Go, Gin, pgx/v5, goose |
| Web | Next.js (App Router) |
| Database | PostgreSQL 16 |
| Kubernetes | Kind (local) + nginx ingress |
| GitOps | ArgoCD |
| CI | GitHub Actions + GHCR |
| Monitoring | Grafana + kube-prometheus-stack |
| Secrets | HashiCorp Vault (KV v2) |

## Prerequisites

- Docker + Kind
- Go 1.22+
- Node.js 20+
- kubectl + helm
- GitHub account + Personal Access Token (repo + packages scope)

## Local Cluster Setup

```bash
# สร้าง Kind cluster
kind create cluster --name kuberlauncher --config k8s/kind-config.yaml

# ติดตั้ง nginx ingress
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# ติดตั้ง ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
kubectl patch deploy argocd-server -n argocd --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--insecure"}]'

# ดู ArgoCD admin password
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d

# ติดตั้ง kube-prometheus-stack (Grafana)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  -n monitoring --create-namespace \
  --set grafana.adminPassword=admin

# ติดตั้ง Vault (dev mode)
helm repo add hashicorp https://helm.releases.hashicorp.com
helm install vault hashicorp/vault -n vault --create-namespace \
  --set server.dev.enabled=true \
  --set server.dev.devRootToken=root
```

สร้าง ingress สำหรับแต่ละ service:

```bash
# ArgoCD ingress → argocd.localhost:8090
# Grafana ingress → grafana.localhost:8090
# Vault ingress   → vault.localhost:8090
```

> ดูตัวอย่าง ingress manifest ได้ใน `docs/`

เพิ่มบรรทัดนี้ใน `/etc/hosts`:

```
127.0.0.1 argocd.localhost grafana.localhost vault.localhost
```

## Running Locally

### 1. Database

```bash
docker compose up postgres -d
```

### 2. API

```bash
cd api
cp .env.example .env   # แก้ไข GITHUB_TOKEN, GITHUB_OWNER, ARGOCD_PASSWORD
go run cmd/server/main.go
```

### 3. Web

```bash
cd web
npm install
npm run dev
```

เข้าใช้งานที่ `http://localhost:3000`

## Environment Variables

**`api/.env`**

```env
PORT=8080
ENV=development

# PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_USER=kuberlauncher
DB_PASSWORD=kuberlauncher
DB_NAME=kuberlauncher

# GitHub
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
GITHUB_OWNER=your-github-username

# ArgoCD
ARGOCD_URL=http://argocd.localhost:8090
ARGOCD_USERNAME=admin
ARGOCD_PASSWORD=your-argocd-password

# Grafana
GRAFANA_URL=http://grafana.localhost:8090
GRAFANA_USERNAME=admin
GRAFANA_PASSWORD=admin

# Vault
VAULT_URL=http://vault.localhost:8090
VAULT_TOKEN=root
```

## Features

### Project Creation

กรอกแค่ 5 ฟิลด์ — ระบบ generate ไฟล์ DevOps ทั้งหมดอัตโนมัติ:

| Input | Options |
|-------|---------|
| Runtime | `go`, `nextjs`, `nestjs`, `fastapi` |
| Database | `postgres`, `mysql`, `none` |
| Cache | `redis`, `none` |
| Port | port ที่ service รัน |

ไฟล์ที่ได้:
- `Dockerfile` — multi-stage build
- `helm/` — Helm chart พร้อม values
- `.github/workflows/ci.yml` — build + push image ไปยัง GHCR
- `argocd/application.yaml` — ArgoCD Application manifest
- `monitoring/servicemonitor.yaml` — Prometheus ServiceMonitor

### Golden Path (One-Click Setup)

กดปุ่ม **Setup All** — รัน 3 ขั้นตอนต่อเนื่องอัตโนมัติ:

```
1. Connect GitHub   → สร้าง repo + push ไฟล์ทั้งหมด
2. Register ArgoCD  → สร้าง ArgoCD Application
3. Setup Monitoring → สร้าง Grafana folder + dashboard
```

progress แสดงผล realtime ผ่าน Server-Sent Events

### Deploy

กรอก branch แล้วกด **Deploy** — ระบบ:

1. Trigger GitHub Actions workflow
2. CI build + push Docker image ไปยัง GHCR
3. ArgoCD sync + deploy ไปยัง Kind cluster

Status อัปเดต realtime:
```
pending → building → deploying → success / failed
```

### Secrets Management

จัดการ secrets ผ่าน Vault KV v2 ได้บนหน้า project โดยตรง:

- เพิ่ม key-value secret
- ดู key names (ไม่แสดง value บนหน้าจอ)
- ลบ secret key

Secret path: `secret/data/kuberlauncher/<project-slug>`

### Monitoring

Grafana dashboard สำหรับแต่ละ project ประกอบด้วย:

- CPU Usage
- Memory Usage (MB)
- HTTP Requests/sec

scoped ไปที่ namespace `<project-name>-dev`

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/projects` | สร้าง project |
| `GET` | `/api/v1/projects` | list ทั้งหมด |
| `GET` | `/api/v1/projects/:id` | ดูรายละเอียด |
| `DELETE` | `/api/v1/projects/:id` | ลบ |
| `GET` | `/api/v1/projects/:id/download` | ดาวน์โหลดไฟล์ zip |
| `GET` | `/api/v1/projects/:id/preview` | ดูไฟล์ที่ generate |
| `POST` | `/api/v1/projects/:id/repo` | connect GitHub repo |
| `POST` | `/api/v1/projects/:id/repo/repair` | re-push ไฟล์ไปยัง repo |
| `POST` | `/api/v1/projects/:id/argocd` | register ArgoCD app |
| `POST` | `/api/v1/projects/:id/monitoring` | setup Grafana dashboard |
| `GET` | `/api/v1/projects/:id/setup/stream` | SSE golden path progress |
| `POST` | `/api/v1/projects/:id/deployments` | trigger deploy |
| `GET` | `/api/v1/projects/:id/deployments` | list deployments |
| `GET` | `/api/v1/projects/:id/deployments/:dep_id` | ดู deployment |
| `GET` | `/api/v1/projects/:id/deployments/:dep_id/stream` | SSE deploy status |
| `POST` | `/api/v1/projects/:id/secrets` | set secret |
| `GET` | `/api/v1/projects/:id/secrets` | list secret keys |
| `DELETE` | `/api/v1/projects/:id/secrets/:key` | ลบ secret key |

## Service URLs

| Service | URL | Credentials |
|---------|-----|-------------|
| Web UI | `http://localhost:3000` | — |
| API | `http://localhost:8080` | — |
| ArgoCD | `http://argocd.localhost:8090` | admin / (จาก secret) |
| Grafana | `http://grafana.localhost:8090` | admin / admin |
| Vault | `http://vault.localhost:8090` | token: `root` |
| pgAdmin | `http://localhost:5050` | admin@kuberlauncher.dev / admin |
