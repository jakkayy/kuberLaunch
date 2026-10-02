# kuberLaunch

Internal Developer Platform (IDP): กรอกฟอร์มครั้งเดียวแล้วได้ service ที่พร้อมใช้งานครบชุด ได้แก่ Dockerfile, Helm chart, GitHub Actions CI,
ArgoCD GitOps สำหรับ **dev / staging / prod**, Postgres/Redis, Grafana dashboard และ secrets จาก Vault

## Architecture

```
Developer ── GitHub OAuth ──▶ Web (Next.js) ──session JWT──▶ API (Go / Gin) ──▶ PostgreSQL
                                                              │
                     ┌──────────────┬───────────┬─────────────┼──────────────┐
                  GitHub         ArgoCD      Grafana        Vault        Kubernetes
               (repo, CI,     (AppProject +  (dashboard)  (secrets,     (namespaces,
                webhooks)      app per env)                k8s auth)     pull secrets, logs)
```

- ผู้ใช้ทุกคน login ด้วย GitHub และมีสิทธิ์ราย project เป็น `viewer` / `maintainer` / `owner` ซึ่งให้ได้ทั้งรายคนและผ่าน team
- สถานะ deployment ขับเคลื่อนด้วย webhook จาก GitHub (`workflow_run`) และ ArgoCD notifications โดยมี server-side poller เป็น fallback
- ทุก request ที่เปลี่ยนแปลงข้อมูลจะถูกบันทึกลง audit log

## Tech Stack

| Layer | Technology |
|-------|-----------|
| API | Go, Gin, pgx/v5, goose, client-go, slog, Prometheus |
| Web | Next.js 16 (App Router, `proxy.ts`), jose |
| Database | PostgreSQL 16 (production: CloudNativePG, HA + S3 backup) |
| GitOps | ArgoCD (AppProject ต่อ service, Application ต่อ environment) |
| CI | GitHub Actions + GHCR |
| Secrets | Vault KV v2 + External Secrets Operator (Kubernetes auth) |
| Monitoring | kube-prometheus-stack (Grafana + ServiceMonitor) |

## Running Locally

### 1. Database และ API

```bash
docker compose up postgres -d
cd api
cp .env.example .env      # อย่างน้อยต้องตั้ง AUTH_JWT_SECRET (≥ 32 ตัวอักษร)
go run ./cmd/server
```

### 2. Web

```bash
cd web
cp .env.example .env.local   # AUTH_JWT_SECRET ต้องตรงกับฝั่ง API
npm install
npm run dev
```

เปิด `http://localhost:3000` ได้เลย ถ้ายังไม่ได้สร้าง GitHub OAuth app ให้ตั้ง `AUTH_DEV_LOGIN=<github-login>`
ใน `web/.env.local` เพื่อ login โดยข้าม GitHub (ใช้ได้เฉพาะตอน dev)

ถ้าจะให้ตัวเองเห็นทุก project ให้ใส่ login ของตัวเองใน `PLATFORM_ADMINS` ของ `api/.env`

### 3. Integration (optional)

ถ้าไม่ได้ตั้งค่า integration ไหน ระบบจะปิด integration นั้นไปเอง โดยดูจากตัวแปรเหล่านี้:

| Integration | ตัวแปรหลัก | หมายเหตุ |
|-------------|-----------|----------|
| GitHub | `GITHUB_OWNER` + `GITHUB_APP_*` หรือ `GITHUB_TOKEN` | แนะนำให้ใช้ GitHub App |
| ArgoCD | `ARGOCD_URL` + `ARGOCD_TOKEN` | สร้าง account ตาม `k8s/platform/argocd-account.yaml` |
| Vault | `VAULT_URL` + `VAULT_AUTH_METHOD` | local ใช้ `token`, in-cluster ใช้ `kubernetes` |
| Grafana | `GRAFANA_URL` + `GRAFANA_PASSWORD` | |
| Webhooks | `PLATFORM_PUBLIC_URL` + `GITHUB_WEBHOOK_SECRET` / `ARGOCD_WEBHOOK_TOKEN` | ถ้าไม่ตั้งจะใช้ polling แทน |
| Pull secret | `REGISTRY_PULL_USERNAME` + `REGISTRY_PULL_TOKEN` | token สิทธิ์ `read:packages` |

รายการตัวแปรทั้งหมดอยู่ใน [api/.env.example](api/.env.example) และ [web/.env.example](web/.env.example)
secret ทุกตัวอ่านจากไฟล์ได้โดยตั้ง `<NAME>_FILE` เช่น `DB_PASSWORD_FILE`

## Local Cluster (Kind)

```bash
scripts/setup-kind.sh                     # Kind + nginx ingress

# ArgoCD
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Operators ที่ chart ที่ generate ต้องใช้
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm install cnpg cnpg/cloudnative-pg -n cnpg-system --create-namespace
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets -n external-secrets --create-namespace
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack -n monitoring --create-namespace

# Vault (dev mode) + Kubernetes auth + policy ของ API
helm repo add hashicorp https://helm.releases.hashicorp.com
helm install vault hashicorp/vault -n vault --create-namespace \
  --set server.dev.enabled=true --set server.dev.devRootToken=root
VAULT_ADDR=http://vault.localhost:8090 VAULT_TOKEN=root scripts/setup-vault.sh
```

## Features

### Project Creation

เลือก runtime (`go`, `nextjs`, `nestjs`, `fastapi`), database (`postgres`, `mysql`, `none`) และ cache (`redis`, `none`)
ชื่อ project จะถูกแปลงเป็น slug (DNS-1035, ยาวไม่เกิน 30 ตัวอักษร, ห้ามใช้ชื่อที่สงวนไว้) แล้ว slug นี้ถูกใช้เป็นชื่อ repo และ namespace

ไฟล์ที่ generate:

| ไฟล์ | หน้าที่ |
|------|--------|
| `Dockerfile` | multi-stage build |
| `.github/workflows/ci.yml` | build + push GHCR แล้วอัปเดต image tag ใน `values-dev.yaml` (ไม่ต้องใช้ cluster credentials) |
| `helm/values.yaml`, `helm/values-{dev,staging,prod}.yaml` | ค่ากลางและค่าราย environment |
| `helm/templates/*` | Deployment, Service, Ingress, HPA, PDB, NetworkPolicy, ServiceMonitor, Vault SecretStore/ExternalSecret, Postgres (CNPG), Redis |
| `argocd/appproject.yaml`, `argocd/application-{env}.yaml` | GitOps manifests |

Postgres และ Redis ถูก provision ให้ราย environment แล้ว inject เข้าไปเป็น `DATABASE_URL` / `REDIS_URL` ส่วน MySQL ให้ใส่ `DATABASE_URL` ใน Vault เอง
เหตุผลของการออกแบบนี้อยู่ใน [ADR 002](docs/adr/002-database-cache-provisioning.md)

template มี version กำกับ (`generator.TemplateVersion`) project ที่ใช้ template เก่าจะเห็นปุ่ม **Open upgrade PR**
ซึ่งจะเปิด PR อัปเดตไฟล์โดยคง image tag ที่ deploy อยู่ไว้ ไม่ push ทับไฟล์ที่ผู้ใช้แก้

### Golden Path (One-Click Setup)

กด **Setup All** แล้วระบบจะรัน 3 ขั้นตอนนี้ใน background:

```
1. Connect GitHub   → สร้าง repo, push ไฟล์, ลงทะเบียน webhook
2. Register ArgoCD  → Vault policy, pull secrets, AppProject, Application ×3
3. Setup Monitoring → Grafana dashboard (เลือก environment ได้)
```

สถานะของแต่ละขั้นตอนเก็บไว้ใน DB ปิดหน้าเว็บระหว่างรันได้ ถ้าขั้นตอนไหน fail ให้กด **Resume** เพื่อทำต่อจากจุดนั้น
ถ้ามี API หลาย replica ก็ไม่รันซ้อนกัน เพราะใช้ Postgres advisory lock

### Environments, Deploy และ Promotion

| Environment | Namespace | Sync | เปลี่ยนเวอร์ชันโดย |
|-------------|-----------|------|------------------|
| dev | `<slug>-dev` | auto | CI ทุกครั้งที่ push `main` หรือกด **Deploy** |
| staging | `<slug>-staging` | auto | **Promote ← dev** (ทำได้ทันที) |
| prod | `<slug>-prod` | manual | **Request promote ← staging** และต้องให้ owner **อีกคน** approve |

promotion ใช้ image tag เดิม (ไม่ build ใหม่) และ promote ได้เฉพาะเมื่อ environment ต้นทางอยู่ในสถานะ Healthy
deployment จะถือว่า success ก็ต่อเมื่อ ArgoCD รายงานว่ากำลังรัน image ที่ถูก build/promote จริง แต่ละ environment มี deployment ที่กำลังรันได้ครั้งละ 1 ตัว

สถานะ deployment: `pending → building → deploying → success / failed` (ถ้าไม่จบภายใน 30 นาทีจะถูกตั้งเป็น failed)

### Access Control

| Role | ทำอะไรได้ |
|------|---------|
| viewer | ดู project, environments, deployments, ชื่อ secret key |
| maintainer | เพิ่มจาก viewer: setup, deploy, promote ไป staging/ขอ promote prod, จัดการ secrets, ดู pod logs, audit log |
| owner | เพิ่มจาก maintainer: approve/reject prod, rollback, จัดการ members, ลบ project |

- คนที่สร้าง project จะเป็น owner โดยอัตโนมัติ และสร้าง team ได้ที่หน้า **Teams**
- ถ้าไม่มีสิทธิ์ใน project นั้น API จะตอบ 404 เพื่อไม่ให้รู้ว่า project มีอยู่
- การลบ project ต้องพิมพ์ slug เพื่อยืนยัน ระบบจะลบ ArgoCD apps และ Vault access, **archive** GitHub repo แล้ว soft delete ใน DB

### Secrets

เก็บใน Vault ที่ `secret/kuberlauncher/<slug>` โดย key ต้องเป็นชื่อ env var ที่ถูกต้อง
แต่ละ project มี service account `<slug>-vault` ที่อ่านได้เฉพาะ path ของตัวเอง แล้ว External Secrets จะ sync เข้า pod ทุก environment

## API Reference

ทุก route ใต้ `/api/v1` ต้องส่ง `Authorization: Bearer <session JWT>` ส่วน role ในตารางคือสิทธิ์ขั้นต่ำที่ต้องมี

| Method | Endpoint | Role |
|--------|----------|------|
| `GET` | `/api/v1/me` | — |
| `GET` / `POST` | `/api/v1/teams` | — |
| `GET` / `PUT` | `/api/v1/teams/:team/members` | team member / team owner |
| `DELETE` | `/api/v1/teams/:team/members/:login` | team owner |
| `GET` | `/api/v1/audit` | platform admin |
| `GET` / `POST` | `/api/v1/projects` | — (list แสดงเฉพาะ project ที่มีสิทธิ์) |
| `GET` | `/api/v1/projects/:id` · `/download` · `/preview` | viewer |
| `GET` | `/api/v1/projects/:id/environments` · `/promotions` · `/members` · `/secrets` · `/setup` · `/setup/stream` | viewer |
| `GET` | `/api/v1/projects/:id/deployments[/:dep_id[/stream]]` | viewer |
| `POST` | `/api/v1/projects/:id/repo` · `/repo/repair` · `/templates/upgrade` · `/argocd` · `/monitoring` · `/setup` | maintainer |
| `POST` | `/api/v1/projects/:id/deployments` · `/environments/:env/promote` | maintainer |
| `POST` / `DELETE` | `/api/v1/projects/:id/secrets[/:key]` | maintainer |
| `GET` | `/api/v1/projects/:id/logs?env=dev` · `/audit` | maintainer |
| `POST` | `/api/v1/projects/:id/environments/:env/rollback` · `/promotions/:pid/approve` · `/promotions/:pid/reject` | owner |
| `PUT` / `DELETE` | `/api/v1/projects/:id/members[/:member_id]` | owner |
| `DELETE` | `/api/v1/projects/:id` (body `{"confirm": "<slug>"}`) | owner |

route ที่ไม่ต้องใช้ session: `/healthz`, `/readyz`, `/metrics`, `POST /webhooks/github` (HMAC signature) และ `POST /webhooks/argocd` (bearer token)

## Deploying kuberLaunch

platform ถูก deploy ด้วย GitOps แบบเดียวกับ service ที่มันสร้าง:

```bash
kubectl apply -f deploy/argocd/kuberlaunch.yaml   # Helm chart: deploy/helm/kuberlaunch
```

- image `ghcr.io/<owner>/kuberlaunch-{api,web}` ถูก build โดย [release.yml](.github/workflows/release.yml)
- database เป็น CloudNativePG cluster 3 instances พร้อม WAL archiving และ daily backup ไป S3 (`postgresql.cnpg.*`)
- secret ของ platform ถูก sync จาก Vault `secret/platform/kuberlaunch` แล้ว mount เป็นไฟล์ (`secrets.*`)
- ArgoCD notifications สำหรับ webhook ดูได้ที่ [k8s/platform/argocd-notifications.yaml](k8s/platform/argocd-notifications.yaml)

## Testing

```bash
cd api
go test ./...                       # unit tests (generator golden files, state machines, clients)
go test -tags integration ./...     # Postgres จริงผ่าน testcontainers (ต้องมี Docker)
go test ./internal/generator -update   # อัปเดต golden files หลังแก้ template (ต้อง review diff ด้วย)

cd ..
scripts/e2e.sh                      # API → generate → build → helm install บน Kind → เรียก service
```

CI ([ci.yml](.github/workflows/ci.yml)) รันชุดเดียวกันนี้ทั้งหมด พร้อม golangci-lint, `helm lint`/kubeconform ของ chart ที่ generate
(ทุก runtime × ทุก environment), actionlint ของ workflow ที่ generate และ lint/build ของ web
